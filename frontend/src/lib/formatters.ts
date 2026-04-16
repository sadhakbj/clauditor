import { format } from 'date-fns'

/** Format a Unix epoch (seconds) as "YYYY-MM-DD HH:mm:ss" in local time. */
export function fmtTs(epochSec: number): string {
  if (!epochSec) return ''
  return format(new Date(epochSec * 1000), 'yyyy-MM-dd HH:mm:ss')
}

export function fmt(n: number): string {
  if (n >= 1e9) return (n / 1e9).toFixed(2) + 'B'
  if (n >= 1e6) return (n / 1e6).toFixed(2) + 'M'
  if (n >= 1e3) return (n / 1e3).toFixed(1) + 'K'
  return n.toLocaleString()
}

export function fmtCost(c: number): string {
  return '$' + c.toFixed(4)
}

export function fmtCostBig(c: number): string {
  return '$' + c.toFixed(2)
}

export function fmtDuration(minutes: number): string {
  if (minutes < 1) return '< 1 min'
  if (minutes < 60) return `${Math.round(minutes)} min`
  const h = Math.floor(minutes / 60)
  const m = Math.round(minutes % 60)
  return m > 0 ? `${h}h ${m}m` : `${h}h`
}
