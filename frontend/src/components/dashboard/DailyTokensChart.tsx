import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
} from 'recharts'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import type { DailyAgg } from '@/hooks/useDerivedMetrics'
import { fmt } from '@/lib/formatters'
import type { TimeRange } from '@/hooks/useFilterStore'
import { RANGES } from '@/hooks/useFilterStore'

interface DailyTokensChartProps {
  data: DailyAgg[]
  range: TimeRange
  loading?: boolean
}

const COLORS = {
  input:          'rgba(59,130,246,0.8)',
  output:         'rgba(139,92,246,0.8)',
  cache_read:     'rgba(16,185,129,0.6)',
  cache_creation: 'rgba(245,158,11,0.6)',
}

const MAX_TICKS: Record<TimeRange, number> = {
  today: 1, yesterday: 1, '7d': 7, '30d': 15, '90d': 13, all: 12,
}

export function DailyTokensChart({ data, range, loading }: DailyTokensChartProps) {
  const rangeLabel = RANGES.find((r) => r.key === range)?.label ?? ''

  return (
    <Card className="flex-1">
      <CardHeader className="pb-2 pt-4 px-5">
        <p className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">
          Daily Token Usage — {rangeLabel}
        </p>
      </CardHeader>
      <CardContent className="px-2 pb-4">
        {loading ? (
          <Skeleton className="h-64 w-full" />
        ) : (
          <ResponsiveContainer width="100%" height={260}>
            <BarChart data={data} margin={{ top: 4, right: 8, left: 0, bottom: 0 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" />
              <XAxis
                dataKey="day"
                tick={{ fill: 'hsl(var(--muted-foreground))', fontSize: 11 }}
                tickLine={false}
                axisLine={false}
                interval="preserveStartEnd"
                tickCount={MAX_TICKS[range]}
              />
              <YAxis
                tickFormatter={fmt}
                tick={{ fill: 'hsl(var(--muted-foreground))', fontSize: 11 }}
                tickLine={false}
                axisLine={false}
                width={52}
              />
              <Tooltip
                formatter={(v, name) => [fmt(Number(v ?? 0)), name]}
                contentStyle={{
                  background: 'hsl(var(--card))',
                  border: '1px solid hsl(var(--border))',
                  borderRadius: 6,
                  fontSize: 12,
                }}
                labelStyle={{ color: 'hsl(var(--foreground))', marginBottom: 4 }}
              />
              <Legend
                wrapperStyle={{ fontSize: 11, color: 'hsl(var(--muted-foreground))' }}
                iconSize={9}
              />
              <Bar dataKey="input" name="Input" stackId="s" fill={COLORS.input} />
              <Bar dataKey="output" name="Output" stackId="s" fill={COLORS.output} />
              <Bar dataKey="cache_read" name="Cache Read" stackId="s" fill={COLORS.cache_read} />
              <Bar dataKey="cache_creation" name="Cache Write" stackId="s" fill={COLORS.cache_creation} radius={[2, 2, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        )}
      </CardContent>
    </Card>
  )
}
