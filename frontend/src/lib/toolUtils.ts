export const TOOL_COLORS: Record<string, string> = {
  claude_code: '#a78bfa',
  codex:       '#22c55e',
}

const FALLBACK_TOOL_COLORS = [
  '#f59e0b', '#0ea5e9', '#f472b6', '#34d399', '#fb923c',
]

export function toolColor(tool: string, index = 0): string {
  return TOOL_COLORS[tool] ?? FALLBACK_TOOL_COLORS[index % FALLBACK_TOOL_COLORS.length]
}

export function toolLabel(tool: string | null | undefined): string {
  if (!tool || tool === 'claude_code') return 'Claude Code'
  if (tool === 'codex') return 'Codex'
  // Capitalize and replace underscores for unknown tools
  return tool.replace(/_/g, ' ').replace(/\b\w/g, c => c.toUpperCase())
}
