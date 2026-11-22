import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, toValue, type MaybeRefOrGetter } from 'vue'
import { z } from 'zod'
import { get, post } from './client'
import {
  aggregatePointSchema, commandSchema, decisionSchema, gardenSchema, nodeSchema, nodeStateSchema, overviewSchema,
} from './schemas'

const REFRESH_MS = 10_000

export const useGardens = () => useQuery({ queryKey: ['gardens'], queryFn: () => get('/v1/gardens', z.array(gardenSchema)) })

export const useOverview = (gardenId: MaybeRefOrGetter<string>) =>
  useQuery({
    queryKey: computed(() => ['overview', toValue(gardenId)]),
    queryFn: () => get(`/v1/gardens/${toValue(gardenId)}/overview`, overviewSchema),
    refetchInterval: REFRESH_MS,
  })

export const useNodes = (gardenId: MaybeRefOrGetter<string>) =>
  useQuery({
    queryKey: computed(() => ['nodes', toValue(gardenId)]),
    queryFn: () => get(`/v1/gardens/${toValue(gardenId)}/nodes`, z.array(nodeSchema)),
    refetchInterval: REFRESH_MS,
  })

export const useNodeState = (gardenId: MaybeRefOrGetter<string>, nodeId: MaybeRefOrGetter<string | undefined>) =>
  useQuery({
    queryKey: computed(() => ['node-state', toValue(gardenId), toValue(nodeId)]),
    queryFn: () => get(`/v1/gardens/${toValue(gardenId)}/nodes/${toValue(nodeId)}/state`, nodeStateSchema),
    enabled: computed(() => !!toValue(nodeId)),
    refetchInterval: REFRESH_MS,
  })

export const useCommands = (gardenId: MaybeRefOrGetter<string>, nodeId?: MaybeRefOrGetter<string | undefined>) =>
  useQuery({
    queryKey: computed(() => ['commands', toValue(gardenId), toValue(nodeId) ?? 'all']),
    queryFn: () => get(`/v1/gardens/${toValue(gardenId)}/commands`, z.array(commandSchema), { nodeId: toValue(nodeId), limit: 200 }),
    refetchInterval: REFRESH_MS,
  })

export const useDecisions = (gardenId: MaybeRefOrGetter<string>, bedId: MaybeRefOrGetter<string>) =>
  useQuery({
    queryKey: computed(() => ['decisions', toValue(gardenId), toValue(bedId)]),
    queryFn: () => get(`/v1/gardens/${toValue(gardenId)}/beds/${toValue(bedId)}/decisions`, z.array(decisionSchema), { limit: 100 }),
    refetchInterval: REFRESH_MS,
  })

export interface AggregateArgs {
  gardenId: string
  nodeId: string | undefined
  kind: string
  bucket: '5m' | '15m' | '1h' | '24h'
  from: Date
  to: Date
}

export const useAggregate = (args: MaybeRefOrGetter<AggregateArgs>) =>
  useQuery({
    queryKey: computed(() => {
      const a = toValue(args)
      return ['aggregate', a.gardenId, a.nodeId, a.kind, a.bucket, a.from.toISOString()]
    }),
    queryFn: () => {
      const a = toValue(args)
      return get(`/v1/gardens/${a.gardenId}/nodes/${a.nodeId}/readings/aggregate`, z.array(aggregatePointSchema), {
        kind: a.kind, bucket: a.bucket, from: a.from.toISOString(), to: a.to.toISOString(),
      })
    },
    enabled: computed(() => !!toValue(args).nodeId),
    refetchInterval: 60_000,
  })

export function useRequestIrrigation(gardenId: MaybeRefOrGetter<string>) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (v: { nodeId: string; durationSeconds: number }) =>
      post(`/v1/gardens/${toValue(gardenId)}/nodes/${v.nodeId}/irrigations`, { durationSeconds: v.durationSeconds }, commandSchema),
    onSettled: () => qc.invalidateQueries(),
  })
}
