import { useState } from 'react'
import { ChevronDown, Check } from 'lucide-react'
import { useFilterStore, RANGES } from '@/hooks/useFilterStore'
import { modelFamily, MODEL_FAMILY_COLORS } from '@/lib/modelUtils'
import { toolLabel, toolColor } from '@/lib/toolUtils'
import { cn } from '@/lib/utils'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command'

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
      <Popover open={modelsOpen} onOpenChange={setModelsOpen}>
        <PopoverTrigger asChild>
          <button
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
        </PopoverTrigger>
        <PopoverContent className="w-52 p-0" align="start">
          <Command>
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
            <CommandInput placeholder="Search models…" className="h-8 text-[12px]" />
            <CommandList className="max-h-64">
              <CommandEmpty className="py-3 text-center text-[12px] text-muted-foreground">No models found.</CommandEmpty>
              <CommandGroup>
                {allModels.map((m) => {
                  const fam = modelFamily(m)
                  const color = MODEL_FAMILY_COLORS[fam]
                  const active = selectedModels.has(m)
                  return (
                    <CommandItem
                      key={m}
                      value={m}
                      onSelect={() => toggleModel(m)}
                      className="flex items-center gap-2.5 px-3 py-2 cursor-pointer"
                    >
                      <span className="w-2 h-2 rounded-full shrink-0" style={{ background: color, opacity: active ? 1 : 0.25 }} />
                      <span className={cn('text-[13px] truncate flex-1', active ? 'text-foreground' : 'text-muted-foreground/50')}>
                        {m}
                      </span>
                      {active && <Check size={11} className="text-violet-400 shrink-0" />}
                    </CommandItem>
                  )
                })}
              </CommandGroup>
            </CommandList>
          </Command>
        </PopoverContent>
      </Popover>

      {/* Tool filter — only if multiple tools */}
      {allTools.length > 1 && (
        <Popover open={toolsOpen} onOpenChange={setToolsOpen}>
          <PopoverTrigger asChild>
            <button
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
          </PopoverTrigger>
          <PopoverContent className="w-44 p-0" align="start">
            <Command>
              <CommandList>
                <CommandGroup>
                  {allTools.map((t, i) => {
                    const active = selectedTools.size === 0 || selectedTools.has(t)
                    return (
                      <CommandItem
                        key={t}
                        value={t}
                        onSelect={() => toggleTool(t)}
                        className="flex items-center gap-2.5 px-3 py-2 cursor-pointer"
                      >
                        <span className="w-2 h-2 rounded-full shrink-0" style={{ background: toolColor(t, i), opacity: active ? 1 : 0.25 }} />
                        <span className={cn('text-[13px] flex-1', active ? 'text-foreground' : 'text-muted-foreground/50')}>
                          {toolLabel(t)}
                        </span>
                        {active && <Check size={11} className="text-violet-400 shrink-0" />}
                      </CommandItem>
                    )
                  })}
                </CommandGroup>
              </CommandList>
            </Command>
          </PopoverContent>
        </Popover>
      )}
    </div>
  )
}
