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
import { api } from '@/lib/api'

export interface UsageAnalysisMetrics {
  request_count: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  cache_read_tokens: number
  cache_write_tokens: number
  cache_write_tokens_5m: number
  cache_write_tokens_1h: number
  input_tokens_total: number
  quota: number
  cache_rate: number
  legacy_request_count: number
}

export interface UsageAnalysisRow extends UsageAnalysisMetrics {
  user_id: number
  username: string
  token_id: number
  token_name: string
  model_name: string
  channel_id: number
  channel_name: string
}

export interface UsageAnalysisTrendPoint extends UsageAnalysisMetrics {
  timestamp: number
}

export interface UsageAnalysisData {
  start_timestamp: number
  end_timestamp: number
  bucket_seconds: number
  page: number
  page_size: number
  total: number
  summary: UsageAnalysisMetrics
  rows: UsageAnalysisRow[]
  trend: UsageAnalysisTrendPoint[]
}

export interface UsageAnalysisOptions {
  users: { id: number; name: string }[]
  tokens: { id: number; user_id: number; name: string }[]
  models: string[]
  channels: { id: number; name: string }[]
  root_user_id: number
}

interface ApiResponse<T> {
  success: boolean
  message?: string
  data?: T
}

export interface UsageAnalysisParams {
  start_timestamp: number
  end_timestamp: number
  page: number
  page_size: number
  user_id?: number
  token_id?: number
  model_name?: string
  channel_id?: number
}

export function buildUsageAnalysisQuery(params: UsageAnalysisParams): string {
  const query = new URLSearchParams({
    start_timestamp: String(params.start_timestamp),
    end_timestamp: String(params.end_timestamp),
    page: String(params.page),
    page_size: String(params.page_size),
  })
  if (params.user_id) query.set('user_id', String(params.user_id))
  if (params.token_id) query.set('token_id', String(params.token_id))
  if (params.model_name) query.set('model_name', params.model_name)
  if (params.channel_id) query.set('channel_id', String(params.channel_id))
  return query.toString()
}

export async function getUsageAnalysis(
  params: UsageAnalysisParams
): Promise<ApiResponse<UsageAnalysisData>> {
  const response = await api.get<ApiResponse<UsageAnalysisData>>(
    `/api/usage-analysis?${buildUsageAnalysisQuery(params)}`,
    { skipBusinessError: true, skipErrorHandler: true }
  )
  return response.data
}

export async function getUsageAnalysisOptions(): Promise<
  ApiResponse<UsageAnalysisOptions>
> {
  const response = await api.get<ApiResponse<UsageAnalysisOptions>>(
    '/api/usage-analysis/options',
    { skipBusinessError: true, skipErrorHandler: true }
  )
  return response.data
}
