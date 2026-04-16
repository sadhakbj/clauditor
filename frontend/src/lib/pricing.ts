interface ModelPricing {
  input: number
  output: number
  cache_write: number
  cache_read: number
}

const PRICING: Record<string, ModelPricing> = {
  // Anthropic — Claude models ($ per million tokens)
  // Source: https://www.anthropic.com/pricing (April 2026)
  'claude-opus-4-6':   { input: 15.00, output: 75.00, cache_write: 18.75, cache_read: 1.50 },
  'claude-opus-4-5':   { input: 15.00, output: 75.00, cache_write: 18.75, cache_read: 1.50 },
  'claude-sonnet-4-6': { input: 3.00,  output: 15.00, cache_write: 3.75,  cache_read: 0.30 },
  'claude-sonnet-4-5': { input: 3.00,  output: 15.00, cache_write: 3.75,  cache_read: 0.30 },
  'claude-haiku-4-5':  { input: 1.00,  output: 5.00,  cache_write: 1.25,  cache_read: 0.10 },
  'claude-haiku-4-6':  { input: 1.00,  output: 5.00,  cache_write: 1.25,  cache_read: 0.10 },
  // OpenAI — Codex / GPT models
  'gpt-5-codex':       { input: 1.25,  output: 10.00, cache_write: 0,     cache_read: 0.125 },
  'gpt-4o':            { input: 2.50,  output: 10.00, cache_write: 0,     cache_read: 1.25  },
  'gpt-4o-mini':       { input: 0.15,  output: 0.60,  cache_write: 0,     cache_read: 0.075 },
}

function getPricing(model: string): ModelPricing | null {
  if (!model) return null
  if (PRICING[model]) return PRICING[model]
  for (const key of Object.keys(PRICING)) {
    if (model.startsWith(key)) return PRICING[key]
  }
  const ml = model.toLowerCase()
  if (ml.includes('opus')) return PRICING['claude-opus-4-6']
  if (ml.includes('sonnet')) return PRICING['claude-sonnet-4-6']
  if (ml.includes('haiku')) return PRICING['claude-haiku-4-5']
  if (ml.includes('gpt-5') || ml.includes('codex')) return PRICING['gpt-5-codex']
  if (ml.includes('gpt-4o-mini')) return PRICING['gpt-4o-mini']
  if (ml.includes('gpt-4o') || ml.includes('gpt-4')) return PRICING['gpt-4o']
  return null
}

export function isBillable(model: string): boolean {
  if (!model) return false
  const ml = model.toLowerCase()
  return ml.includes('opus') || ml.includes('sonnet') || ml.includes('haiku') || ml.startsWith('gpt-')
}

export function calcCost(
  model: string,
  input: number,
  output: number,
  cacheRead: number,
  cacheCreation: number,
): number {
  if (!isBillable(model)) return 0
  const p = getPricing(model)
  if (!p) return 0
  return (
    (input * p.input) / 1e6 +
    (output * p.output) / 1e6 +
    (cacheRead * p.cache_read) / 1e6 +
    (cacheCreation * p.cache_write) / 1e6
  )
}
