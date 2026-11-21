# 0006. Agente de IA sem acesso direto a atuadores; propostas com aprovação (G-06)

- **Status:** Proposto
- **Data:** 2026-11-21
- **Id no design doc:** G-06 (seção 13; detalhe na seção 8)

## Contexto

O assistente com IA explica decisões e diagnostica por foto, e um diagnóstico errado é um risco listado (seção 15). Um agente com acesso a válvulas e bomba poderia causar dano físico por erro do modelo ou por prompt injection.

## Decisão

O agente (LangGraph, ferramentas MCP `get_garden_status`, `get_sensor_history`, `get_irrigation_plan`, `explain_decision`, `propose_irrigation_change`) nunca aciona atuador diretamente. `propose_irrigation_change` gera uma proposta que passa pelas regras de segurança e exige aprovação do dono da horta (approval gate). O agente não recomenda produtos químicos nem dosagens, e suspeita com baixa confiança é apresentada como incerta, com orientação de procurar um agrônomo.

## Alternativas consideradas

- O design doc não registra alternativas para esta decisão. A preencher pelo autor antes de aceitar.

## Consequências

- Nenhuma ação física sai do modelo sem regra determinística e humano no caminho.
- Evals de recusa de recomendação química e de fonte obrigatória (seção 11).
- Mais atrito para o usuário em mudanças de rega sugeridas pela IA.
