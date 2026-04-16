import { useEffect } from 'react'
import { useDashboardData } from '@/hooks/useDashboardData'
import { useFilterStore, RANGES } from '@/hooks/useFilterStore'
import { useDerivedMetrics } from '@/hooks/useDerivedMetrics'
import { FilterBar } from '@/components/layout/FilterBar'
import { KpiGrid } from './KpiGrid'
import { DailyChart } from './DailyChart'
import { DailyUsageTable } from './DailyUsageTable'
import { SessionsTable } from './SessionsTable'
import { ModelCostTable } from './ModelCostTable'

export function DashboardPage() {
  const { data, isLoading, error } = useDashboardData()
  const { range, selectedModels, selectedTools, initFilters } = useFilterStore()

  useEffect(() => {
    if (data) initFilters(data.all_models, data.all_tools)
  }, [data?.all_models, data?.all_tools]) // eslint-disable-line react-hooks/exhaustive-deps

  const rangeLabel = RANGES.find((r) => r.key === range)?.label ?? ''

  const {
    filteredSessions,
    dailyAgg,
    dailyCostAgg,
    byModel,
    byTool,
    totals,
    sessionCosts,
  } = useDerivedMetrics(
    data?.daily_by_model ?? [],
    data?.sessions_all ?? [],
    data?.all_models ?? [],
    data?.all_tools ?? [],
    range,
    selectedModels,
    selectedTools,
  )

  if (error) {
    return (
      <div className="p-8">
        <div className="rounded-xl border border-red-500/20 bg-red-500/5 p-5 text-sm">
          <p className="text-red-400 font-medium">Failed to load data</p>
          <p className="mt-1 text-red-400/60 text-xs">
            {error instanceof Error ? error.message : 'Unknown error'} — try running <code className="font-mono">clauditor scan</code>
          </p>
        </div>
      </div>
    )
  }

  return (
    <main className="flex-1 flex flex-col gap-6 px-6 md:px-8 py-6 w-full">

      {/* Page header with inline filters */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-lg font-semibold">Overview</h1>
          <p className="text-sm text-muted-foreground mt-0.5">
            {filteredSessions.length} sessions · {rangeLabel}
          </p>
        </div>
        <FilterBar
          allModels={data?.all_models ?? []}
          allTools={data?.all_tools ?? []}
        />
      </div>

      {/* KPI cards */}
      <KpiGrid totals={totals} byTool={byTool} loading={isLoading} />

      {/* Main chart */}
      <DailyChart dailyAgg={dailyAgg} dailyCostAgg={dailyCostAgg} loading={isLoading} />

      {/* Model breakdown + daily summary */}
      <div className="grid grid-cols-1 lg:grid-cols-5 gap-6">
        <div className="lg:col-span-3">
          <ModelCostTable byModel={byModel} />
        </div>
        <div className="lg:col-span-2">
          <DailyUsageTable daily={data?.daily_by_model ?? []} />
        </div>
      </div>

      {/* Sessions */}
      <SessionsTable sessions={filteredSessions} total={filteredSessions.length} sessionCosts={sessionCosts} />

      <footer className="border-t border-border/30 pt-4">
        <p className="text-[11px] text-muted-foreground/40">
          Cost estimates use Anthropic API and OpenAI API pricing. Actual subscription costs differ.
        </p>
      </footer>
    </main>
  )
}
