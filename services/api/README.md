# services/api

REST, SSE e comandos do garden-iot (ADR 0012). Contrato: [`contracts/openapi/garden-api.yaml`](../../contracts/openapi/garden-api.yaml), conferido contra as rotas por teste.

```
domain/          modelo e política de rega (sem dependências)
application/     casos de uso e portas
infrastructure/  postgres (RLS por tenant), mqtt (MQTT 5), auth (JWT)
interfaces/      httpapi (chi, SSE)
cmd/api          binário; cmd/devtoken emite JWT de desenvolvimento
```

## Rodar

```bash
make up && make seed
set -a; . ./.env; set +a
make api                                  # :8090
TOKEN=$(make -s token TENANT=demo ROLE=operator)
curl -H "Authorization: Bearer $TOKEN" localhost:8090/v1/gardens
curl -H "Authorization: Bearer $TOKEN" -d '{"durationSeconds":120}' \
  localhost:8090/v1/gardens/horta-1/nodes/n3/irrigations
curl -N "localhost:8090/v1/stream/readings?garden=horta-1&access_token=$TOKEN"
```

## Segurança

- JWT HS256 com segredo de 32+ caracteres; `alg` fixado, `iss` e `exp` obrigatórios; claims `tenant`, `sub`, `role`.
- O tenant vem só do token. Cada requisição roda como `tenant_<id>` no banco (RLS).
- Rega manual: papel `owner` ou `operator`, política (duração, geada, vento com aspersão, reservatório, comando em aberto), publicação MQTT 5 com Message Expiry. A API nunca aciona atuador; o nó confirma ou recusa.

## Testes

`go test ./...` (unitários, rotas contra o OpenAPI) e `go test -tags=integration ./...` (RLS no banco real, via `make test-integration`).
