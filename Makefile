SHELL := /bin/bash
.DEFAULT_GOAL := help

COMPOSE ?= docker compose
PROFILES ?= --profile core --profile observability
GO_MODULES := contracts services/bridge services/api services/irrigation tools/fleet-sim
BUF ?= buf

-include .env
export

.PHONY: help
help: ## Lista os alvos
	@grep -E '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-18s %s\n", $$1, $$2}'

.PHONY: env
env: ## Cria .env a partir de .env.example se não existir
	@test -f .env || cp .env.example .env

.PHONY: up
up: env ## Sobe core + observability (use PROFILES="--profile core --profile tools" para outros)
	$(COMPOSE) $(PROFILES) up -d --remove-orphans
	infra/wait-healthy.sh

.PHONY: down
down: ## Para os containers (mantém volumes)
	$(COMPOSE) --profile core --profile observability --profile tools down

.PHONY: logs
logs: ## Segue os logs (SERVICE=emqx para filtrar)
	$(COMPOSE) --profile core --profile observability --profile tools logs -f $(SERVICE)

.PHONY: ps
ps: ## Estado dos containers
	$(COMPOSE) --profile core --profile observability --profile tools ps

.PHONY: pki
pki: ## Reexecuta o bootstrap da PKI (CA, certs de servidor e serviços)
	$(COMPOSE) --profile core run --rm vault-init

.PHONY: provision
provision: ## Provisiona dispositivo: make provision TENANT=t1 HORTA=h1 NO=n1
	infra/vault/provision.sh $(TENANT) $(HORTA) $(NO)

.PHONY: seed
seed: ## Provisiona 3 nós de exemplo do tenant demo
	@for n in n1 n2 n3; do infra/vault/provision.sh demo horta-1 $$n; done

.PHONY: compose-check
compose-check: ## Valida o compose com todos os profiles
	$(COMPOSE) --profile core --profile observability --profile tools config -q

.PHONY: lint
lint: compose-check contracts-lint ## Lint do que existe neste checkout
	@for m in $(GO_MODULES); do \
	  if [ -f $$m/go.mod ]; then echo "== vet $$m"; (cd $$m && go vet ./...) || exit 1; fi; \
	done
	@shellcheck infra/vault/*.sh infra/kafka/*.sh 2>/dev/null || echo "shellcheck ausente, ignorando"

.PHONY: test
test: ## Testes unitários Go de todos os módulos
	@for m in $(GO_MODULES); do \
	  if [ -f $$m/go.mod ]; then echo "== test $$m"; (cd $$m && go test ./... -race -count=1) || exit 1; fi; \
	done

.PHONY: test-integration
test-integration: ## Integração (exige `make up`): ACL do broker e RLS do banco
	infra/timescaledb/tests/rls_test.sh
	cd infra/tests && go test -tags=integration ./... -count=1
	@for m in $(GO_MODULES); do \
	  if [ -f $$m/go.mod ]; then (cd $$m && go test -tags=integration ./... -count=1) || exit 1; fi; \
	done


.PHONY: api
api: ## Roda a API local (exige make up)
	cd services/api && go run ./cmd/api

.PHONY: web-dev
web-dev: ## Front-end em modo dev (Vite em :5173, /api vai para a API em :8090)
	cd web && npm ci --no-audit --no-fund && npm run dev

.PHONY: token
token: ## Token de dev: make token TENANT=demo ROLE=operator
	@cd services/api && go run ./cmd/devtoken $(or $(TENANT),demo) $(or $(ROLE),operator)

.PHONY: contracts-lint
contracts-lint: ## buf lint
	cd contracts && $(BUF) lint

.PHONY: contracts-breaking
contracts-breaking: ## buf breaking contra origin/main
	cd contracts && $(BUF) breaking --against '../.git#branch=origin/main,subdir=contracts'

.PHONY: contracts-gen
contracts-gen: ## Regenera contracts/gen/go
	cd contracts && $(BUF) generate

.PHONY: clean
clean: ## Remove containers, volumes e certificados gerados
	$(COMPOSE) --profile core --profile observability --profile tools down -v --remove-orphans
	find infra/pki/out -mindepth 1 ! -name .gitkeep -delete
