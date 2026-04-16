import { Card, CardContent } from '@/components/ui/card'
import type { ByTool } from '@/hooks/useDerivedMetrics'
import { fmt, fmtCostBig } from '@/lib/formatters'
import { toolLabel, toolColor } from '@/lib/toolUtils'

interface ToolCardsProps {
  byTool: ByTool[]
}

export function ToolCards({ byTool }: ToolCardsProps) {
  if (byTool.length === 0) return null

  return (
    <div className="grid gap-3" style={{ gridTemplateColumns: `repeat(${Math.min(byTool.length, 4)}, 1fr)` }}>
      {byTool.map((t, i) => (
        <Card key={t.tool}>
          <CardContent className="p-5">
            <div className="flex items-center gap-2 mb-3">
              <span
                className="w-2 h-2 rounded-full shrink-0"
                style={{ background: toolColor(t.tool, i) }}
              />
              <span className="text-sm font-semibold">{toolLabel(t.tool)}</span>
            </div>
            <dl className="flex flex-col gap-1">
              <Row label="Sessions" value={t.sessions.toLocaleString()} />
              <Row label="Turns" value={fmt(t.turns)} />
              <Row label="Input" value={fmt(t.input)} />
              <Row label="Output" value={fmt(t.output)} />
              <Row
                label="Est. Cost"
                value={t.cost > 0 ? fmtCostBig(t.cost) : '—'}
                accent={t.cost > 0}
              />
            </dl>
          </CardContent>
        </Card>
      ))}
    </div>
  )
}

function Row({ label, value, accent }: { label: string; value: string; accent?: boolean }) {
  return (
    <div className="flex justify-between text-xs">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className={accent ? 'text-emerald-500 font-mono' : 'font-mono'}>{value}</dd>
    </div>
  )
}
