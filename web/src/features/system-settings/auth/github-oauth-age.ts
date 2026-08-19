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
export const DEFAULT_GITHUB_OAUTH_MINIMUM_AGE_YEARS = 1
export const MAX_GITHUB_OAUTH_MINIMUM_AGE_YEARS = 100

export function normalizeGitHubOAuthMinimumAgeYears(value: unknown): number {
  if (
    typeof value === 'number' &&
    Number.isInteger(value) &&
    value >= 0 &&
    value <= MAX_GITHUB_OAUTH_MINIMUM_AGE_YEARS
  ) {
    return value
  }
  return DEFAULT_GITHUB_OAUTH_MINIMUM_AGE_YEARS
}
