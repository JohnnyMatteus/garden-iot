import { useSessionStorage } from '@vueuse/core'
import { computed } from 'vue'

// O token vive só na aba (sessionStorage): fechar a aba encerra a sessão.
const token = useSessionStorage<string>('cota.token', '')

export type Role = 'owner' | 'operator' | 'viewer'

export interface Claims {
  sub: string
  tenant: string
  role: Role
  exp: number
}

// Só lê as claims para a interface; quem valida a assinatura é a API.
export function decodeClaims(jwt: string): Claims | null {
  const part = jwt.split('.')[1]
  if (!part) return null
  try {
    const json = atob(part.replace(/-/g, '+').replace(/_/g, '/'))
    const c = JSON.parse(json) as Partial<Claims>
    if (!c.sub || !c.tenant || !c.exp || !['owner', 'operator', 'viewer'].includes(c.role ?? '')) return null
    return c as Claims
  } catch {
    return null
  }
}

export const claims = computed(() => decodeClaims(token.value))
export const isAuthenticated = computed(() => claims.value !== null && claims.value.exp * 1000 > Date.now())
export const canOperate = computed(() => claims.value?.role === 'owner' || claims.value?.role === 'operator')

export function getToken(): string {
  return token.value
}

export function signIn(jwt: string): boolean {
  if (!decodeClaims(jwt.trim())) return false
  token.value = jwt.trim()
  return true
}

export function signOut(): void {
  token.value = ''
}
