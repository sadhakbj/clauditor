import { PieChart, Pie, Cell, Tooltip, Legend, ResponsiveContainer } from 'recharts'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import type { ByModel } from '@/hooks/useDerivedMetrics'
import { CHART_COLORS } from '@/lib/modelUtils'
import { fmt } from '@/lib/formatters'

interface ModelDonutChartProps {
  byModel: ByModel[]
  loading?: boolean
}

export function ModelDonutChart({ byModel, loading }: ModelDonutChartProps) {
  const data = byModel.map((m) => ({
    name: m.model,
    value: m.input + m.output,
  }))

  return (
    <Card>
      <CardHeader className="pb-2 pt-4 px-5">
        <p className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">
          By Model
        </p>
      </CardHeader>
      <CardContent className="px-2 pb-4">
        {loading ? (
          <Skeleton className="h-64 w-full rounded-full mx-auto aspect-square max-w-48" />
        ) : (
          <ResponsiveContainer width="100%" height={260}>
            <PieChart>
              <Pie
                data={data}
                cx="50%"
                cy="45%"
                innerRadius={60}
                outerRadius={90}
                paddingAngle={2}
                dataKey="value"
              >
                {data.map((_, i) => (
                  <Cell key={i} fill={CHART_COLORS[i % CHART_COLORS.length]} strokeWidth={0} />
                ))}
              </Pie>
              <Tooltip
                formatter={(v) => fmt(Number(v ?? 0))}
                contentStyle={{
                  background: 'hsl(var(--card))',
                  border: '1px solid hsl(var(--border))',
                  borderRadius: 6,
                  fontSize: 12,
                }}
              />
              <Legend
                wrapperStyle={{ fontSize: 10, color: 'hsl(var(--muted-foreground))' }}
                iconSize={8}
                iconType="circle"
              />
            </PieChart>
          </ResponsiveContainer>
        )}
      </CardContent>
    </Card>
  )
}
