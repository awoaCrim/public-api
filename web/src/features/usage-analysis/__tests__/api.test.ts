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
import { afterEach, describe, test } from 'node:test'

import { api } from '@/lib/api'

import {
  buildUsageAnalysisQuery,
  getUsageAnalysis,
  getUsageAnalysisOptions,
} from '../api'

const originalGet = api.get

afterEach(() => {
  api.get = originalGet
})

describe('usage analysis API query', () => {
  test('includes pagination and only selected filters', () => {
    const query = new URLSearchParams(
      buildUsageAnalysisQuery({
        start_timestamp: 100,
        end_timestamp: 200,
        page: 3,
        page_size: 20,
        user_id: 7,
        model_name: 'gpt-test',
      })
    )

    assert.equal(query.get('start_timestamp'), '100')
    assert.equal(query.get('end_timestamp'), '200')
    assert.equal(query.get('page'), '3')
    assert.equal(query.get('page_size'), '20')
    assert.equal(query.get('user_id'), '7')
    assert.equal(query.get('model_name'), 'gpt-test')
    assert.equal(query.has('token_id'), false)
    assert.equal(query.has('channel_id'), false)
  })

  test('suppresses global error toasts for inline error rendering', async () => {
    const calls: unknown[][] = []
    api.get = (async (...args: unknown[]) => {
      calls.push(args)
      return { data: { success: true } }
    }) as typeof api.get

    await getUsageAnalysis({
      start_timestamp: 100,
      end_timestamp: 200,
      page: 1,
      page_size: 20,
    })
    await getUsageAnalysisOptions()

    assert.deepEqual(calls[0]?.[1], {
      skipBusinessError: true,
      skipErrorHandler: true,
    })
    assert.deepEqual(calls[1]?.[1], {
      skipBusinessError: true,
      skipErrorHandler: true,
    })
  })
})
