import { Card } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import type { ByModel } from '@/hooks/useDerivedMetrics'
import { fmt, fmtCostBig } from '@/lib/formatters'
import { calcCost, isBillable } from '@/lib/pricing'
import { modelFamily, MODEL_FAMILY_COLORS } from '@/lib/modelUtils'

interface ModelCostTableProps {
  byModel: ByModel[]
}

export function ModelCostTable({ byModel }: ModelCostTableProps) {
  const costs = byModel.map((m) => calcCost(m.model, m.input, m.output, m.cache_read, m.cache_creation))
  const maxCost = Math.max(...costs, 0.001)

  if (byModel.length === 0) return null

  return (
    <Card className="overflow-hidden">
      <div className="px-5 py-3.5 border-b border-border/50">
        <p className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
          By Model
        </p>
      </div>
      <div className="divide-y divide-border/40">
        {byModel.map((m, i) => {
          const fam = modelFamily(m.model)
          const cost = costs[i]
          const pct = maxCost > 0 ? (cost / maxCost) * 100 : 0
          const color = MODEL_FAMILY_COLORS[fam]

          return (
            <div key={m.model} className="px-5 py-3.5">
              <div className="flex items-center justify-between mb-2">
                <Badge
                  variant="outline"
                  className="gap-1.5 font-normal text-[11px] border-0 pl-0"
                  style={{ color }}
                >
                  <span className="w-1.5 h-1.5 rounded-full shrink-0" style={{ background: color }} />
                  {m.model}
                </Badge>
                <div className="flex items-center gap-4 text-xs text-muted-foreground">
                  <span>{fmt(m.turns)} turns</span>
                  <span>{fmt(m.input + m.output)} tokens</span>
                  <span className={isBillable(m.model) ? 'text-emerald-400 font-medium' : ''}>
                    {isBillable(m.model) ? fmtCostBig(cost) : '—'}
                  </span>
                </div>
              </div>
              {isBillable(m.model) && (
                <div className="h-1 rounded-full bg-muted/50 overflow-hidden">
                  <div
                    className="h-full rounded-full transition-all duration-500"
                    style={{ width: `${pct}%`, background: color, opacity: 0.7 }}
                  />
                </div>
              )}
            </div>
          )
        })}
      </div>
    </Card>
  )
}
