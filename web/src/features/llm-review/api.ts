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

import type {
  ApiResponse,
  GetReviewTasksParams,
  GetReviewTasksResponse,
  LLMReviewConfig,
  LLMReviewConfigUpdate,
  ReviewQueueSummary,
  ReviewSchemaStatus,
  ReviewTask,
  StructuredOutputMode,
  TestConnectionResult,
} from './types'

const REVIEW_BASE = '/api/llm_review'

export async function getLLMReviewConfig(): Promise<
  ApiResponse<LLMReviewConfig>
> {
  const res = await api.get(`${REVIEW_BASE}/config`)
  return res.data
}

export async function updateLLMReviewConfig(
  body: LLMReviewConfigUpdate
): Promise<ApiResponse> {
  const res = await api.put(`${REVIEW_BASE}/config`, body)
  return res.data
}

export async function testLLMReviewSchema(
  body: {
    base_url?: string
    api_key?: string
    model?: string
    structured_output_mode?: StructuredOutputMode
    timeout_seconds?: number
    allow_private_url?: boolean
  } = {}
): Promise<
  ApiResponse<{
    ok: boolean
    schema_tested: boolean
    structured_output_tested: boolean
    structured_output_mode: StructuredOutputMode
    model: string
  }>
> {
  const res = await api.post(`${REVIEW_BASE}/test_schema`, body)
  return res.data
}

export async function testLLMReviewConnection(
  body: {
    base_url?: string
    api_key?: string
    model?: string
    structured_output_mode?: StructuredOutputMode
    timeout_seconds?: number
    allow_private_url?: boolean
  } = {}
): Promise<ApiResponse<TestConnectionResult>> {
  const res = await api.post(`${REVIEW_BASE}/test_connection`, body)
  return res.data
}

export async function getLLMReviewSchemaStatus(): Promise<
  ApiResponse<ReviewSchemaStatus>
> {
  const res = await api.get(`${REVIEW_BASE}/schema_status`)
  return res.data
}

export async function getLLMReviewQueueSummary(): Promise<
  ApiResponse<ReviewQueueSummary>
> {
  const res = await api.get(`${REVIEW_BASE}/tasks/summary`)
  return res.data
}

export async function getLLMReviewTasks(
  params: GetReviewTasksParams = {}
): Promise<GetReviewTasksResponse> {
  const searchParams = new URLSearchParams()
  if (params.p) searchParams.set('page', String(params.p))
  if (params.page_size) searchParams.set('page_size', String(params.page_size))
  if (params.status) searchParams.set('status', params.status)
  if (params.user_id) searchParams.set('user_id', String(params.user_id))
  if (params.username) searchParams.set('username', params.username)
  if (params.model_name) searchParams.set('model_name', params.model_name)
  if (params.trigger_type) searchParams.set('trigger_type', params.trigger_type)
  if (params.category) searchParams.set('category', params.category)
  if (params.keyword) searchParams.set('keyword', params.keyword)
  if (params.start_time) {
    searchParams.set('start_time', String(params.start_time))
  }
  if (params.end_time) {
    searchParams.set('end_time', String(params.end_time))
  }

  const res = await api.get(`${REVIEW_BASE}/tasks?${searchParams.toString()}`)
  const body = res.data as ApiResponse<{
    data: ReviewTask[]
    total: number
  }>
  return {
    success: body.success,
    message: body.message,
    data: body.data?.data || [],
    total: body.data?.total || 0,
  }
}

export async function getLLMReviewTaskDetail(
  id: number
): Promise<ApiResponse<ReviewTask>> {
  const res = await api.get(`${REVIEW_BASE}/tasks/${id}`)
  return res.data
}

export async function retryLLMReviewTask(
  id: number
): Promise<ApiResponse<ReviewTask>> {
  const res = await api.post(`${REVIEW_BASE}/tasks/${id}/retry`)
  return res.data
}

export async function clearLLMReviewApiKey(): Promise<ApiResponse> {
  const res = await api.post(`${REVIEW_BASE}/clear_api_key`)
  return res.data
}
