import { createFileRoute } from '@tanstack/react-router'
import { SessionDetailPage } from '@/components/session-detail/SessionDetailPage'

export const Route = createFileRoute('/sessions/$id')({
  component: SessionDetailPage,
})
