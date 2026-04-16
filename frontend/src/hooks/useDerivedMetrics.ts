import { useMemo } from 'react'
import type { DailyModelRow, SessionRow } from '@/types/api'
import type { TimeRange } from './useFilterStore'
import { calcCost, isBillable } from '@/lib/pricing'
import { modelPriority } from '@/lib/modelUtils'

function localDateStr(d: Date): string {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

export function getCutoffDate(range: TimeRange): string | null {
  if (range === 'all') return null
  if (range === 'today') return localDateStr(new Date())
  if (range === 'yesterday') {
    const d = new Date()
    d.setDate(d.getDate() - 1)
    return localDateStr(d)
  }
  const days = { '7d': 7, '30d': 30, '90d': 90 }[range as '7d' | '30d' | '90d']
  const d = new Date()
  d.setDate(d.getDate() - days)
  return localDateStr(d)
}

export interface DailyAgg {
  day: string
  input: number
  output: number
  cache_read: number
  cache_creation: number
}

export interface DailyCostAgg {
  day: string
  cost: number
}

export interface ByModel {
  model: string
  input: number
  output: number
  cache_read: number
  cache_creation: number
  turns: number
}

export interface ByTool {
  tool: string
  sessions: number
  turns: number
  input: number
  output: number
  cache_read: number
  cache_creation: number
  cost: number
}

export interface ByProject {
  project: string
  input: number
  output: number
  cost: number
}

export interface Totals {
  sessions: number
  turns: number
  input: number
  output: number
  cache_read: number
  cache_creation: number
  cost: number
}

function filterDailyRows(
  daily: DailyModelRow[],
  cutoff: string | null,
  ceiling: string | null,
  selectedModels: Set<string>,
  selectedTools: Set<string>,
  allTools: string[],
): DailyModelRow[] {
  const noToolFilter = selectedTools.size === 0 || selectedTools.size === allTools.length
  return daily.filter(
    (r) =>
      selectedModels.has(r.model) &&
      (!cutoff || r.day >= cutoff) &&
      (!ceiling || r.day <= ceiling) &&
      (noToolFilter || selectedTools.has(r.tool || 'claude_code')),
  )
}

function filterSessions(
  sessions: SessionRow[],
  cutoff: string | null,
  ceiling: string | null,
  selectedModels: Set<string>,
  selectedTools: Set<string>,
  allTools: string[],
): SessionRow[] {
  const noToolFilter = selectedTools.size === 0 || selectedTools.size === allTools.length
  return sessions.filter(
    (s) =>
      selectedModels.has(s.model) &&
      (!cutoff || s.last_date >= cutoff) &&
      (!ceiling || s.last_date <= ceiling) &&
      (noToolFilter || selectedTools.has(s.tool || 'claude_code')),
  )
}

export function useDerivedMetrics(
  daily: DailyModelRow[],
  sessions: SessionRow[],
  allModels: string[],
  allTools: string[],
  range: TimeRange,
  selectedModels: Set<string>,
  selectedTools: Set<string>,
) {
  const cutoff = useMemo(() => getCutoffDate(range), [range])

  const ceiling = useMemo(() => {
    if (range !== 'yesterday') return null
    const d = new Date()
    d.setDate(d.getDate() - 1)
    return d.toISOString().slice(0, 10)
  }, [range])

  const sortedModels = useMemo(
    () => [...allModels].sort((a, b) => modelPriority(a) - modelPriority(b) || a.localeCompare(b)),
    [allModels],
  )

  const filteredDaily = useMemo(
    () => filterDailyRows(daily, cutoff, ceiling, selectedModels, selectedTools, allTools),
    [daily, cutoff, ceiling, selectedModels, selectedTools, allTools],
  )

  const filteredSessions = useMemo(
    () => filterSessions(sessions, cutoff, ceiling, selectedModels, selectedTools, allTools),
    [sessions, cutoff, ceiling, selectedModels, selectedTools, allTools],
  )

  const dailyAgg = useMemo((): DailyAgg[] => {
    const map: Record<string, DailyAgg> = {}
    for (const r of filteredDaily) {
      if (!map[r.day])
        map[r.day] = { day: r.day, input: 0, output: 0, cache_read: 0, cache_creation: 0 }
      map[r.day].input += r.input
      map[r.day].output += r.output
      map[r.day].cache_read += r.cache_read
      map[r.day].cache_creation += r.cache_creation
    }
    return Object.values(map).sort((a, b) => a.day.localeCompare(b.day))
  }, [filteredDaily])

  const byModel = useMemo((): ByModel[] => {
    const map: Record<string, ByModel> = {}
    for (const r of filteredDaily) {
      if (!map[r.model])
        map[r.model] = { model: r.model, input: 0, output: 0, cache_read: 0, cache_creation: 0, turns: 0 }
      map[r.model].input += r.input
      map[r.model].output += r.output
      map[r.model].cache_read += r.cache_read
      map[r.model].cache_creation += r.cache_creation
      map[r.model].turns += r.turns
    }
    return Object.values(map).sort((a, b) => b.input + b.output - (a.input + a.output))
  }, [filteredDaily])

  // Per-session cost derived from already-date-filtered daily rows
  const sessionCosts = useMemo((): Record<string, number> => {
    const map: Record<string, number> = {}
    for (const r of filteredDaily) {
      if (r.session_id) {
        map[r.session_id] = (map[r.session_id] ?? 0) + calcCost(r.model, r.input, r.output, r.cache_read, r.cache_creation)
      }
    }
    return map
  }, [filteredDaily])

  const byProject = useMemo((): ByProject[] => {
    const map: Record<string, ByProject> = {}
    for (const s of filteredSessions) {
      if (!map[s.project]) map[s.project] = { project: s.project, input: 0, output: 0, cost: 0 }
      map[s.project].input += s.input
      map[s.project].output += s.output
      map[s.project].cost += sessionCosts[s.session_id] ?? 0
    }
    return Object.values(map).sort((a, b) => b.cost - a.cost).slice(0, 8)
  }, [filteredSessions, sessionCosts])

  const byTool = useMemo((): ByTool[] => {
    const map: Record<string, ByTool> = {}
    for (const r of filteredDaily) {
      const t = r.tool || 'claude_code'
      if (!map[t]) map[t] = { tool: t, turns: 0, input: 0, output: 0, cache_read: 0, cache_creation: 0, cost: 0, sessions: 0 }
      map[t].input += r.input
      map[t].output += r.output
      map[t].cache_read += r.cache_read
      map[t].cache_creation += r.cache_creation
      map[t].turns += r.turns
      map[t].cost += calcCost(r.model, r.input, r.output, r.cache_read, r.cache_creation)
    }
    const sessionsByTool: Record<string, number> = {}
    for (const s of filteredSessions) {
      const t = s.tool || 'claude_code'
      sessionsByTool[t] = (sessionsByTool[t] ?? 0) + 1
    }
    return Object.values(map)
      .map((t) => ({ ...t, sessions: sessionsByTool[t.tool] ?? 0 }))
      .sort((a, b) => a.tool.localeCompare(b.tool))
  }, [filteredDaily, filteredSessions])

  const totals = useMemo((): Totals => ({
    sessions: filteredSessions.length,
    turns: byModel.reduce((s, m) => s + m.turns, 0),
    input: byModel.reduce((s, m) => s + m.input, 0),
    output: byModel.reduce((s, m) => s + m.output, 0),
    cache_read: byModel.reduce((s, m) => s + m.cache_read, 0),
    cache_creation: byModel.reduce((s, m) => s + m.cache_creation, 0),
    cost: byModel.reduce(
      (s, m) => s + calcCost(m.model, m.input, m.output, m.cache_read, m.cache_creation),
      0,
    ),
  }), [byModel, filteredSessions])

  const billableByModel = useMemo(
    () => byModel.filter((m) => isBillable(m.model)),
    [byModel],
  )

  const dailyCostAgg = useMemo((): DailyCostAgg[] => {
    const map: Record<string, number> = {}
    for (const r of filteredDaily) {
      map[r.day] = (map[r.day] ?? 0) + calcCost(r.model, r.input, r.output, r.cache_read, r.cache_creation)
    }
    return Object.entries(map)
      .map(([day, cost]) => ({ day, cost }))
      .sort((a, b) => a.day.localeCompare(b.day))
  }, [filteredDaily])

  return {
    sortedModels,
    filteredDaily,
    filteredSessions,
    dailyAgg,
    dailyCostAgg,
    byModel,
    byProject,
    byTool,
    totals,
    billableByModel,
    sessionCosts,
  }
}
