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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import type { UsageAnalysisTrendPoint } from '../../api'
import {
  areUsageAnalysisSelectionsEqual,
  buildUsageAnalysisTrendData,
  getTodayUsageAnalysisRange,
  hasSameUsageAnalysisDataScope,
  resetTokenSelectionForUser,
} from '../usage-analysis'

const emptyMetrics = {
  request_count: 0,
  prompt_tokens: 0,
  completion_tokens: 0,
  total_tokens: 0,
  cache_read_tokens: 0,
  cache_write_tokens: 0,
  cache_write_tokens_5m: 0,
  cache_write_tokens_1h: 0,
  input_tokens_total: 0,
  quota: 0,
  cache_rate: 0,
  legacy_request_count: 0,
}

describe('usage analysis helpers', () => {
  test('fills missing hourly buckets without changing returned metrics', () => {
    const points: UsageAnalysisTrendPoint[] = [
      {
        ...emptyMetrics,
        timestamp: 3_600,
        total_tokens: 120,
        prompt_tokens: 100,
        completion_tokens: 20,
        cache_read_tokens: 40,
        cache_write_tokens: 15,
      },
      {
        ...emptyMetrics,
        timestamp: 10_800,
        total_tokens: 55,
        prompt_tokens: 50,
        completion_tokens: 5,
        cache_read_tokens: 10,
      },
    ]

    const result = buildUsageAnalysisTrendData(
      points,
      new Date(3_600_000),
      new Date(10_800_000)
    )

    assert.deepEqual(
      result.map((point) => point.timestamp),
      [3_600, 7_200, 10_800]
    )
    assert.equal(result[0].cacheReadTokens, 40)
    assert.equal(result[1].totalTokens, 0)
    assert.equal(result[2].completionTokens, 5)
  })

  test('resets an incompatible token when the user changes', () => {
    const current = {
      range: { start: new Date(0), end: new Date(1) },
      userId: '1',
      tokenId: '10',
      modelName: 'all',
      channelId: 'all',
    }

    assert.deepEqual(resetTokenSelectionForUser(current, '2'), {
      ...current,
      userId: '2',
      tokenId: 'all',
    })
  })

  test('recognizes unchanged filters even when state objects were recreated', () => {
    const start = new Date(1_000)
    const end = new Date(2_000)
    const first = {
      range: { start, end },
      userId: 'all',
      tokenId: 'all',
      modelName: 'all',
      channelId: 'all',
    }
    const second = {
      ...first,
      range: { start: new Date(1_000), end: new Date(2_000) },
    }

    assert.equal(areUsageAnalysisSelectionsEqual(first, second), true)
    assert.equal(
      areUsageAnalysisSelectionsEqual(first, { ...second, modelName: 'gpt' }),
      false
    )
  })

  test('keeps previous data only for pagination within the same filter scope', () => {
    const firstPage = {
      start_timestamp: 100,
      end_timestamp: 200,
      page: 1,
      page_size: 20,
      user_id: 7,
      token_id: 11,
      model_name: 'gpt-test',
      channel_id: 19,
    }

    assert.equal(
      hasSameUsageAnalysisDataScope({ ...firstPage, page: 2 }, firstPage),
      true,
      'changing only the page may retain the previous table while fetching'
    )
    assert.equal(
      hasSameUsageAnalysisDataScope(
        { ...firstPage, page: 1, user_id: 8 },
        firstPage
      ),
      false,
      'a user filter change must not relabel stale aggregate data'
    )
    assert.equal(
      hasSameUsageAnalysisDataScope(
        { ...firstPage, page: 1, start_timestamp: 101 },
        firstPage
      ),
      false,
      'a date-range change must load a fresh aggregate'
    )
  })

  test('starts the default range at local midnight', () => {
    const now = new Date(2026, 7, 13, 12, 34, 56)
    const range = getTodayUsageAnalysisRange(now)

    assert.equal(range.start.getFullYear(), 2026)
    assert.equal(range.start.getMonth(), 7)
    assert.equal(range.start.getDate(), 13)
    assert.equal(range.start.getHours(), 0)
    assert.equal(range.start.getMinutes(), 0)
    assert.equal(range.end, now)
  })
})
