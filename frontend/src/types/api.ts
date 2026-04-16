export interface DailyModelRow {
  day: string
  model: string
  tool: string
  session_id: string
  input: number
  output: number
  cache_read: number
  cache_creation: number
  turns: number
}

export interface SessionRow {
  session_id: string
  project: string
  first_ts: number
  last_ts: number
  last_date: string
  duration_min: number
  model: string
  tool: string
  turns: number
  input: number
  output: number
  cache_read: number
  cache_creation: number
  total_cost_usd: number
}

export interface ToolSummaryRow {
  tool: string
  sessions: number
  turns: number
  input: number
  output: number
  cache_read: number
  cache_creation: number
}

export interface DashboardData {
  error?: string
  all_models: string[]
  all_tools: string[]
  daily_by_model: DailyModelRow[]
  sessions_all: SessionRow[]
  tool_summary: ToolSummaryRow[]
  generated_at: string
}

export interface TurnDetailRow {
  id: number
  session_id: string
  timestamp_ts: number
  model: string
  input_tokens: number
  output_tokens: number
  cache_read_tokens: number
  cache_creation_tokens: number
  tool_name: string | null
  cwd: string | null
  tool: string
  cost_usd: number
}

export interface SessionDetail {
  session: SessionRow
  turns: TurnDetailRow[]
}

export interface UserMessage {
  timestamp_ts: number
  content: string
}

export interface RequestGroup {
  message: UserMessage | null
  pre_action_text: string
  thinking_tokens: number
  assistant_response: string
  turns: TurnDetailRow[]
  total_tokens: number
  first_ts: number
  elapsed_sec: number
  cost_usd: number
}

export interface SessionRequestsResponse {
  session: SessionRow
  groups: RequestGroup[]
  total_groups: number
  page: number
  limit: number
  total_cost_usd: number
}
