# M1-03 · Firmware: buffer offline, deep sleep e watchdog

- **core:** false
- **curso:** —
- **subagente:** implementador (C, ESP-IDF)
- **depende de:** M1-02
- **estimativa:** 1 dia

## Objetivo

Guardar leituras em flash quando o nó está offline e reenviá-las com o timestamp original quando a conexão volta; entrar em deep sleep entre ciclos (nós a bateria); watchdog com reinício seguro (seção 4.1).

## Critérios de aceite

```gherkin
Funcionalidade: Robustez do firmware

  Cenário: Reenvio após queda
    Dado o broker indisponível durante 3 ciclos
    Quando a conexão volta
    Então os 3 lotes são publicados com os timestamps e sequências originais

  Cenário: Buffer cheio
    Dado o buffer em flash no limite
    Quando chega um novo lote
    Então o mais antigo é descartado e um contador de descartes vai no próximo lote

  Cenário: Watchdog
    Dado uma tarefa travada
    Quando o watchdog estoura
    Então o nó reinicia e volta ao ciclo normal sem perder a sequência
```

## Arquivos prováveis

- `firmware/main/offline_buffer.c|h`
- `firmware/main/power.c`
- `firmware/test/` (testes unitários do buffer)

## Notas

- A política de descarte com buffer cheio (mais antigo primeiro) é uma proposta; o design doc não define.
- OTA assinado com rollback (seção 4.1) é do M6.
