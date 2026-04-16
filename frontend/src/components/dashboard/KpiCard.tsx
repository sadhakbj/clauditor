import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'

interface KpiCardProps {
  label: string
  value: string
  sub?: string
  accent?: 'violet' | 'emerald' | 'blue' | 'amber'
  loading?: boolean
}

const ACCENT_STYLES = {
  violet: 'text-violet-400',
  emerald: 'text-emerald-400',
  blue:    'text-blue-400',
  amber:   'text-amber-400',
}

export function KpiCard({ label, value, sub, accent, loading }: KpiCardProps) {
  if (loading) {
    return (
      <div className="rounded-xl border border-border/50 bg-card p-5">
        <Skeleton className="h-3 w-16 mb-3" />
        <Skeleton className="h-8 w-24 mb-2" />
        {sub && <Skeleton className="h-3 w-12" />}
      </div>
    )
  }

  return (
    <div className="rounded-xl border border-border/50 bg-card p-5 hover:border-border/80 transition-colors">
      <p className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
        {label}
      </p>
      <p className={cn(
        'mt-2 text-3xl font-bold tabular-nums leading-none',
        accent ? ACCENT_STYLES[accent] : 'text-foreground',
      )}>
        {value}
      </p>
      {sub && (
        <p className="mt-1.5 text-[11px] text-muted-foreground/60">{sub}</p>
      )}
    </div>
  )
}
