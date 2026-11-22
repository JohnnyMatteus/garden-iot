import { startOfDay } from 'date-fns'
import type { AggregatePoint, Command } from '@/api/schemas'

export interface DayBalance {
  day: Date
  avg: number | null
  min: number | null
  max: number | null
  irrigatedSeconds: number
  irrigations: number
}

/** Regas que o nó aceitou. Pedido recusado, expirado ou falho não molhou o canteiro. */
const APPLIED = new Set(['confirmed'])

/**
 * Balanço observado por dia: o que o solo mediu e quanto tempo de rega foi aplicado.
 * Não é o balanço FAO-56 (ET₀, chuva efetiva): esse depende do planejador de irrigação persistir o déficit.
 */
export function dailyBalance(days: AggregatePoint[], commands: Command[]): DayBalance[] {
  const byDay = new Map<number, DayBalance>()
  const slot = (d: Date) => {
    const key = startOfDay(d).getTime()
    let row = byDay.get(key)
    if (!row) {
      row = { day: new Date(key), avg: null, min: null, max: null, irrigatedSeconds: 0, irrigations: 0 }
      byDay.set(key, row)
    }
    return row
  }
  for (const p of days) {
    const row = slot(p.at)
    row.avg = p.avg
    row.min = p.min
    row.max = p.max
  }
  for (const c of commands) {
    if (!APPLIED.has(c.status)) continue
    const row = slot(c.createdAt)
    row.irrigatedSeconds += c.durationSeconds
    row.irrigations += 1
  }
  return [...byDay.values()].sort((a, b) => b.day.getTime() - a.day.getTime())
}
