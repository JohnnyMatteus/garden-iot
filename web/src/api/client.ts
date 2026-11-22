import { FetchError, ofetch } from 'ofetch'
import type { ZodType } from 'zod'
import { getToken, signOut } from '@/features/auth/session'
import { problemSchema, type Problem } from './schemas'

export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly problem: Problem | null,
  ) {
    super(problem?.title ?? `Falha ${status}`)
  }
}

const http = ofetch.create({
  baseURL: '/api',
  retry: 0,
  onRequest({ options }) {
    const headers = new Headers(options.headers)
    const token = getToken()
    if (token) headers.set('Authorization', `Bearer ${token}`)
    options.headers = headers
  },
})

function toApiError(e: unknown): Error {
  if (e instanceof FetchError) {
    const status = e.status ?? 0
    if (status === 401) signOut()
    const parsed = problemSchema.safeParse(e.data)
    return new ApiError(status, parsed.success ? parsed.data : null)
  }
  return e instanceof Error ? e : new Error(String(e))
}

/** Toda resposta passa pelo schema: contrato quebrado vira erro visível, não `undefined` na tela. */
export async function get<T>(path: string, schema: ZodType<T>, query?: Record<string, string | number | undefined>): Promise<T> {
  try {
    return schema.parse(await http(path, { query }))
  } catch (e) {
    throw toApiError(e)
  }
}

export async function post<T>(path: string, body: unknown, schema: ZodType<T>): Promise<T> {
  try {
    return schema.parse(await http(path, { method: 'POST', body: body as Record<string, unknown> }))
  } catch (e) {
    throw toApiError(e)
  }
}
