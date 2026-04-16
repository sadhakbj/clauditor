import { useNavigate } from '@tanstack/react-router'
import { Card } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import type { SessionRow } from '@/types/api'
import { fmtTs, fmtCostBig } from '@/lib/formatters'
import { modelFamily, MODEL_FAMILY_COLORS } from '@/lib/modelUtils'
import { ArrowRight } from 'lucide-react'

interface SessionsTableProps {
  sessions: SessionRow[]
  total: number
  sessionCosts: Record<string, number>
}

const VISIBLE = 30

export function SessionsTable({ sessions, total, sessionCosts }: SessionsTableProps) {
  const navigate = useNavigate()
  const visible = sessions.slice(0, VISIBLE)

  return (
    <Card className="overflow-hidden">
      <div className="flex items-center justify-between px-5 py-3.5 border-b border-border/50">
        <p className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
          Recent Sessions
        </p>
        <span className="text-[11px] text-muted-foreground">
          {visible.length} of {total}
        </span>
      </div>

      <div className="divide-y divide-border/30">
        {visible.length === 0 ? (
          <div className="py-12 text-center text-sm text-muted-foreground">
            No sessions found
          </div>
        ) : (
          visible.map((s) => {
            const cost = sessionCosts[s.session_id] ?? s.total_cost_usd
            const fam = modelFamily(s.model)
            const color = MODEL_FAMILY_COLORS[fam]

            return (
              <div
                key={s.session_id}
                className="flex items-center gap-4 px-5 py-3.5 hover:bg-muted/20 cursor-pointer group transition-colors"
                onClick={() => navigate({ to: '/sessions/$id', params: { id: s.session_id } })}
              >
                {/* Project */}
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium truncate">{s.project}</p>
                  <p className="text-[11px] text-muted-foreground mt-0.5">{fmtTs(s.last_ts)}</p>
                </div>

                {/* Model badge */}
                <Badge
                  variant="outline"
                  className="gap-1.5 font-normal text-[11px] shrink-0 hidden sm:flex"
                  style={{ borderColor: color + '30', color }}
                >
                  <span className="w-1 h-1 rounded-full" style={{ background: color }} />
                  {s.model}
                </Badge>

                {/* Stats */}
                <div className="flex items-center gap-4 shrink-0">
                  <span className="text-xs text-muted-foreground hidden md:block">
                    {s.turns} turns
                  </span>
                  <span className="text-xs text-muted-foreground hidden md:block">
                    {s.duration_min}m
                  </span>
                  <span className={`text-xs font-medium w-14 text-right ${cost > 0 ? 'text-emerald-400' : 'text-muted-foreground'}`}>
                    {cost > 0 ? fmtCostBig(cost) : '—'}
                  </span>
                </div>

                <ArrowRight size={13} className="text-muted-foreground/30 group-hover:text-muted-foreground shrink-0 transition-colors" />
              </div>
            )
          })
        )}
      </div>
    </Card>
  )
}
