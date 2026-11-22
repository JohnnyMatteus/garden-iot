import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { defineComponent, h } from 'vue'
import type { BedOverview } from '@/api/schemas'
import BedPlan from './BedPlan.vue'

const bed: BedOverview = {
  id: 'c1', name: 'Alface', crop: 'alface', irrigation: 'sprinkler', idealMinPct: 30, idealMaxPct: 40,
  moistureStatus: 'below', moisturePct: 22, deficitPct: 18, readAt: new Date(), irrigating: false,
}
const rect = { x: 16, y: 16, w: 200, h: 176 }
const mountBed = (b: BedOverview) =>
  mount(defineComponent({ render: () => h('svg', [h(BedPlan, { bed: b, rect })]) }))

describe('BedPlan', () => {
  it('desenha nível e hachura de déficit e expõe a descrição para leitor de tela', () => {
    const w = mountBed(bed)
    expect(w.find('.water').exists()).toBe(true)
    expect(w.find('.dry').exists()).toBe(true)
    expect(w.find('[role="button"]').attributes('aria-label')).toContain('abaixo da faixa')
    expect(w.find('.flow').exists()).toBe(false)
  })

  it('só anima o fluxo quando o nó reportou a válvula aberta', () => {
    const w = mountBed({ ...bed, irrigating: true, valve: { inSync: true, reported: { valve: 'open', at: new Date() } } })
    expect(w.find('.flow').exists()).toBe(true)
    const desiredOnly = mountBed({ ...bed, valve: { inSync: false, desired: { valve: 'open', at: new Date() } } })
    expect(desiredOnly.find('.flow').exists()).toBe(false)
  })

  it('canteiro sem leitura fica tracejado, sem nível e sem hachura', () => {
    const w = mountBed({ ...bed, moisturePct: undefined, readAt: undefined, deficitPct: undefined, moistureStatus: 'unknown' })
    expect(w.find('.water').exists()).toBe(false)
    expect(w.find('.dry').exists()).toBe(false)
    expect(w.find('.outline').classes()).toContain('unknown')
  })

  it('canteiro dentro ou acima da faixa não mostra hachura', () => {
    const w = mountBed({ ...bed, moisturePct: 41, deficitPct: 0, moistureStatus: 'above' })
    expect(w.find('.dry').exists()).toBe(false)
  })
})
