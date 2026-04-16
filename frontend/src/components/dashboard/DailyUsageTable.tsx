import { useMemo } from 'react'
import { Card } from '@/components/ui/card'
import type { DailyModelRow } from '@/types/api'
import { fmt, fmtCostBig } from '@/lib/formatters'
import { calcCost } from '@/lib/pricing'

interface DailyUsageTableProps {
  daily: DailyModelRow[]
}

interface DailyTotal {
  day: string
  input: number
  output: number
  turns: number
  cost: number
}

export function DailyUsageTable({ daily }: DailyUsageTableProps) {
  const rows = useMemo((): DailyTotal[] => {
    const days = new Set<string>()
    for (let i = 0; i < 7; i++) {
      const d = new Date()
      d.setDate(d.getDate() - i)
      days.add(d.toISOString().slice(0, 10))
    }

    const map: Record<string, DailyTotal> = {}
    for (const r of daily) {
      if (!days.has(r.day)) continue
      if (!map[r.day]) map[r.day] = { day: r.day, input: 0, output: 0, turns: 0, cost: 0 }
      map[r.day].input += r.input
      map[r.day].output += r.output
      map[r.day].turns += r.turns
      map[r.day].cost += calcCost(r.model, r.input, r.output, r.cache_read, r.cache_creation)
    }

    return Array.from(days)
      .sort((a, b) => b.localeCompare(a))
      .map((day) => map[day] ?? { day, input: 0, output: 0, turns: 0, cost: 0 })
  }, [daily])

  const maxCost = Math.max(...rows.map((r) => r.cost), 0.001)

  return (
    <Card className="overflow-hidden">
      <div className="px-5 py-3.5 border-b border-border/50">
        <p className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
          Last 7 Days
        </p>
      </div>
      <div className="divide-y divide-border/30">
        {rows.map((r) => {
          const isEmpty = r.turns === 0
          const pct = maxCost > 0 ? (r.cost / maxCost) * 100 : 0
          const dayLabel = new Date(r.day + 'T12:00:00').toLocaleDateString(undefined, {
            weekday: 'short', month: 'short', day: 'numeric',
          })

          return (
            <div key={r.day} className={`px-5 py-3 ${isEmpty ? 'opacity-35' : ''}`}>
              <div className="flex items-center justify-between mb-1.5">
                <span className="text-[13px] font-medium">{dayLabel}</span>
                <div className="flex items-center gap-3 text-[12px] text-muted-foreground">
                  {!isEmpty && (
                    <>
                      <span>{r.turns} turns</span>
                      <span>{fmt(r.input + r.output)} tok</span>
                    </>
                  )}
                  <span className={r.cost > 0 ? 'text-emerald-400 font-medium' : ''}>
                    {r.cost > 0 ? fmtCostBig(r.cost) : '—'}
                  </span>
                </div>
              </div>
              {!isEmpty && (
                <div className="h-1 rounded-full bg-muted/40 overflow-hidden">
                  <div
                    className="h-full rounded-full bg-emerald-500/60 transition-all duration-500"
                    style={{ width: `${pct}%` }}
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
