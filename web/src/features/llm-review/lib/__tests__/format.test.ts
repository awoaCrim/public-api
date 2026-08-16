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

import {
  formatFailureRate,
  formatReviewTime,
  formatWaitingSeconds,
  REVIEW_CATEGORY_LABEL_KEYS,
  REVIEW_OUTPUT_MODE_LABEL_KEYS,
  REVIEW_STATUS_LABEL_KEYS,
  REVIEW_TRIGGER_LABEL_KEYS,
} from '../format'

describe('review format helpers', () => {
  test('formats failure rates as bounded percentages', () => {
    assert.equal(formatFailureRate(0.1234), '12.3%')
    assert.equal(formatFailureRate(undefined), '-')
    assert.equal(formatFailureRate(Number.NaN), '-')
  })

  test('formats waiting seconds in human-readable units', () => {
    assert.equal(formatWaitingSeconds(45), '45s')
    assert.equal(formatWaitingSeconds(125), '2m 5s')
    assert.equal(formatWaitingSeconds(3725), '1h 2m')
    assert.equal(formatWaitingSeconds(undefined), '-')
    assert.equal(formatWaitingSeconds(-1), '-')
  })

  test('renders missing timestamps as a dash', () => {
    assert.equal(formatReviewTime(0), '-')
    assert.notEqual(formatReviewTime(1700000000), '-')
  })

  test('keeps every status, trigger and category in the label maps', () => {
    assert.equal(Object.keys(REVIEW_STATUS_LABEL_KEYS).length, 8)
    assert.equal(Object.keys(REVIEW_TRIGGER_LABEL_KEYS).length, 3)
    assert.equal(Object.keys(REVIEW_CATEGORY_LABEL_KEYS).length, 10)
    assert.equal(Object.keys(REVIEW_OUTPUT_MODE_LABEL_KEYS).length, 3)
  })
})
