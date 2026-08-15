/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

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

import { getChannelKey } from '../api'

const originalPost = api.post

afterEach(() => {
  api.post = originalPost
})

describe('Channel key API', () => {
  test('requests the key without a secondary security-proof header', async () => {
    const calls: unknown[][] = []
    api.post = (async (...args: unknown[]) => {
      calls.push(args)
      return { data: { success: true, data: { key: 'channel-secret' } } }
    }) as typeof api.post

    await getChannelKey(42)

    assert.equal(calls.length, 1)
    assert.equal(calls[0]?.[0], '/api/channel/42/key')
    assert.equal(calls[0]?.[1], undefined)
    assert.deepEqual(calls[0]?.[2], {
      skipBusinessError: true,
      skipErrorHandler: true,
    })
  })
})
