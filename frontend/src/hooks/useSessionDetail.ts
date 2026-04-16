import { useQuery } from '@tanstack/react-query'
import type { SessionDetail } from '@/types/api'

export function useSessionDetail(id: string) {
  return useQuery<SessionDetail>({
    queryKey: ['session', id],
    queryFn: async () => {
      const res = await fetch(`/api/sessions/${encodeURIComponent(id)}`)
      if (res.status === 404) throw new Error('Session not found')
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      return res.json()
    },
    staleTime: 5 * 60 * 1000,
    enabled: !!id,
  })
}
