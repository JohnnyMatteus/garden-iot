import type { AggregatePoint } from '@/api/schemas'

export interface ChartBox {
  width: number
  height: number
  left: number
  right: number
  top: number
  bottom: number
}

export interface Domain {
  t0: number
  t1: number
  y0: number
  y1: number
}

/** O eixo Y sempre inclui a faixa ideal, para ela nunca ficar fora do desenho. */
export function chartDomain(points: AggregatePoint[], band: { min: number; max: number } | null, from: Date, to: Date): Domain {
  const values = points.flatMap((p) => [p.min, p.max])
  if (band) values.push(band.min, band.max)
  const lo = values.length ? Math.min(...values) : 0
  const hi = values.length ? Math.max(...values) : 1
  const pad = Math.max((hi - lo) * 0.1, 0.5)
  return { t0: from.getTime(), t1: to.getTime(), y0: lo - pad, y1: hi + pad }
}

export const scaleX = (box: ChartBox, d: Domain, t: number) =>
  box.left + ((t - d.t0) / (d.t1 - d.t0)) * (box.width - box.left - box.right)

export const scaleY = (box: ChartBox, d: Domain, v: number) =>
  box.top + (1 - (v - d.y0) / (d.y1 - d.y0)) * (box.height - box.top - box.bottom)

/** Linha da média e sombra min/max; buracos maiores que 3 baldes quebram a linha em vez de ligar pontos distantes. */
export function buildPaths(points: AggregatePoint[], box: ChartBox, d: Domain, bucketMs: number) {
  const segments: AggregatePoint[][] = []
  for (const p of points) {
    const last = segments.at(-1)
    const prev = last?.at(-1)
    if (last && prev && p.at.getTime() - prev.at.getTime() <= bucketMs * 3) last.push(p)
    else segments.push([p])
  }
  const xy = (p: AggregatePoint, v: number) => `${scaleX(box, d, p.at.getTime()).toFixed(1)},${scaleY(box, d, v).toFixed(1)}`
  const line = segments.map((s) => 'M' + s.map((p) => xy(p, p.avg)).join('L')).join('')
  const shade = segments
    .filter((s) => s.length > 1)
    .map((s) => 'M' + s.map((p) => xy(p, p.max)).join('L') + 'L' + [...s].reverse().map((p) => xy(p, p.min)).join('L') + 'Z')
    .join('')
  return { line, shade }
}

export function nearestPoint(points: AggregatePoint[], t: number): AggregatePoint | null {
  let best: AggregatePoint | null = null
  for (const p of points) {
    if (!best || Math.abs(p.at.getTime() - t) < Math.abs(best.at.getTime() - t)) best = p
  }
  return best
}
