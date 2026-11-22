import { describe, expect, it } from 'vitest'
import type { AggregatePoint, Command } from '@/api/schemas'
import { dailyBalance } from './balance'

const point = (iso: string, avg: number): AggregatePoint => ({ at: new Date(iso), avg, min: avg - 2, max: avg + 2, count: 10 })
const cmd = (iso: string, status: Command['status'], durationSeconds: number): Command => ({
  id: iso, nodeId: 'n3', action: 'open_valve', durationSeconds, status, requestedBy: 'ana', createdAt: new Date(iso), expiresAt: new Date(iso),
})

describe('dailyBalance', () => {
  it('soma só as regas que o nó confirmou e ordena do dia mais novo ao mais antigo', () => {
    const rows = dailyBalance(
      [point('2026-11-20T03:00:00Z', 30), point('2026-11-21T03:00:00Z', 26)],
      [cmd('2026-11-21T09:00:00Z', 'confirmed', 120), cmd('2026-11-21T10:00:00Z', 'confirmed', 60), cmd('2026-11-21T11:00:00Z', 'expired', 300)],
    )
    expect(rows.map((r) => r.day.getTime())).toEqual([...rows.map((r) => r.day.getTime())].sort((a, b) => b - a))
    const latest = rows[0]!
    expect(latest.irrigatedSeconds).toBe(180)
    expect(latest.irrigations).toBe(2)
  })

  it('dia com rega e sem leitura mantém a rega e deixa a umidade vazia', () => {
    const [row] = dailyBalance([], [cmd('2026-11-21T09:00:00Z', 'confirmed', 60)])
    expect(row?.avg).toBeNull()
    expect(row?.irrigatedSeconds).toBe(60)
  })
})
