# Contexto de build: raiz do repositório (o módulo usa replace para ../../contracts).
#   docker build -f services/bridge/Dockerfile --target bridge  -t garden-bridge .
#   docker build -f services/bridge/Dockerfile --target persist -t garden-persist .
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY contracts/ contracts/
COPY services/bridge/ services/bridge/
WORKDIR /src/services/bridge
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/bridge ./cmd/bridge && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/persist ./cmd/persist

FROM alpine:3.20 AS base
RUN adduser -D -u 10001 app
USER app

FROM base AS bridge
COPY --from=build /out/bridge /usr/local/bin/bridge
EXPOSE 9102
ENTRYPOINT ["/usr/local/bin/bridge"]

FROM base AS persist
COPY --from=build /out/persist /usr/local/bin/persist
EXPOSE 9103
ENTRYPOINT ["/usr/local/bin/persist"]
