export type ModelFamily = 'opus' | 'sonnet' | 'haiku' | 'other'

export function modelFamily(model: string): ModelFamily {
  if (!model) return 'other'
  const ml = model.toLowerCase()
  if (ml.includes('opus')) return 'opus'
  if (ml.includes('sonnet')) return 'sonnet'
  if (ml.includes('haiku')) return 'haiku'
  return 'other'
}

export function modelPriority(model: string): number {
  return { opus: 0, sonnet: 1, haiku: 2, other: 3 }[modelFamily(model)]
}

export const MODEL_FAMILY_COLORS: Record<ModelFamily, string> = {
  opus:   '#a78bfa',
  sonnet: '#0ea5e9',
  haiku:  '#10b981',
  other:  '#f59e0b',
}

// Palette for chart series (models in order)
export const CHART_COLORS = [
  '#0ea5e9',
  '#a78bfa',
  '#10b981',
  '#f59e0b',
  '#f472b6',
  '#60a5fa',
  '#34d399',
  '#fb923c',
]
