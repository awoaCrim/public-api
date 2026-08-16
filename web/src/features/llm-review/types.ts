/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
export type ReviewTaskStatus =
  | 'pending'
  | 'reviewing'
  | 'compliant'
  | 'violation'
  | 'uncertain'
  | 'skipped'
  | 'failed'
  | 'superseded'

export type ReviewTriggerType = 'rpm' | 'input_token' | 'output_token'

export type ReviewTriggerStage = 'preflight' | 'postflight'

export type ReviewVerdict = 'violation' | 'compliant' | 'uncertain'

export type StructuredOutputMode =
  | 'strict_schema'
  | 'json_object'
  | 'prompt_json'

export type ReviewCategory =
  | 'commercial_use'
  | 'account_sharing'
  | 'unauthorized_client'
  | 'stress_test'
  | 'abnormal_automation'
  | 'limit_bypass'
  | 'harmful_resource_use'
  | 'code_generation'
  | 'other'
  | 'none'

export interface ApiResponse<T = unknown> {
  success: boolean
  message?: string
  data?: T
}

export interface ReviewAttempt {
  id: number
  task_id: number
  attempt_no: number
  requested_at: number
  duration_ms: number
  http_status: number
  response: string
  error: string
  retryable: boolean
  created_at: number
}

export interface ReviewTask {
  id: number
  review_no: string
  user_id: number
  username: string
  display_name: string
  status: ReviewTaskStatus
  trigger_type: ReviewTriggerType
  trigger_types: string
  trigger_stage: ReviewTriggerStage
  model_name: string
  channel_id: number
  channel_name: string
  channel_assignment: 'assigned' | 'unassigned_preflight' | 'unassigned'
  recent_channel_id: number
  recent_channel_name: string
  recent_channel_at: number
  api_endpoint: string
  current_value: number
  limit_value: number
  estimated_input_tokens: number
  actual_input_tokens: number
  actual_output_tokens: number
  verdict: ReviewVerdict | ''
  category: ReviewCategory | ''
  confidence: number
  short_reason: string
  banned: boolean
  ban_error: string
  merged_event_count: number
  ip_masked: string
  next_retry_at: number
  created_at: number
  started_at: number
  finished_at: number
  request_summary?: string
  review_payload?: string
  evidence?: string[]
  raw_response?: string
  schema_valid?: boolean
  schema_error?: string
  review_model?: string
  policy_id?: string
  prompt_template_version?: string
  schema_version?: string
  output_mode?: StructuredOutputMode
  human_override?: string
  attempts?: ReviewAttempt[]
}

export interface ReviewQueueSummary {
  pending: number
  reviewing: number
  oldest_waiting_seconds: number
  recent_failure_rate: number
  merged_events: number
}

export interface ReviewSchemaStatus {
  status: 'untested' | 'passed' | 'failed'
  tested: boolean
  supports_strict_json_schema: boolean
  structured_output_mode: StructuredOutputMode
  structured_output_tested: boolean
  structured_output_tested_at: number
  structured_output_tested_model: string
  structured_output_version: string
  policy_configured: boolean
  ready: boolean
  readiness_reason: string
  tested_at: number
  tested_model: string
  schema_version: string
  error: string
}

export interface TestConnectionResult {
  ok: boolean
  latency_ms: number
  model: string
  schema_tested: boolean
}

export interface LLMReviewConfig {
  enabled: boolean
  base_url: string
  api_key: string
  model: string
  policy_text: string
  timeout_seconds: number
  max_attempts: number
  retry_interval_seconds: number
  worker_concurrency: number
  confidence_threshold: number
  compliant_limit: number
  immune_hours: number
  retention_days: number
  max_output_tokens: number
  allow_private_url: boolean
  schema_tested: boolean
  structured_output_mode: StructuredOutputMode
  structured_output_tested: boolean
  structured_output_tested_at: number
  structured_output_tested_model: string
  structured_output_version: string
  policy_configured: boolean
  readiness_ready: boolean
  readiness_reason: string
}

export interface LLMReviewConfigUpdate {
  enabled: boolean
  base_url: string
  api_key?: string
  model: string
  policy_text?: string
  timeout_seconds: number
  max_attempts: number
  retry_interval_seconds: number
  worker_concurrency: number
  confidence_threshold: number
  compliant_limit: number
  immune_hours: number
  retention_days: number
  max_output_tokens: number
  allow_private_url: boolean
  structured_output_mode?: StructuredOutputMode
}

export interface GetReviewTasksParams {
  p?: number
  page_size?: number
  status?: string
  user_id?: number
  username?: string
  model_name?: string
  trigger_type?: string
  category?: string
  keyword?: string
  start_time?: number
  end_time?: number
}

export interface GetReviewTasksResponse {
  success: boolean
  message?: string
  data: ReviewTask[]
  total: number
}
