import { create } from 'zustand'
import { persist } from 'zustand/middleware'

export type TimeRange = 'today' | 'yesterday' | '7d' | '30d' | '90d' | 'all'

export const RANGES: { key: TimeRange; label: string }[] = [
  { key: 'today',     label: 'Today' },
  { key: 'yesterday', label: 'Yesterday' },
  { key: '7d',        label: '7d' },
  { key: '30d',       label: '30d' },
  { key: '90d',       label: '90d' },
  { key: 'all',       label: 'All' },
]

interface FilterStore {
  range: TimeRange
  selectedModels: Set<string>
  selectedTools: Set<string>
  theme: 'dark' | 'light'
  setRange: (r: TimeRange) => void
  toggleModel: (m: string) => void
  toggleTool: (t: string) => void
  selectAll: (models: string[], tools: string[]) => void
  clearAll: () => void
  setTheme: (t: 'dark' | 'light') => void
  toggleTheme: () => void
  initFilters: (models: string[], tools: string[]) => void
}

export const useFilterStore = create<FilterStore>()(
  persist(
    (set, get) => ({
      range: '30d',
      selectedModels: new Set<string>(),
      selectedTools: new Set<string>(),
      theme: 'dark',
      setRange: (range) => set({ range }),
      toggleModel: (m) =>
        set((s) => {
          const next = new Set(s.selectedModels)
          next.has(m) ? next.delete(m) : next.add(m)
          return { selectedModels: next }
        }),
      toggleTool: (t) =>
        set((s) => {
          const next = new Set(s.selectedTools)
          next.has(t) ? next.delete(t) : next.add(t)
          return { selectedTools: next }
        }),
      selectAll: (models, tools) =>
        set({ selectedModels: new Set(models), selectedTools: new Set(tools) }),
      clearAll: () =>
        set({ selectedModels: new Set(), selectedTools: new Set() }),
      setTheme: (theme) => set({ theme }),
      toggleTheme: () => set((s) => ({ theme: s.theme === 'dark' ? 'light' : 'dark' })),
      initFilters: (models, tools) => {
        const { selectedModels, selectedTools } = get()
        if (selectedModels.size === 0) set({ selectedModels: new Set(models) })
        if (selectedTools.size === 0) set({ selectedTools: new Set(tools) })
      },
    }),
    {
      name: 'clauditor-filters',
      partialize: (s) => ({
        range: s.range,
        theme: s.theme,
        selectedModels: [...s.selectedModels],
        selectedTools: [...s.selectedTools],
      }),
      merge: (persisted, current) => {
        const p = persisted as {
          range?: TimeRange
          theme?: 'dark' | 'light'
          selectedModels?: string[]
          selectedTools?: string[]
        }
        return {
          ...current,
          range: p.range ?? current.range,
          theme: p.theme ?? current.theme,
          selectedModels: new Set<string>(p.selectedModels ?? []),
          selectedTools: new Set<string>(p.selectedTools ?? []),
        }
      },
    },
  ),
)
