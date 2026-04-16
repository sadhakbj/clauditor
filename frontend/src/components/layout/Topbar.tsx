import { Sun, Moon } from 'lucide-react'
import { useFilterStore } from '@/hooks/useFilterStore'
import { useDashboardData } from '@/hooks/useDashboardData'

export function Topbar() {
  const { theme, toggleTheme } = useFilterStore()
  const { data, isRefetching } = useDashboardData()

  return (
    <header className="sticky top-0 z-40 flex items-center justify-between px-6 h-12 bg-background/80 backdrop-blur border-b border-border/40">
      {/* Brand */}
      <div className="flex items-center gap-2.5">
        <div className="w-5 h-5 rounded bg-violet-500 flex items-center justify-center shrink-0">
          <span className="text-white text-[10px] font-bold leading-none">C</span>
        </div>
        <span className="text-sm font-semibold tracking-tight">clauditor</span>
      </div>

      {/* Right: live dot + theme */}
      <div className="flex items-center gap-3">
        {data?.generated_at && (
          <div className="flex items-center gap-1.5">
            <span className={`w-1.5 h-1.5 rounded-full ${isRefetching ? 'bg-amber-400' : 'bg-emerald-500'} animate-pulse`} />
            <span className="text-[11px] text-muted-foreground hidden sm:block">
              {isRefetching ? 'Refreshing…' : 'Live'}
            </span>
          </div>
        )}
        <button
          onClick={toggleTheme}
          className="p-1.5 rounded-md text-muted-foreground hover:text-foreground hover:bg-muted/50 transition-colors"
          aria-label="Toggle theme"
        >
          {theme === 'dark' ? <Sun size={14} /> : <Moon size={14} />}
        </button>
      </div>
    </header>
  )
}
