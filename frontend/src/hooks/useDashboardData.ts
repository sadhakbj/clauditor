import { useQuery, queryOptions } from '@tanstack/react-query'
import type { DashboardData } from '@/types/api'

export const dashboardQueryOptions = queryOptions({
  queryKey: ['dashboard'],
  queryFn: async (): Promise<DashboardData> => {
    const res = await fetch('/api/data')
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    const data: DashboardData = await res.json()
    if (data.error) throw new Error(data.error)
    return data
  },
  refetchInterval: 60_000,
  refetchIntervalInBackground: false,
  staleTime: 30_000,
})

export function useDashboardData() {
  return useQuery(dashboardQueryOptions)
}
