import { useQuery } from '@tanstack/react-query'
import type { SessionRequestsResponse } from '@/types/api'

export function useSessionRequests(id: string, page: number, limit = 20) {
  return useQuery<SessionRequestsResponse>({
    queryKey: ['session-requests', id, page, limit],
    queryFn: async () => {
      const res = await fetch(
        `/api/sessions/${encodeURIComponent(id)}/requests?page=${page}&limit=${limit}`
      )
      if (res.status === 404) throw new Error('Session not found')
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      return res.json()
    },
    staleTime: 5 * 60 * 1000,
    enabled: !!id,
  })
}
