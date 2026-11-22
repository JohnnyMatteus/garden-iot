import { describe, expect, it } from 'vitest'
import { decodeClaims } from './session'

const jwt = (payload: object) => `h.${btoa(JSON.stringify(payload)).replace(/=+$/, '')}.s`

describe('decodeClaims', () => {
  it('lê tenant, papel e validade do token da API', () => {
    expect(decodeClaims(jwt({ sub: 'ana', tenant: 'demo', role: 'operator', exp: 1 }))).toMatchObject({ tenant: 'demo', role: 'operator' })
  })
  it('recusa papel desconhecido e texto que não é JWT', () => {
    expect(decodeClaims(jwt({ sub: 'ana', tenant: 'demo', role: 'admin', exp: 1 }))).toBeNull()
    expect(decodeClaims('qualquer coisa')).toBeNull()
  })
})
