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

import { updateUserVisionSettings } from '../api'

const originalPut = api.put

afterEach(() => {
  api.put = originalPut
})

describe('Vision settings API', () => {
  test('uses the focused endpoint and preserves explicit false and zero values', async () => {
    const calls: unknown[][] = []
    api.put = (async (...args: unknown[]) => {
      calls.push(args)
      return { data: { success: true } }
    }) as typeof api.put

    await updateUserVisionSettings({
      vision: {
        enabled: false,
        vision_model: 'vision-model',
        vision_suffix: '-vision',
        prompt_template: 'Describe this image.',
        phash_threshold: 0,
      },
    })

    assert.equal(calls.length, 1)
    assert.equal(calls[0]?.[0], '/api/user/setting/vision')
    assert.deepEqual(calls[0]?.[1], {
      vision: {
        enabled: false,
        vision_model: 'vision-model',
        vision_suffix: '-vision',
        prompt_template: 'Describe this image.',
        phash_threshold: 0,
      },
    })
  })
})
