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
import type { UsageAnalysisParams, UsageAnalysisTrendPoint } from '../api'

export type UsageAnalysisSelection = {
  range: { start: Date; end: Date }
  userId: string
  tokenId: string
  modelName: string
  channelId: string
}

export type UsageAnalysisTrendDatum = {
  timestamp: number
  totalTokens: number
  promptTokens: number
  completionTokens: number
  cacheReadTokens: number
  cacheWriteTokens: number
}

export function areUsageAnalysisSelectionsEqual(
  first: UsageAnalysisSelection,
  second: UsageAnalysisSelection
): boolean {
  return (
    first.range.start.getTime() === second.range.start.getTime() &&
    first.range.end.getTime() === second.range.end.getTime() &&
    first.userId === second.userId &&
    first.tokenId === second.tokenId &&
    first.modelName === second.modelName &&
    first.channelId === second.channelId
  )
}

export function resetTokenSelectionForUser(
  current: UsageAnalysisSelection,
  userId: string,
  allValue = 'all'
): UsageAnalysisSelection {
  return { ...current, userId, tokenId: allValue }
}

/**
 * Previous query data is safe to display only while moving between pages of
 * the same aggregate. Reusing it after a filter/range change would label stale
 * usage totals with the newly selected user, token, model, or channel.
 */
export function hasSameUsageAnalysisDataScope(
  current: UsageAnalysisParams,
  previous: UsageAnalysisParams | undefined
): boolean {
  if (!previous) return false
  return (
    current.start_timestamp === previous.start_timestamp &&
    current.end_timestamp === previous.end_timestamp &&
    current.page_size === previous.page_size &&
    current.user_id === previous.user_id &&
    current.token_id === previous.token_id &&
    current.model_name === previous.model_name &&
    current.channel_id === previous.channel_id
  )
}

export function getTodayUsageAnalysisRange(now = new Date()): {
  start: Date
  end: Date
} {
  const start = new Date(now)
  start.setHours(0, 0, 0, 0)
  return { start, end: now }
}

export function buildUsageAnalysisTrendData(
  points: UsageAnalysisTrendPoint[],
  start: Date,
  end: Date,
  bucketSeconds = 3600
): UsageAnalysisTrendDatum[] {
  const firstBucket =
    Math.floor(start.getTime() / 1000 / bucketSeconds) * bucketSeconds
  const lastBucket =
    Math.floor(end.getTime() / 1000 / bucketSeconds) * bucketSeconds
  const pointsByTimestamp = new Map(
    points.map((point) => [point.timestamp, point])
  )
  const result = []

  for (
    let timestamp = firstBucket;
    timestamp <= lastBucket;
    timestamp += bucketSeconds
  ) {
    const point = pointsByTimestamp.get(timestamp)
    result.push({
      timestamp,
      totalTokens: point?.total_tokens ?? 0,
      promptTokens: point?.prompt_tokens ?? 0,
      completionTokens: point?.completion_tokens ?? 0,
      cacheReadTokens: point?.cache_read_tokens ?? 0,
      cacheWriteTokens: point?.cache_write_tokens ?? 0,
    })
  }

  return result
}
