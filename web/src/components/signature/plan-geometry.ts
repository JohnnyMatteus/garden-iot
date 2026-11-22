import type { BedOverview } from '@/api/schemas'
import { pct, points, ago } from '@/lib/format'

/** Umidade que enche o canteiro até o topo; acima disso o nível encosta no contorno. */
export const SCALE_MAX_PCT = 60

export const CELL = { w: 200, h: 176 }
export const GUTTER = 16
const MAX_COLS = 3

export interface Rect {
  x: number
  y: number
  w: number
  h: number
}

/** O modelo ainda não guarda coordenadas: a planta distribui os canteiros em grade, na ordem de cadastro. */
export function layoutBeds(count: number): { rects: Rect[]; width: number; height: number } {
  const cols = Math.max(1, Math.min(count, MAX_COLS))
  const rows = Math.max(1, Math.ceil(count / cols))
  const rects = Array.from({ length: count }, (_, i) => ({
    x: GUTTER + (i % cols) * (CELL.w + GUTTER),
    y: GUTTER + Math.floor(i / cols) * (CELL.h + GUTTER),
    w: CELL.w,
    h: CELL.h,
  }))
  return { rects, width: GUTTER + cols * (CELL.w + GUTTER), height: GUTTER + rows * (CELL.h + GUTTER) }
}

/** Fração 0..1 da altura do canteiro, contada de baixo para cima. */
export function levelFraction(valuePct: number): number {
  return Math.min(1, Math.max(0, valuePct / SCALE_MAX_PCT))
}

export type ValveView = 'unknown' | 'closed' | 'open' | 'awaiting-open' | 'awaiting-close'

/** O desenho mostra o que o nó reportou; o desejado só aparece como "aguardando o nó". */
export function valveView(bed: BedOverview): ValveView {
  const v = bed.valve
  if (!v?.reported) return v?.desired ? (v.desired.valve === 'open' ? 'awaiting-open' : 'closed') : 'unknown'
  if (v.reported.valve === 'open') return v.desired?.valve === 'closed' ? 'awaiting-close' : 'open'
  return v.desired?.valve === 'open' ? 'awaiting-open' : 'closed'
}

const VALVE_TEXT: Record<ValveView, string> = {
  unknown: 'válvula sem informação',
  closed: 'válvula fechada',
  open: 'válvula aberta, rega em curso',
  'awaiting-open': 'válvula fechada, aguardando o nó abrir',
  'awaiting-close': 'válvula aberta, aguardando o nó fechar',
}
export const valveText = (v: ValveView) => VALVE_TEXT[v]

const MOISTURE_TEXT = { below: 'abaixo da faixa', within: 'dentro da faixa', above: 'acima da faixa', unknown: '' } as const

/** Texto lido por leitores de tela: tudo que o desenho mostra, em frase. */
export function describeBed(bed: BedOverview): string {
  const head = bed.crop ? `${bed.name}, ${bed.crop}` : bed.name
  const band = `${bed.idealMinPct.toFixed(0)} a ${bed.idealMaxPct.toFixed(0)} %`
  const valve = valveText(valveView(bed))
  if (bed.moisturePct === undefined || bed.readAt === undefined) {
    return `${head}: sem leitura recente de umidade; ${valve}`
  }
  const deficit = bed.deficitPct ? `, déficit de ${points(bed.deficitPct)}` : ''
  return `${head}: umidade ${pct(bed.moisturePct)}, ${MOISTURE_TEXT[bed.moistureStatus]} de ${band}${deficit}; ${valve}; leitura ${ago(bed.readAt)}`
}
