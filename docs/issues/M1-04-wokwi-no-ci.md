# M1-04 · Wokwi no CI e testes unitários do firmware

- **core:** false
- **curso:** lt-devops-sre
- **subagente:** devops + testador
- **depende de:** M1-02
- **estimativa:** ½ dia

## Objetivo

Rodar o firmware no Wokwi pelo `wokwi-cli` no CI, validando a saída serial com `--expect-text`, e rodar os testes unitários de conversão do firmware, para ter regressão de firmware sem placa (seção 4.3, ADR 0008).

## Critérios de aceite

```gherkin
Funcionalidade: Firmware testado no CI

  Cenário: Simulação passa
    Dado um PR que muda firmware/
    Quando o job de firmware roda
    Então o wokwi-cli executa o binário e encontra o texto esperado de "lote publicado" dentro do tempo limite

  Cenário: Regressão detectada
    Dado uma mudança que impede a publicação do lote
    Quando o job roda
    Então ele falha com a saída serial anexada

  Cenário: Testes unitários
    Dado os testes de conversão e de buffer do firmware
    Quando o job roda
    Então todos passam
```

## Arquivos prováveis

- `.github/workflows/firmware.yml`
- `firmware/wokwi.toml`, `firmware/test/`

## Notas

- O `wokwi-cli` no CI exige um token do Wokwi como segredo do repositório; o autor precisa criar.
- Ver no README o ponto em aberto sobre o firmware simulado alcançar o EMQX local no CI.
