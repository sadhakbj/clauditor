import { useState, useRef, useEffect } from 'react'
import { ChevronDown, Check } from 'lucide-react'
import { useFilterStore, RANGES } from '@/hooks/useFilterStore'
import { modelFamily, MODEL_FAMILY_COLORS } from '@/lib/modelUtils'
import { toolLabel, toolColor } from '@/lib/toolUtils'
import { cn } from '@/lib/utils'

interface FilterBarProps {
  allModels: string[]
  allTools: string[]
}

export function FilterBar({ allModels, allTools }: FilterBarProps) {
  const {
    range, selectedModels, selectedTools,
    setRange, toggleModel, toggleTool, selectAll, clearAll,
  } = useFilterStore()

  const [modelsOpen, setModelsOpen] = useState(false)
  const [toolsOpen, setToolsOpen] = useState(false)
  const modelsRef = useRef<HTMLDivElement>(null)
  const toolsRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function handler(e: MouseEvent) {
      if (modelsRef.current && !modelsRef.current.contains(e.target as Node)) setModelsOpen(false)
      if (toolsRef.current && !toolsRef.current.contains(e.target as Node)) setToolsOpen(false)
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [])

  const allModelsSelected = selectedModels.size === allModels.length
  const allToolsSelected = selectedTools.size === 0 || selectedTools.size === allTools.length

  const modelLabel = allModelsSelected
    ? 'All models'
    : `${selectedModels.size} model${selectedModels.size !== 1 ? 's' : ''}`

  const toolLabel_ = allToolsSelected || allTools.length <= 1
    ? 'All tools'
    : `${selectedTools.size} tool${selectedTools.size !== 1 ? 's' : ''}`

  return (
    <div className="flex items-center gap-2 flex-wrap">
      {/* Time range pills */}
      <div className="flex items-center gap-0.5 bg-muted/40 rounded-lg p-0.5 border border-border/30">
        {RANGES.map((r) => (
          <button
            key={r.key}
            onClick={() => setRange(r.key)}
            className={cn(
              'px-3 py-1 rounded-md text-[12px] font-medium transition-all whitespace-nowrap',
              range === r.key
                ? 'bg-background text-foreground shadow-sm border border-border/50'
                : 'text-muted-foreground hover:text-foreground',
            )}
          >
            {r.label}
          </button>
        ))}
      </div>

      {/* Model filter */}
      <div ref={modelsRef} className="relative">
        <button
          onClick={() => { setModelsOpen((o) => !o); setToolsOpen(false) }}
          className={cn(
            'flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-[12px] font-medium border transition-all',
            modelsOpen || !allModelsSelected
              ? 'border-violet-500/40 text-violet-400 bg-violet-500/5'
              : 'border-border/40 text-muted-foreground hover:text-foreground hover:border-border/60 bg-muted/20',
          )}
        >
          {modelLabel}
          <ChevronDown size={12} className={cn('transition-transform', modelsOpen && 'rotate-180')} />
        </button>

        {modelsOpen && (
          <div className="absolute top-full left-0 mt-1.5 z-50 w-52 rounded-xl border border-border/60 bg-card shadow-lg shadow-black/20 overflow-hidden">
            <div className="flex items-center justify-between px-3 py-2 border-b border-border/40">
              <span className="text-[11px] font-semibold text-muted-foreground uppercase tracking-wider">Models</span>
              <div className="flex gap-2">
                <button
                  onClick={() => selectAll(allModels, allTools)}
                  className="text-[11px] text-muted-foreground hover:text-foreground transition-colors"
                >
                  All
                </button>
                <button
                  onClick={clearAll}
                  className="text-[11px] text-muted-foreground hover:text-foreground transition-colors"
                >
                  None
                </button>
              </div>
            </div>
            <div className="max-h-64 overflow-y-auto py-1">
              {allModels.map((m) => {
                const fam = modelFamily(m)
                const color = MODEL_FAMILY_COLORS[fam]
                const active = selectedModels.has(m)
                return (
                  <button
                    key={m}
                    onClick={() => toggleModel(m)}
                    className="w-full flex items-center gap-2.5 px-3 py-2 hover:bg-muted/40 transition-colors text-left"
                  >
                    <span className="w-2 h-2 rounded-full shrink-0" style={{ background: color, opacity: active ? 1 : 0.25 }} />
                    <span className={cn('text-[13px] truncate flex-1', active ? 'text-foreground' : 'text-muted-foreground/50')}>
                      {m}
                    </span>
                    {active && <Check size={11} className="text-violet-400 shrink-0" />}
                  </button>
                )
              })}
            </div>
          </div>
        )}
      </div>

      {/* Tool filter — only if multiple tools */}
      {allTools.length > 1 && (
        <div ref={toolsRef} className="relative">
          <button
            onClick={() => { setToolsOpen((o) => !o); setModelsOpen(false) }}
            className={cn(
              'flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-[12px] font-medium border transition-all',
              toolsOpen || !allToolsSelected
                ? 'border-violet-500/40 text-violet-400 bg-violet-500/5'
                : 'border-border/40 text-muted-foreground hover:text-foreground hover:border-border/60 bg-muted/20',
            )}
          >
            {toolLabel_}
            <ChevronDown size={12} className={cn('transition-transform', toolsOpen && 'rotate-180')} />
          </button>

          {toolsOpen && (
            <div className="absolute top-full left-0 mt-1.5 z-50 w-44 rounded-xl border border-border/60 bg-card shadow-lg shadow-black/20 overflow-hidden">
              <div className="py-1">
                {allTools.map((t, i) => {
                  const active = selectedTools.size === 0 || selectedTools.has(t)
                  return (
                    <button
                      key={t}
                      onClick={() => toggleTool(t)}
                      className="w-full flex items-center gap-2.5 px-3 py-2 hover:bg-muted/40 transition-colors text-left"
                    >
                      <span className="w-2 h-2 rounded-full shrink-0" style={{ background: toolColor(t, i), opacity: active ? 1 : 0.25 }} />
                      <span className={cn('text-[13px] flex-1', active ? 'text-foreground' : 'text-muted-foreground/50')}>
                        {toolLabel(t)}
                      </span>
                      {active && <Check size={11} className="text-violet-400 shrink-0" />}
                    </button>
                  )
                })}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
