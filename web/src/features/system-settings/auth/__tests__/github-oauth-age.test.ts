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
  DEFAULT_GITHUB_OAUTH_MINIMUM_AGE_YEARS,
  MAX_GITHUB_OAUTH_MINIMUM_AGE_YEARS,
  normalizeGitHubOAuthMinimumAgeYears,
} from '../github-oauth-age'

describe('GitHub OAuth registration age setting', () => {
  test('defaults missing or invalid values to one calendar year', () => {
    assert.equal(DEFAULT_GITHUB_OAUTH_MINIMUM_AGE_YEARS, 1)
    assert.equal(normalizeGitHubOAuthMinimumAgeYears(undefined), 1)
    assert.equal(normalizeGitHubOAuthMinimumAgeYears('1'), 1)
    assert.equal(normalizeGitHubOAuthMinimumAgeYears(-1), 1)
    assert.equal(normalizeGitHubOAuthMinimumAgeYears(1.5), 1)
    assert.equal(normalizeGitHubOAuthMinimumAgeYears(101), 1)
  })

  test('preserves zero and supported whole-year values', () => {
    assert.equal(normalizeGitHubOAuthMinimumAgeYears(0), 0)
    assert.equal(normalizeGitHubOAuthMinimumAgeYears(3), 3)
    assert.equal(
      normalizeGitHubOAuthMinimumAgeYears(MAX_GITHUB_OAUTH_MINIMUM_AGE_YEARS),
      MAX_GITHUB_OAUTH_MINIMUM_AGE_YEARS
    )
  })
})
