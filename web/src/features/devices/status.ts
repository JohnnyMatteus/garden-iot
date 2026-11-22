/** Nó sem telemetria há mais que isso é tratado como offline (o ciclo de deep sleep dos nós a bateria é de minutos). */
export const OFFLINE_AFTER_MS = 5 * 60_000

export function isOnline(lastSeenAt: Date | null | undefined, now: Date = new Date()): boolean {
  return !!lastSeenAt && now.getTime() - lastSeenAt.getTime() <= OFFLINE_AFTER_MS
}
