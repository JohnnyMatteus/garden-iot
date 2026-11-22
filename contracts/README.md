# contracts

Contratos entre as partes do garden-iot. A fonte da verdade é o Protobuf em `proto/`; o código Go gerado fica em `gen/go/` (módulo `github.com/JohnnyMatteus/garden-iot/contracts`) e é versionado para que os serviços não precisem do `buf` para compilar.

| Contrato | Tópico Kafka | Chave | Retenção |
| --- | --- | --- | --- |
| `telemetria.bruta.v1.TelemetriaBruta` | `telemetria.bruta.v1` | `node_id` | 7 dias |

## Regras de evolução (ADR 0002)

- Compatibilidade BACKWARD no Schema Registry; `buf breaking` (regra `WIRE_JSON`) roda no CI contra a `main`.
- Nunca reutilizar nem renumerar campo; remover campo exige `reserved`.
- Mudança incompatível vira tópico novo (`telemetria.bruta.v2`).
- O subject no registry segue `TopicNameStrategy`: `telemetria.bruta.v1-value`.

## Comandos

```bash
make contracts-lint        # buf lint
make contracts-breaking    # buf breaking contra origin/main
make contracts-gen         # regenera gen/go (exige buf e protoc-gen-go)
```

## Payload MQTT

O corpo publicado pelo firmware em `horta/{tenant}/{horta}/{no}/telemetria` é definido pelo firmware (M1-02) e viaja intacto em `payload`. A bridge só extrai tenant, horta e nó do tópico, e `sequence`/`fw_version`/`read_at` do envelope do payload.
