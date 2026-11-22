# Contexto de build: raiz do repositório (o módulo usa replace para ../../contracts).
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY contracts/ contracts/
COPY services/irrigation/ services/irrigation/
WORKDIR /src/services/irrigation
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/irrigation ./cmd/irrigation

FROM alpine:3.20
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 app
USER app
COPY --from=build /out/irrigation /usr/local/bin/irrigation
EXPOSE 9104
ENTRYPOINT ["/usr/local/bin/irrigation"]
