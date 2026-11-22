import { describe, expect, it } from 'vitest'
import type { BedOverview } from '@/api/schemas'
import { describeBed, layoutBeds, levelFraction, valveView } from './plan-geometry'

const base: BedOverview = {
  id: 'c1', name: 'Canteiro de alface', crop: 'alface', irrigation: 'drip', idealMinPct: 30, idealMaxPct: 40,
  moistureStatus: 'below', moisturePct: 22, deficitPct: 18, readAt: new Date(), irrigating: false,
}
const at = new Date()

describe('levelFraction', () => {
  it('limita o nível ao canteiro, de 0 a 1', () => {
    expect(levelFraction(-5)).toBe(0)
    expect(levelFraction(30)).toBe(0.5)
    expect(levelFraction(95)).toBe(1)
  })
})

describe('layoutBeds', () => {
  it('quebra em linhas de três e nunca sobrepõe canteiros', () => {
    const { rects, width, height } = layoutBeds(4)
    expect(rects[3]!.y).toBeGreaterThan(rects[0]!.y)
    expect(rects[1]!.x).toBeGreaterThanOrEqual(rects[0]!.x + rects[0]!.w)
    expect(width).toBeGreaterThan(rects[2]!.x + rects[2]!.w)
    expect(height).toBeGreaterThan(rects[3]!.y + rects[3]!.h)
  })
  it('horta vazia ainda tem área de desenho', () => {
    expect(layoutBeds(0).rects).toEqual([])
  })
})

describe('valveView', () => {
  it('mostra o reportado e trata desejado diferente como aguardando o nó', () => {
    const v = (d?: 'open' | 'closed', r?: 'open' | 'closed') => ({
      ...base,
      valve: { inSync: d === r, desired: d && { valve: d, at }, reported: r && { valve: r, at } },
    })
    expect(valveView(v('open', 'open'))).toBe('open')
    expect(valveView(v('open', 'closed'))).toBe('awaiting-open')
    expect(valveView(v('closed', 'open'))).toBe('awaiting-close')
    expect(valveView(v('open', undefined))).toBe('awaiting-open')
    expect(valveView(base)).toBe('unknown')
  })
})

describe('describeBed', () => {
  it('diz em frase o que o desenho mostra', () => {
    const text = describeBed({ ...base, irrigating: true, valve: { inSync: true, reported: { valve: 'open', at } } })
    expect(text).toContain('umidade 22,0 %')
    expect(text).toContain('abaixo da faixa de 30 a 40 %')
    expect(text).toContain('déficit de 18,0 pp')
    expect(text).toContain('rega em curso')
  })
  it('sem leitura nunca afirma que está dentro da faixa', () => {
    const text = describeBed({ ...base, moisturePct: undefined, readAt: undefined, moistureStatus: 'unknown', deficitPct: undefined })
    expect(text).toContain('sem leitura recente')
    expect(text).not.toContain('dentro da faixa')
  })
})
