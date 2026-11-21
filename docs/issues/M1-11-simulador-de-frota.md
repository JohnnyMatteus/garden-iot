# M1-11 · Simulador de frota (Go), versão inicial

- **core:** false
- **curso:** fc4-tests-advanced
- **subagente:** implementador (Go) + testador
- **depende de:** M1-06, M1-08
- **estimativa:** 1 dia

## Objetivo

Gerar telemetria de N nós simulados, cada um com certificado próprio, publicando no formato do firmware v1 com perfis (dia ensolarado, chuva, vento forte, sensor travado, nó que cai), para alimentar o pipeline e os testes sem hardware (seção 4.3).

## Critérios de aceite

```gherkin
Funcionalidade: Simulador de frota

  Cenário: Frota pequena
    Dado 10 nós com o perfil "dia ensolarado" a 1 leitura por minuto
    Quando o simulador roda por 5 minutos
    Então 50 lotes chegam ao EMQX com sequências crescentes por nó

  Cenário: Sensor travado
    Dado um nó com o perfil "sensor travado"
    Então o valor de umidade do solo fica constante

  Cenário: Nó que cai
    Dado um nó com o perfil "nó que cai"
    Quando ele desconecta sem aviso
    Então o EMQX publica o Last Will dele

  Cenário: Duplicadas e fora de ordem sob demanda
    Dado a opção de injetar duplicadas e atraso
    Então o simulador republica lotes já enviados e envia lotes fora de ordem na taxa configurada

  Cenário: Reprodutível
    Dado a mesma semente
    Então a sequência de leituras geradas é a mesma
```

## Arquivos prováveis

- `services/fleet-sim/`
- `services/fleet-sim/profiles/*.yaml`

## Notas

- O teste de carga com 1.000 nós e os perfis de caos completos ficam para o M6; o M1 só precisa do simulador funcional.
