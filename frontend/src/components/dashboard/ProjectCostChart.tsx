import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Cell,
} from 'recharts'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import type { ByProject } from '@/hooks/useDerivedMetrics'
import { CHART_COLORS } from '@/lib/modelUtils'

interface ProjectCostChartProps {
  byProject: ByProject[]
  loading?: boolean
}

export function ProjectCostChart({ byProject, loading }: ProjectCostChartProps) {
  const data = byProject.map((p) => ({
    name: p.project.length > 26 ? '…' + p.project.slice(-24) : p.project,
    cost: +p.cost.toFixed(4),
  }))

  return (
    <Card>
      <CardHeader className="pb-2 pt-4 px-5">
        <p className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">
          Top Projects by Cost
        </p>
      </CardHeader>
      <CardContent className="px-2 pb-4">
        {loading ? (
          <Skeleton className="h-52 w-full" />
        ) : (
          <ResponsiveContainer width="100%" height={210}>
            <BarChart data={data} layout="vertical" margin={{ top: 0, right: 16, left: 0, bottom: 0 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" horizontal={false} />
              <XAxis
                type="number"
                tickFormatter={(v) => '$' + v}
                tick={{ fill: 'hsl(var(--muted-foreground))', fontSize: 10 }}
                tickLine={false}
                axisLine={false}
              />
              <YAxis
                type="category"
                dataKey="name"
                width={110}
                tick={{ fill: 'hsl(var(--muted-foreground))', fontSize: 10 }}
                tickLine={false}
                axisLine={false}
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
              <Bar dataKey="cost" radius={[0, 3, 3, 0]}>
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
