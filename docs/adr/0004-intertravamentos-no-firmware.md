# 0004. Intertravamentos de segurança no firmware, independentes da nuvem (G-04)

- **Status:** Proposto
- **Data:** 2026-11-21
- **Id no design doc:** G-04 (seção 13; detalhe na seção 4.2)

## Contexto

Ações físicas (água, bomba) não podem depender da disponibilidade da nuvem ou da rede: uma desconexão durante a rega ou um comando errado pode alagar o canteiro, esvaziar o reservatório ou queimar a bomba.

## Decisão

O nó atuador aplica intertravamentos locais no firmware, que valem mesmo sem nuvem: tempo máximo de válvula aberta; bomba nunca liga com reservatório vazio; fecha tudo se perder conexão por mais de X minutos durante uma rega. Válvulas são normalmente fechadas. O nó valida o intertravamento antes de aceitar um comando e reporta o estado real (aberta/fechada, vazão medida). As regras de segurança da irrigação no domínio (seção 6.4) continuam valendo na nuvem, inclusive para regas manuais.

## Alternativas consideradas

- O design doc não registra alternativas para esta decisão. A preencher pelo autor antes de aceitar.

## Consequências

- O pior caso de falha da nuvem é "não rega", nunca "rega sem parar".
- Regras duplicadas em dois lugares (firmware em C e domínio na nuvem), com testes nos dois (seção 11).
- Os valores (tempo máximo, X minutos) não estão fixados no design doc.
