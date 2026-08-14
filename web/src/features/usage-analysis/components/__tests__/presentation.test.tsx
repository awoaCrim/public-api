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
import { after, describe, test } from 'node:test'

import { Window } from 'happy-dom'
import type React from 'react'

import type { UsageAnalysisMetrics, UsageAnalysisRow } from '../../api'

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'CustomEvent',
  'MutationObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'getComputedStyle',
] as const

for (const key of domGlobals) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { UsageAnalysisOverview } = await import('../usage-analysis-overview')
const { UsageAnalysisBreakdown } = await import('../usage-analysis-breakdown')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'Actual Consumed Tokens': 'Actual Consumed Tokens',
        'API Key': 'API Key',
        'Average Tokens per Request': 'Average Tokens per Request',
        'Cache Rate': 'Cache Rate',
        'Cache Read Tokens': 'Cache Read Tokens',
        'Cache Write Tokens': 'Cache Write Tokens',
        Channel: 'Channel',
        'Consumed Quota': 'Consumed Quota',
        'Input Tokens': 'Input Tokens',
        'Legacy rows are excluded from cache metrics.':
          'Legacy rows are excluded from cache metrics.',
        'Legacy Usage Rows': 'Legacy Usage Rows',
        Model: 'Model',
        Next: 'Next',
        'No usage data found.': 'No usage data found.',
        'Output Tokens': 'Output Tokens',
        'Page {{current}} of {{total}}': 'Page {{current}} of {{total}}',
        Previous: 'Previous',
        Requests: 'Requests',
        'Token Usage Breakdown': 'Token Usage Breakdown',
        'Total Cost': 'Total Cost',
        'Total Requests': 'Total Requests',
        'Total Tokens': 'Total Tokens',
        User: 'User',
      },
    },
  },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

const summary: UsageAnalysisMetrics = {
  request_count: 12,
  prompt_tokens: 600,
  completion_tokens: 400,
  total_tokens: 1_000,
  cache_read_tokens: 200,
  cache_write_tokens: 50,
  cache_write_tokens_5m: 30,
  cache_write_tokens_1h: 20,
  input_tokens_total: 850,
  quota: 500_000,
  cache_rate: 23.53,
  legacy_request_count: 2,
}

const row: UsageAnalysisRow = {
  ...summary,
  request_count: 3,
  user_id: 7,
  username: 'Alice',
  token_id: 11,
  token_name: 'Primary',
  model_name: 'gpt-test',
  channel_id: 19,
  channel_name: 'Channel A',
}

type Rendered = {
  container: HTMLDivElement
  root: ReturnType<typeof createRoot>
}

async function render(component: React.ReactNode): Promise<Rendered> {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)

  await act(async () => {
    root.render(<I18nextProvider i18n={i18n}>{component}</I18nextProvider>)
  })

  return { container, root }
}

async function unmount(rendered: Rendered) {
  await act(async () => rendered.root.unmount())
  rendered.container.remove()
}

describe('usage analysis presentation', () => {
  after(() => {
    domWindow.close()
  })

  test('restores the token hero while keeping cache and legacy disclosures visible', async () => {
    const rendered = await render(
      <UsageAnalysisOverview
        summary={summary}
        selectedUserName='Alice'
        selectedTokenName='Primary'
        isLoading={false}
      />
    )

    const text = rendered.container.textContent ?? ''
    assert.equal(text.includes('Actual Consumed Tokens'), true)
    assert.equal(text.includes('Alice · Primary'), true)
    assert.equal(text.includes('Total Requests'), true)
    assert.equal(text.includes('Total Cost'), true)
    assert.equal(text.includes('Cache Read Tokens'), true)
    assert.equal(text.includes('Cache Write Tokens'), true)
    const cacheRateMetric = rendered.container.querySelector(
      '[aria-label="Cache Rate"]'
    )
    assert.ok(cacheRateMetric)
    assert.equal((cacheRateMetric.textContent ?? '').includes('23.53%'), true)
    assert.equal(text.includes('Legacy Usage Rows'), true)
    assert.equal(
      text.includes('Legacy rows are excluded from cache metrics.'),
      true
    )

    await unmount(rendered)
  })

  test('keeps the paginated breakdown and exposes separate cache columns', async () => {
    let nextCalls = 0
    const rendered = await render(
      <UsageAnalysisBreakdown
        rows={[row]}
        page={1}
        totalPages={2}
        isLoading={false}
        isFetching={false}
        onPrevious={() => undefined}
        onNext={() => {
          nextCalls += 1
        }}
      />
    )

    const headings = new Set(
      [...rendered.container.querySelectorAll('th')].map(
        (heading) => heading.textContent
      )
    )
    assert.equal(headings.has('Total Tokens'), true)
    assert.equal(headings.has('Cache Read Tokens'), true)
    assert.equal(headings.has('Cache Write Tokens'), true)
    assert.equal(
      rendered.container.querySelector('caption')?.textContent,
      'Token Usage Breakdown'
    )
    assert.equal(
      (rendered.container.textContent ?? '').includes('Page 1 of 2'),
      true
    )

    const nextButton = [...rendered.container.querySelectorAll('button')].find(
      (button) => button.textContent === 'Next'
    )
    assert.ok(nextButton)
    await act(async () => nextButton.click())
    assert.equal(nextCalls, 1)

    await unmount(rendered)
  })

  test('keeps stale rows visible and disables pagination while a page is fetching', async () => {
    const rendered = await render(
      <UsageAnalysisBreakdown
        rows={[row]}
        page={1}
        totalPages={2}
        isLoading={false}
        isFetching
        onPrevious={() => undefined}
        onNext={() => undefined}
      />
    )

    assert.equal(rendered.container.textContent?.includes('Alice'), true)
    assert.equal(
      rendered.container.querySelector('[aria-busy="true"]') !== null,
      true
    )
    const buttons = [...rendered.container.querySelectorAll('button')]
    assert.equal(
      buttons.every((button) => button.disabled),
      true
    )

    await unmount(rendered)
  })
})
