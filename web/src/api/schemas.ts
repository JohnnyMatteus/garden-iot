import { z } from 'zod'

// O Go serializa instantes em RFC 3339; a fronteira converte para Date uma única vez.
const instant = z.string().transform((s, ctx) => {
  const d = new Date(s)
  if (Number.isNaN(d.getTime())) {
    ctx.addIssue({ code: 'custom', message: 'data inválida' })
    return z.NEVER
  }
  return d
})

export const valveSchema = z.enum(['open', 'closed'])
export type Valve = z.infer<typeof valveSchema>

const actuatorState = z.object({ valve: valveSchema, flowLpm: z.number().optional(), at: instant })
export type ActuatorState = z.infer<typeof actuatorState>

export const commandStatusSchema = z.enum(['pending', 'sent', 'confirmed', 'rejected', 'expired', 'failed'])
export type CommandStatus = z.infer<typeof commandStatusSchema>

export const commandSchema = z.object({
  id: z.string(),
  nodeId: z.string(),
  action: z.string(),
  durationSeconds: z.number(),
  status: commandStatusSchema,
  requestedBy: z.string(),
  createdAt: instant,
  expiresAt: instant,
})
export type Command = z.infer<typeof commandSchema>

export const moistureStatusSchema = z.enum(['unknown', 'below', 'within', 'above'])
export type MoistureStatus = z.infer<typeof moistureStatusSchema>

export const bedOverviewSchema = z.object({
  id: z.string(),
  name: z.string(),
  crop: z.string().default(''),
  irrigation: z.enum(['drip', 'sprinkler']),
  idealMinPct: z.number(),
  idealMaxPct: z.number(),
  moistureStatus: moistureStatusSchema,
  moisturePct: z.number().optional(),
  deficitPct: z.number().optional(),
  airTempC: z.number().optional(),
  readAt: instant.optional(),
  sensorNodeId: z.string().optional(),
  actuatorNodeId: z.string().optional(),
  valve: z
    .object({ desired: actuatorState.optional(), reported: actuatorState.optional(), inSync: z.boolean() })
    .optional(),
  irrigating: z.boolean(),
  openCommand: commandSchema.optional(),
})
export type BedOverview = z.infer<typeof bedOverviewSchema>

export const overviewSchema = z.object({
  gardenId: z.string(),
  generatedAt: instant,
  openAlerts: z.number(),
  beds: z.array(bedOverviewSchema),
})
export type Overview = z.infer<typeof overviewSchema>

export const gardenSchema = z.object({ id: z.string(), name: z.string(), createdAt: instant })
export type Garden = z.infer<typeof gardenSchema>

export const nodeSchema = z.object({
  id: z.string(),
  kind: z.enum(['sensor', 'actuator']),
  bedId: z.string().nullish(),
  fwVersion: z.string().default(''),
  lastSeenAt: instant.nullish(),
})
export type GardenNode = z.infer<typeof nodeSchema>

export const nodeStateSchema = z.object({
  nodeId: z.string(),
  desired: actuatorState.optional(),
  reported: actuatorState.optional(),
  inSync: z.boolean(),
})
export type NodeState = z.infer<typeof nodeStateSchema>

export const decisionSchema = z.object({
  at: instant,
  kind: z.enum(['irrigation', 'alert']),
  outcome: z.string(),
  reason: z.string(),
  nodeId: z.string().optional(),
  durationSeconds: z.number().optional(),
})
export type Decision = z.infer<typeof decisionSchema>

export const aggregatePointSchema = z.object({
  at: instant,
  avg: z.number(),
  min: z.number(),
  max: z.number(),
  count: z.number(),
})
export type AggregatePoint = z.infer<typeof aggregatePointSchema>

export const problemSchema = z.object({
  title: z.string(),
  status: z.number(),
  detail: z.string().optional(),
  violations: z.array(z.object({ rule: z.string(), reason: z.string() })).optional(),
})
export type Problem = z.infer<typeof problemSchema>
