import { describe, expect, it } from 'vitest'
import { nodeSchema, overviewSchema, problemSchema } from './schemas'

// Corpo no formato exato que services/api/interfaces/httpapi/dto.go serializa.
const overview = {
  gardenId: 'horta-1',
  generatedAt: '2026-11-21T12:00:00.123456Z',
  openAlerts: 1,
  beds: [
    {
      id: 'canteiro-1', gardenId: 'horta-1', name: 'Canteiro de alface', crop: 'alface', irrigation: 'drip',
      idealMinPct: 30, idealMaxPct: 40, moistureStatus: 'below', moisturePct: 22, deficitPct: 18, airTempC: 24.5,
      readAt: '2026-11-21T11:59:30Z', sensorNodeId: 'n1', actuatorNodeId: 'n3',
      valve: { desired: { valve: 'open', at: '2026-11-21T11:58:00Z' }, reported: { valve: 'open', flowLpm: 2.4, at: '2026-11-21T11:58:02Z' }, inSync: true },
      irrigating: true,
    },
    { id: 'canteiro-2', name: 'Tomate', irrigation: 'sprinkler', idealMinPct: 25, idealMaxPct: 35, moistureStatus: 'unknown', irrigating: false },
  ],
}

describe('overviewSchema', () => {
  it('converte instantes em Date e mantém campos opcionais ausentes como undefined', () => {
    const o = overviewSchema.parse(overview)
    const [alface, tomate] = o.beds
    expect(o.generatedAt).toBeInstanceOf(Date)
    expect(alface?.valve?.reported?.flowLpm).toBe(2.4)
    expect(alface?.readAt?.toISOString()).toBe('2026-11-21T11:59:30.000Z')
    expect(tomate?.moisturePct).toBeUndefined()
    expect(tomate?.crop).toBe('')
  })

  it('recusa situação de umidade que a API não define', () => {
    const bad = structuredClone(overview)
    bad.beds[0]!.moistureStatus = 'wet'
    expect(() => overviewSchema.parse(bad)).toThrow()
  })

  it('recusa data inválida em vez de deixar Invalid Date chegar à tela', () => {
    const bad = structuredClone(overview)
    bad.generatedAt = 'ontem'
    expect(() => overviewSchema.parse(bad)).toThrow()
  })
})

describe('nodeSchema', () => {
  it('aceita nó sem canteiro e sem última telemetria', () => {
    const n = nodeSchema.parse({ id: 'n9', kind: 'sensor', lastSeenAt: null })
    expect(n.fwVersion).toBe('')
    expect(n.lastSeenAt).toBeNull()
  })
})

describe('problemSchema', () => {
  it('lê as violações de política devolvidas no 409', () => {
    const p = problemSchema.parse({ title: 'rega bloqueada', status: 409, violations: [{ rule: 'wind', reason: 'vento acima do limite' }] })
    expect(p.violations?.[0]?.rule).toBe('wind')
  })
})
