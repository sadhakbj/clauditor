import { KpiCard } from './KpiCard'
import type { Totals, ByTool } from '@/hooks/useDerivedMetrics'
import { fmt, fmtCostBig } from '@/lib/formatters'
import { toolLabel, toolColor } from '@/lib/toolUtils'

interface KpiGridProps {
  totals: Totals
  byTool: ByTool[]
  loading?: boolean
}

export function KpiGrid({ totals, byTool, loading }: KpiGridProps) {
  const totalTokens = totals.input + totals.output

  // Tools card content
  const activeTools = byTool.filter((t) => t.turns > 0)

  return (
    <div className="grid grid-cols-2 gap-3 sm:grid-cols-5">
      <KpiCard
        label="Sessions"
        value={totals.sessions.toLocaleString()}
        loading={loading}
      />
      <KpiCard
        label="Turns"
        value={fmt(totals.turns)}
        loading={loading}
      />
      <KpiCard
        label="Tokens"
        value={fmt(totalTokens)}
        sub={`${fmt(totals.input)} in · ${fmt(totals.output)} out`}
        loading={loading}
      />
      <KpiCard
        label="Est. Cost"
        value={fmtCostBig(totals.cost)}
        sub="API pricing"
        accent="emerald"
        loading={loading}
      />

      {/* Tools card — custom rendering */}
      {loading ? (
        <div className="rounded-xl border border-border/50 bg-card p-5 animate-pulse">
          <div className="h-3 w-10 bg-muted rounded mb-3" />
          <div className="h-8 w-6 bg-muted rounded mb-2" />
          <div className="h-3 w-20 bg-muted rounded" />
        </div>
      ) : (
        <div className="rounded-xl border border-border/50 bg-card p-5 hover:border-border/80 transition-colors">
          <p className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
            Tools
          </p>
          <p className="mt-2 text-3xl font-bold tabular-nums leading-none">
            {activeTools.length || '—'}
          </p>
          {activeTools.length > 0 && (
            <div className="mt-2 flex flex-col gap-1">
              {activeTools.map((t, i) => (
                <span
                  key={t.tool}
                  className="text-[11px] font-medium flex items-center gap-1.5"
                  style={{ color: toolColor(t.tool, i) }}
                >
                  <span
                    className="w-1.5 h-1.5 rounded-full shrink-0"
                    style={{ background: toolColor(t.tool, i) }}
                  />
                  {toolLabel(t.tool)}
                </span>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  )
}
