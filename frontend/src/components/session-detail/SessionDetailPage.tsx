import { useState } from 'react'
import { useParams, Link } from '@tanstack/react-router'
import {
  ArrowLeft, ChevronLeft, ChevronRight, X,
  MessageSquare, Brain, Bot,
  Terminal, FileText, FilePen, Search, Wrench, Cpu, Globe,
  NotebookPen,
} from 'lucide-react'
import { useSessionRequests } from '@/hooks/useSessionRequests'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { Sheet, SheetContent, SheetClose } from '@/components/ui/sheet'
import { fmt, fmtCost, fmtCostBig, fmtTs, fmtDuration } from '@/lib/formatters'
import { isBillable } from '@/lib/pricing'
import { modelFamily, MODEL_FAMILY_COLORS } from '@/lib/modelUtils'
import { toolLabel, toolColor } from '@/lib/toolUtils'
import type { RequestGroup, TurnDetailRow } from '@/types/api'
import { cn } from '@/lib/utils'

const GROUPS_PER_PAGE = 20

// ── Tool icon map ─────────────────────────────────────────────────────────────

function ToolIcon({ name, size = 12 }: { name: string | null; size?: number }) {
  const n = (name ?? '').toLowerCase()
  if (n.includes('bash') || n.includes('shell')) return <Terminal size={size} />
  if (n.includes('read')) return <FileText size={size} />
  if (n.includes('write')) return <FilePen size={size} />
  if (n.includes('edit')) return <FilePen size={size} />
  if (n.includes('glob') || n.includes('grep') || n.includes('search')) return <Search size={size} />
  if (n.includes('web') || n.includes('fetch')) return <Globe size={size} />
  if (n.includes('notebook')) return <NotebookPen size={size} />
  if (!name) return <Cpu size={size} />
  return <Wrench size={size} />
}

// ── Unified timeline sheet ────────────────────────────────────────────────────


function fmtElapsed(sec: number): string {
  if (sec <= 0) return ''
  if (sec < 60) return `${sec}s`
  const m = Math.floor(sec / 60)
  const s = sec % 60
  return s > 0 ? `${m}m ${s}s` : `${m}m`
}

function RequestSheet({ group, open, onClose }: { group: RequestGroup; open: boolean; onClose: () => void }) {
  const totalCost = group.cost_usd
  const billable = isBillable(group.turns[0]?.model)
  const thinkingSec = group.thinking_tokens > 0 ? Math.max(1, Math.round(group.thinking_tokens / 50)) : 0
  const elapsedSec = group.elapsed_sec ?? 0

  // Build the timeline nodes
  type Node =
    | { kind: 'message' }
    | { kind: 'thinking' }
    | { kind: 'call'; turn: TurnDetailRow; idx: number }
    | { kind: 'response' }

  const nodes: Node[] = []
  if (group.message) nodes.push({ kind: 'message' })
  if (group.thinking_tokens > 0) nodes.push({ kind: 'thinking' })
  group.turns.forEach((turn, idx) => nodes.push({ kind: 'call', turn, idx }))
  if (group.assistant_response) nodes.push({ kind: 'response' })

  return (
    <Sheet open={open} onOpenChange={(o) => { if (!o) onClose() }}>
      <SheetContent
        side="right"
        className="flex flex-col p-0 !w-[min(92vw,860px)] sm:!max-w-[min(92vw,860px)]"
      >
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-border/40 shrink-0">
          <div className="flex items-center gap-3 min-w-0">
            <MessageSquare size={14} className="text-violet-400 shrink-0" />
            <span className="text-sm font-semibold shrink-0">Request timeline</span>
            <span className="text-[11px] text-muted-foreground bg-muted/40 px-2 py-0.5 rounded-full shrink-0">
              {group.turns.length} call{group.turns.length !== 1 ? 's' : ''}
            </span>
            {elapsedSec > 0 && (
              <span className="text-[12px] font-medium text-muted-foreground/60 shrink-0">
                Crunched for <span className="text-foreground/80 font-semibold">{fmtElapsed(elapsedSec)}</span>
              </span>
            )}
          </div>
          <div className="flex items-center gap-4 shrink-0">
            {billable && (
              <span className="text-[13px] font-mono text-emerald-400 font-semibold">{fmtCost(totalCost)}</span>
            )}
            <SheetClose className="p-1.5 rounded-md text-muted-foreground hover:text-foreground hover:bg-muted/50 transition-colors">
              <X size={14} />
            </SheetClose>
          </div>
        </div>

        {/* Timeline */}
        <div className="flex-1 overflow-y-auto px-6 py-6">
          <div className="flex flex-col">
            {nodes.map((node, ni) => {
              const isLast = ni === nodes.length - 1

              // ── Your message ──────────────────────────────────────────────
              if (node.kind === 'message') {
                return (
                  <div key="msg" className="flex gap-4">
                    <div className="flex flex-col items-center w-8 shrink-0">
                      <div className="w-8 h-8 rounded-full bg-violet-500/15 border-2 border-violet-500/40 flex items-center justify-center shrink-0">
                        <MessageSquare size={13} className="text-violet-400" />
                      </div>
                      {!isLast && <div className="w-px flex-1 bg-border/30 mt-1" />}
                    </div>
                    <div className={cn('flex-1 min-w-0 pb-6')}>
                      <div className="flex items-center justify-between mb-2">
                        <span className="text-[11px] font-semibold uppercase tracking-wider text-violet-400">You</span>
                        <span className="text-[10px] font-mono text-muted-foreground/40">
                          {fmtTs(group.message!.timestamp_ts)}
                        </span>
                      </div>
                      <div className="rounded-xl bg-muted/20 border border-border/30 px-4 py-3">
                        <pre className="text-[13px] text-foreground whitespace-pre-wrap font-sans leading-relaxed">
                          {group.message!.content}
                        </pre>
                      </div>
                    </div>
                  </div>
                )
              }

              // ── Thinking ──────────────────────────────────────────────────
              if (node.kind === 'thinking') {
                return (
                  <div key="thinking" className="flex gap-4">
                    <div className="flex flex-col items-center w-8 shrink-0">
                      <div className="w-8 h-8 rounded-full bg-amber-500/10 border-2 border-amber-400/40 flex items-center justify-center shrink-0">
                        <Brain size={13} className="text-amber-400" />
                      </div>
                      {!isLast && <div className="w-px flex-1 bg-border/30 mt-1" />}
                    </div>
                    <div className={cn('flex-1 min-w-0 pb-6')}>
                      <div className="flex items-center gap-3 mb-2">
                        <span className="text-[11px] font-semibold uppercase tracking-wider text-amber-400">Thinking</span>
                        <span className="text-[11px] font-mono text-amber-400/60">~{thinkingSec}s</span>
                        <span className="text-[10px] text-muted-foreground/30 font-mono">
                          {group.thinking_tokens.toLocaleString()} tokens
                        </span>
                      </div>
                      {group.pre_action_text && (
                        <div className="rounded-xl bg-amber-500/[0.06] border border-amber-400/15 px-4 py-3">
                          <p className="text-[13px] text-foreground/75 italic leading-relaxed font-sans">
                            {group.pre_action_text}
                          </p>
                        </div>
                      )}
                    </div>
                  </div>
                )
              }

              // ── API call ──────────────────────────────────────────────────
              if (node.kind === 'call') {
                const { turn, idx } = node
                const cost = turn.cost_usd
                const color = MODEL_FAMILY_COLORS[modelFamily(turn.model)]

                return (
                  <div key={turn.id} className="flex gap-4">
                    <div className="flex flex-col items-center w-8 shrink-0">
                      <div
                        className="w-8 h-8 rounded-full flex items-center justify-center shrink-0 border-2 text-[10px] font-bold"
                        style={{ borderColor: color + '50', background: color + '10', color }}
                      >
                        {idx + 1}
                      </div>
                      {!isLast && <div className="w-px flex-1 bg-border/30 mt-1" />}
                    </div>
                    <div className={cn('flex-1 min-w-0 pb-5')}>
                      {/* Row header */}
                      <div className="flex items-center justify-between mb-1.5">
                        <div className="flex items-center gap-2">
                          <span className="text-muted-foreground/50" style={{ color }}>
                            <ToolIcon name={turn.tool_name} size={12} />
                          </span>
                          <span className="text-[13px] font-medium text-foreground">
                            {turn.tool_name ?? 'Inference'}
                          </span>
                          <span
                            className="text-[10px] font-semibold px-1.5 py-0.5 rounded"
                            style={{ background: color + '12', color }}
                          >
                            {turn.model}
                          </span>
                        </div>
                        <div className="flex items-center gap-3 shrink-0">
                          <span className="text-[10px] font-mono text-muted-foreground/40">
                            {fmtTs(turn.timestamp_ts)}
                          </span>
                          {isBillable(turn.model)
                            ? <span className="text-[12px] font-mono text-emerald-400 font-semibold">{fmtCost(cost)}</span>
                            : <span className="text-[11px] text-muted-foreground/30">—</span>}
                        </div>
                      </div>
                      {/* Token bar */}
                      <div className="flex items-center gap-2 mt-1">
                        <div className="flex-1 flex gap-px h-1 rounded-full overflow-hidden bg-muted/20">
                          {turn.input_tokens > 0 && (
                            <div className="h-full bg-indigo-400/60" style={{ flex: turn.input_tokens }} />
                          )}
                          {turn.cache_read_tokens > 0 && (
                            <div className="h-full bg-sky-400/50" style={{ flex: turn.cache_read_tokens }} />
                          )}
                          {turn.output_tokens > 0 && (
                            <div className="h-full bg-violet-400/60" style={{ flex: turn.output_tokens }} />
                          )}
                        </div>
                        <span className="text-[10px] font-mono text-muted-foreground/40 shrink-0 tabular-nums">
                          {fmt(turn.input_tokens)}↑ · {fmt(turn.output_tokens)}↓
                          {turn.cache_read_tokens > 0 && ` · ${fmt(turn.cache_read_tokens)} cached`}
                        </span>
                      </div>
                    </div>
                  </div>
                )
              }

              // ── Response ──────────────────────────────────────────────────
              if (node.kind === 'response') {
                return (
                  <div key="response" className="flex gap-4">
                    <div className="flex flex-col items-center w-8 shrink-0">
                      <div className="w-8 h-8 rounded-full bg-violet-500/15 border-2 border-violet-500/40 flex items-center justify-center shrink-0">
                        <Bot size={13} className="text-violet-400" />
                      </div>
                      {!isLast && <div className="w-px flex-1 bg-border/30 mt-1" />}
                    </div>
                    <div className="flex-1 min-w-0">
                      <span className="text-[11px] font-semibold uppercase tracking-wider text-violet-400 block mb-2">Response</span>
                      <div className="rounded-xl bg-violet-500/[0.05] border border-violet-500/15 px-4 py-3">
                        <pre className="text-[13px] text-foreground/80 whitespace-pre-wrap font-sans leading-relaxed">
                          {group.assistant_response}
                        </pre>
                      </div>
                    </div>
                  </div>
                )
              }

              return null
            })}
          </div>

          {/* Summary footer */}
          <div className="mt-6 pt-4 border-t border-border/20 flex items-center justify-between">
            <div className="flex items-center gap-4 text-[11px] text-muted-foreground/50">
              <span>{group.turns.length} API call{group.turns.length !== 1 ? 's' : ''}</span>
              <span>{fmt(group.total_tokens)} tokens</span>
              {thinkingSec > 0 && <span>~{thinkingSec}s thinking</span>}
            </div>
            {billable && (
              <span className="text-[13px] font-mono text-emerald-400 font-semibold">{fmtCost(totalCost)} total</span>
            )}
          </div>
        </div>
      </SheetContent>
    </Sheet>
  )
}

// ── Group row ─────────────────────────────────────────────────────────────────

function GroupRow({ group, index, onClick }: { group: RequestGroup; index: number; onClick: () => void }) {
  const firstModel = group.turns[0]?.model ?? ''
  const color = MODEL_FAMILY_COLORS[modelFamily(firstModel)]
  const totalCost = group.cost_usd
  const text = group.message?.content ?? null
  const truncated = text && text.length > 90 ? text.slice(0, 90).trimEnd() + '…' : text

  return (
    <div
      className="flex items-center gap-4 px-5 py-4 hover:bg-muted/20 transition-colors border-b border-border/30 last:border-0 cursor-pointer"
      onClick={onClick}
    >
      <span className="text-xs font-mono text-muted-foreground/40 w-6 shrink-0 text-right select-none">{index}</span>
      <div className="flex-1 min-w-0">
        {truncated
          ? <p className="text-[12px] text-foreground/80 truncate leading-snug">{truncated}</p>
          : <p className="text-[12px] text-muted-foreground/40 italic">no message</p>}
        <div className="flex items-center gap-2 mt-0.5">
          <span className="text-[11px] font-mono text-muted-foreground/50">{fmtTs(group.first_ts)}</span>
          {group.turns[0]?.cwd && (
            <span className="text-[11px] text-muted-foreground/30 truncate hidden sm:block">
              {group.turns[0].cwd.replace(/^\/Users\/[^/]+/, '~')}
            </span>
          )}
        </div>
      </div>
      <div className="flex items-center gap-2 shrink-0">
        <span className="text-[10px] font-semibold px-2 py-0.5 rounded-md hidden sm:inline"
          style={{ background: color + '15', color }}>
          {firstModel}
        </span>
        {group.turns.length > 1 && (
          <span className="text-[10px] text-muted-foreground/60 bg-muted/40 px-1.5 py-0.5 rounded-md tabular-nums">
            {group.turns.length} calls
          </span>
        )}
        {group.thinking_tokens > 0 && (
          <span className="text-[10px] text-amber-400/60 bg-amber-400/8 px-1.5 py-0.5 rounded-md">
            ~{Math.max(1, Math.round(group.thinking_tokens / 50))}s
          </span>
        )}
      </div>
      <div className="text-right hidden md:block shrink-0 w-16">
        <p className="text-[12px] font-mono text-foreground">{fmt(group.total_tokens)}</p>
        <p className="text-[10px] text-muted-foreground/40">tokens</p>
      </div>
      <div className="w-16 text-right shrink-0">
        {isBillable(firstModel)
          ? <span className="text-[12px] font-mono text-emerald-400">{fmtCost(totalCost)}</span>
          : <span className="text-[12px] text-muted-foreground/30">—</span>}
      </div>
    </div>
  )
}

// ── Main page ─────────────────────────────────────────────────────────────────

export function SessionDetailPage() {
  const { id } = useParams({ from: '/sessions/$id' })
  const [page, setPage] = useState(1)
  const [activeGroup, setActiveGroup] = useState<RequestGroup | null>(null)

  const { data, isLoading, isFetching, error } = useSessionRequests(id, page, GROUPS_PER_PAGE)

  const session = data?.session
  const groups = data?.groups ?? []
  const totalGroups = data?.total_groups ?? 0
  const totalPages = Math.max(1, Math.ceil(totalGroups / GROUPS_PER_PAGE))

  const fam = session ? modelFamily(session.model) : 'other'
  const cost = data?.total_cost_usd ?? 0

  return (
    <main className="flex-1 flex flex-col gap-6 px-6 md:px-8 py-6 w-full">
      <Link to="/" className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors w-fit">
        <ArrowLeft size={13} />
        Overview
      </Link>

      {isLoading ? (
        <div className="flex flex-col gap-3">
          <Skeleton className="h-7 w-56" />
          <Skeleton className="h-4 w-72" />
        </div>
      ) : error ? (
        <div className="rounded-xl border border-red-500/20 bg-red-500/5 p-5 text-sm text-red-400">
          {error instanceof Error ? error.message : 'Failed to load session.'}
        </div>
      ) : session ? (
        <>
          <div>
            <div className="flex items-center gap-2.5 flex-wrap">
              <h1 className="text-xl font-semibold">{session.project}</h1>
              <Badge variant="outline" className="gap-1.5 font-normal text-[11px]"
                style={{ borderColor: toolColor(session.tool) + '30', color: toolColor(session.tool) }}>
                <span className="w-1.5 h-1.5 rounded-full" style={{ background: toolColor(session.tool) }} />
                {toolLabel(session.tool)}
              </Badge>
              <Badge variant="outline" className="gap-1.5 font-normal text-[11px]"
                style={{ borderColor: MODEL_FAMILY_COLORS[fam] + '30', color: MODEL_FAMILY_COLORS[fam] }}>
                <span className="w-1.5 h-1.5 rounded-full" style={{ background: MODEL_FAMILY_COLORS[fam] }} />
                {session.model}
              </Badge>
            </div>
            <p className="text-sm text-muted-foreground mt-1">
              <span className="font-mono text-xs">{id.slice(0, 12)}…</span>
              {' · '}{fmtTs(session.last_ts)}
            </p>
          </div>

          <div className="flex flex-wrap gap-px rounded-xl border border-border/50 overflow-hidden bg-border/30">
            <div className="min-w-48 bg-card px-5 py-4">
              <p className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">Duration</p>
              <p className="mt-1.5 text-xl font-bold tabular-nums">{fmtDuration(session.duration_min)}</p>
              <p className="mt-1 text-[11px] text-muted-foreground/60 font-mono">
                {fmtTs(session.first_ts)} → {fmtTs(session.last_ts)}
              </p>
            </div>
            {[
              { label: 'Requests', value: totalGroups.toLocaleString() },
              { label: 'Turns', value: session.turns.toLocaleString() },
              { label: 'Input', value: fmt(session.input) },
              { label: 'Output', value: fmt(session.output) },
              { label: 'Cache Read', value: fmt(session.cache_read) },
              { label: 'Est. Cost', value: cost > 0 ? fmtCostBig(cost) : '—', accent: cost > 0 },
            ].map((stat) => (
              <div key={stat.label} className="flex-1 min-w-24 bg-card px-5 py-4">
                <p className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">{stat.label}</p>
                <p className={`mt-1.5 text-xl font-bold tabular-nums ${stat.accent ? 'text-emerald-400' : ''}`}>
                  {stat.value}
                </p>
              </div>
            ))}
          </div>

          <div className="rounded-xl border border-border/50 overflow-hidden bg-card">
            <div className="flex items-center justify-between px-5 py-3.5 border-b border-border/40 bg-muted/20">
              <p className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">Requests</p>
              <div className="flex items-center gap-3">
                {isFetching && !isLoading && <span className="text-[11px] text-muted-foreground/60">Loading…</span>}
                <span className="text-[11px] text-muted-foreground">
                  {totalGroups} requests · {session.turns} turns
                </span>
              </div>
            </div>

            {isLoading ? (
              <div className="divide-y divide-border/30">
                {Array.from({ length: 8 }).map((_, i) => (
                  <div key={i} className="flex items-center gap-4 px-5 py-4">
                    <Skeleton className="h-3 w-4" />
                    <div className="flex-1 flex flex-col gap-1.5">
                      <Skeleton className="h-3 w-3/4" />
                      <Skeleton className="h-2.5 w-32" />
                    </div>
                    <Skeleton className="h-5 w-24" />
                    <Skeleton className="h-3 w-16" />
                    <Skeleton className="h-3 w-12" />
                  </div>
                ))}
              </div>
            ) : groups.length === 0 ? (
              <div className="py-16 text-center text-sm text-muted-foreground">No requests found</div>
            ) : (
              <div className={cn('transition-opacity duration-150', isFetching ? 'opacity-60' : 'opacity-100')}>
                {groups.map((g, i) => (
                  <GroupRow
                    key={`${g.first_ts}-${i}`}
                    group={g}
                    index={totalGroups - (page - 1) * GROUPS_PER_PAGE - i}
                    onClick={() => setActiveGroup(g)}
                  />
                ))}
              </div>
            )}

            {totalPages > 1 && (
              <div className="flex items-center justify-between px-5 py-3.5 border-t border-border/40 bg-muted/10">
                <button
                  onClick={() => setPage((p) => Math.max(1, p - 1))}
                  disabled={page === 1 || isFetching}
                  className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-[12px] font-medium text-muted-foreground hover:text-foreground hover:bg-muted/50 disabled:opacity-30 disabled:cursor-not-allowed transition-all border border-border/40"
                >
                  <ChevronLeft size={13} /> Prev
                </button>
                <div className="flex items-center gap-1.5">
                  {Array.from({ length: Math.min(totalPages, 7) }).map((_, i) => {
                    const p = totalPages <= 7 ? i + 1
                      : page <= 4 ? i + 1
                      : page >= totalPages - 3 ? totalPages - 6 + i
                      : page - 3 + i
                    return (
                      <button key={p} onClick={() => setPage(p)} disabled={isFetching}
                        className={cn('w-7 h-7 rounded-md text-[12px] font-medium transition-all',
                          p === page
                            ? 'bg-violet-500/15 text-violet-400 border border-violet-500/30'
                            : 'text-muted-foreground hover:text-foreground hover:bg-muted/50')}>
                        {p}
                      </button>
                    )
                  })}
                </div>
                <button
                  onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                  disabled={page === totalPages || isFetching}
                  className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-[12px] font-medium text-muted-foreground hover:text-foreground hover:bg-muted/50 disabled:opacity-30 disabled:cursor-not-allowed transition-all border border-border/40"
                >
                  Next <ChevronRight size={13} />
                </button>
              </div>
            )}
          </div>
        </>
      ) : null}

      {activeGroup && (
        <RequestSheet group={activeGroup} open={true} onClose={() => setActiveGroup(null)} />
      )}
    </main>
  )
}
