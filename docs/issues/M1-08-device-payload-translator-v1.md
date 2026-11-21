# M1-08 · `DevicePayloadTranslator` v1

- **core:** false
- **curso:** fc4-arch-hexagonal
- **subagente:** implementador (Go) + testador
- **depende de:** M1-02; ADR 0007
- **estimativa:** 1 dia

## Objetivo

Implementar, como pacote Go da camada de infraestrutura, o tradutor da ACL que converte o payload do firmware v1 (ADC, lux) em grandezas de domínio (umidade volumétrica %, °C, % de umidade relativa, lux, W/m² estimado) usando a calibração do sensor, com testes de contrato por versão de firmware.

## Critérios de aceite

```gherkin
Funcionalidade: Tradução do payload do dispositivo

  Cenário: Umidade do solo calibrada
    Dado calibração seco = 3000 e saturado = 1200 para o sensor
    E uma leitura ADC de 2100
    Quando o tradutor converte
    Então a umidade volumétrica é 50%

  Cenário: Luz como estimativa
    Dado uma leitura de luz em lux
    Quando o tradutor converte para radiação
    Então o valor vem com flag de qualidade "estimado" (seção 6.3)

  Cenário: Versão de firmware desconhecida
    Dado um payload com fw_version sem tradutor registrado
    Quando o tradutor recebe
    Então devolve erro explícito com a versão, sem converter

  Cenário: Contrato por versão
    Dado testdata/payload/v1/*.json e as saídas esperadas
    Quando os testes rodam
    Então todas as saídas conferem
```

## Arquivos prováveis

- `services/internal/acl/devicepayload/translator.go`
- `services/internal/acl/devicepayload/v1.go`
- `services/internal/acl/devicepayload/testdata/v1/`

## Notas

- Calibração por canteiro vem do contexto Cultivo (M3). No M1, a calibração vem de configuração por sensor; ver pontos em aberto.
- O mapeamento linear do primeiro cenário é ilustrativo; a curva de calibração real não está no design doc.
