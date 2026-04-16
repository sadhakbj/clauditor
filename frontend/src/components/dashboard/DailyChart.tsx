import { useState } from 'react'
import {
  BarChart, Bar, AreaChart, Area,
  XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer,
} from 'recharts'
import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import type { DailyAgg, DailyCostAgg } from '@/hooks/useDerivedMetrics'
import { fmt, fmtCostBig } from '@/lib/formatters'
import { cn } from '@/lib/utils'

interface DailyChartProps {
  dailyAgg: DailyAgg[]
  dailyCostAgg: DailyCostAgg[]
  loading?: boolean
}

type View = 'tokens' | 'cost'

const TOKEN_COLORS = {
  input:          '#6366f1',
  output:         '#a78bfa',
  cache_read:     '#34d399',
  cache_creation: '#fbbf24',
}

export function DailyChart({ dailyAgg, dailyCostAgg, loading }: DailyChartProps) {
  const [view, setView] = useState<View>('tokens')

  const tabClass = (v: View) => cn(
    'px-3 py-1 rounded-md text-[12px] font-medium transition-all',
    view === v
      ? 'bg-muted text-foreground'
      : 'text-muted-foreground hover:text-foreground',
  )

  return (
    <Card>
      <div className="flex items-center justify-between px-5 pt-4 pb-3 border-b border-border/50">
        <p className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
          Daily Usage
        </p>
        <div className="flex gap-1 bg-muted/40 rounded-lg p-0.5">
          <button className={tabClass('tokens')} onClick={() => setView('tokens')}>Tokens</button>
          <button className={tabClass('cost')} onClick={() => setView('cost')}>Cost</button>
        </div>
      </div>
      <CardContent className="px-2 pb-4 pt-4">
        {loading ? (
          <Skeleton className="h-64 w-full" />
        ) : view === 'tokens' ? (
          <ResponsiveContainer width="100%" height={260}>
            <BarChart data={dailyAgg} margin={{ top: 4, right: 8, left: 0, bottom: 0 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" vertical={false} />
              <XAxis
                dataKey="day"
                tick={{ fill: 'hsl(var(--muted-foreground))', fontSize: 11 }}
                tickLine={false}
                axisLine={false}
                interval="preserveStartEnd"
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
                  borderRadius: 8,
                  fontSize: 12,
                }}
                labelStyle={{ color: 'hsl(var(--foreground))', marginBottom: 4, fontWeight: 500 }}
                cursor={{ fill: 'hsl(var(--muted))', opacity: 0.4 }}
              />
              <Legend
                wrapperStyle={{ fontSize: 11, color: 'hsl(var(--muted-foreground))', paddingTop: 8 }}
                iconSize={8}
                iconType="circle"
              />
              <Bar dataKey="input" name="Input" stackId="s" fill={TOKEN_COLORS.input} />
              <Bar dataKey="output" name="Output" stackId="s" fill={TOKEN_COLORS.output} />
              <Bar dataKey="cache_read" name="Cache Read" stackId="s" fill={TOKEN_COLORS.cache_read} />
              <Bar dataKey="cache_creation" name="Cache Write" stackId="s" fill={TOKEN_COLORS.cache_creation} radius={[3, 3, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        ) : (
          <ResponsiveContainer width="100%" height={260}>
            <AreaChart data={dailyCostAgg} margin={{ top: 4, right: 8, left: 0, bottom: 0 }}>
              <defs>
                <linearGradient id="costGrad" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#34d399" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#34d399" stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" vertical={false} />
              <XAxis
                dataKey="day"
                tick={{ fill: 'hsl(var(--muted-foreground))', fontSize: 11 }}
                tickLine={false}
                axisLine={false}
                interval="preserveStartEnd"
              />
              <YAxis
                tickFormatter={(v) => fmtCostBig(v)}
                tick={{ fill: 'hsl(var(--muted-foreground))', fontSize: 11 }}
                tickLine={false}
                axisLine={false}
                width={60}
              />
              <Tooltip
                formatter={(v) => [fmtCostBig(Number(v ?? 0)), 'Est. Cost']}
                contentStyle={{
                  background: 'hsl(var(--card))',
                  border: '1px solid hsl(var(--border))',
                  borderRadius: 8,
                  fontSize: 12,
                }}
                labelStyle={{ color: 'hsl(var(--foreground))', marginBottom: 4, fontWeight: 500 }}
              />
              <Area
                type="monotone"
                dataKey="cost"
                name="Est. Cost"
                stroke="#34d399"
                strokeWidth={2}
                fill="url(#costGrad)"
                dot={false}
                activeDot={{ r: 4, fill: '#34d399' }}
              />
            </AreaChart>
          </ResponsiveContainer>
        )}
      </CardContent>
    </Card>
  )
}
