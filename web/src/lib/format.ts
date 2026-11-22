import { format, formatDistanceToNowStrict } from 'date-fns'
import { ptBR } from 'date-fns/locale'

const nf = (digits: number) => new Intl.NumberFormat('pt-BR', { minimumFractionDigits: digits, maximumFractionDigits: digits })
const n1 = nf(1)

export const pct = (v: number) => `${n1.format(v)} %`
export const celsius = (v: number) => `${n1.format(v)} °C`
export const points = (v: number) => `${n1.format(v)} pp`
export const hhmmss = (d: Date) => format(d, 'HH:mm:ss')
export const dayMonth = (d: Date) => format(d, 'dd/MM')
export const dayMonthTime = (d: Date) => format(d, 'dd/MM HH:mm')
export const ago = (d: Date) => formatDistanceToNowStrict(d, { locale: ptBR, addSuffix: true })

export function duration(seconds: number): string {
  if (seconds < 60) return `${seconds} s`
  const m = Math.floor(seconds / 60)
  const s = seconds % 60
  return s ? `${m} min ${s} s` : `${m} min`
}
