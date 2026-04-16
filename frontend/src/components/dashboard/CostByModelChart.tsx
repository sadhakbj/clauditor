import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Cell } from 'recharts'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import type { ByModel } from '@/hooks/useDerivedMetrics'
import { calcCost } from '@/lib/pricing'
import { CHART_COLORS } from '@/lib/modelUtils'

interface CostByModelChartProps {
  billableByModel: ByModel[]
  loading?: boolean
}

export function CostByModelChart({ billableByModel, loading }: CostByModelChartProps) {
  const data = billableByModel.map((m) => ({
    name: m.model.length > 20 ? '…' + m.model.slice(-18) : m.model,
    cost: +calcCost(m.model, m.input, m.output, m.cache_read, m.cache_creation).toFixed(4),
  }))

  return (
    <Card>
      <CardHeader className="pb-2 pt-4 px-5">
        <p className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">
          Cost by Model
        </p>
      </CardHeader>
      <CardContent className="px-2 pb-4">
        {loading ? (
          <Skeleton className="h-52 w-full" />
        ) : (
          <ResponsiveContainer width="100%" height={210}>
            <BarChart data={data} margin={{ top: 4, right: 8, left: 0, bottom: 0 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" vertical={false} />
              <XAxis
                dataKey="name"
                tick={{ fill: 'hsl(var(--muted-foreground))', fontSize: 10 }}
                tickLine={false}
                axisLine={false}
              />
              <YAxis
                tickFormatter={(v) => '$' + v}
                tick={{ fill: 'hsl(var(--muted-foreground))', fontSize: 10 }}
                tickLine={false}
                axisLine={false}
                width={52}
              />
              <Tooltip
                formatter={(v) => ['$' + Number(v ?? 0).toFixed(4), 'Est. Cost']}
                contentStyle={{
                  background: 'hsl(var(--card))',
                  border: '1px solid hsl(var(--border))',
                  borderRadius: 6,
                  fontSize: 12,
                }}
              />
              <Bar dataKey="cost" radius={[3, 3, 0, 0]}>
                {data.map((_, i) => (
                  <Cell key={i} fill={CHART_COLORS[i % CHART_COLORS.length]} />
                ))}
              </Bar>
            </BarChart>
          </ResponsiveContainer>
        )}
      </CardContent>
    </Card>
  )
}
