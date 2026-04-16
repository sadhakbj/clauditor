import { useQuery } from '@tanstack/react-query'
import type { UserMessage } from '@/types/api'

export function useSessionMessages(sessionId: string) {
  return useQuery<UserMessage[]>({
    queryKey: ['session-messages', sessionId],
    queryFn: async () => {
      const res = await fetch(`/api/sessions/${sessionId}/messages`)
      if (!res.ok) throw new Error('Messages not found')
      return res.json()
    },
    staleTime: 5 * 60 * 1000,
    retry: false,
  })
}
