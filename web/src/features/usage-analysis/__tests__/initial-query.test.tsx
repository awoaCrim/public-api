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

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLButtonElement',
  'HTMLInputElement',
  'HTMLLabelElement',
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

if (!('ResizeObserver' in globalThis)) {
  Object.defineProperty(globalThis, 'ResizeObserver', {
    configurable: true,
    value: class ResizeObserver {
      observe() {}
      unobserve() {}
      disconnect() {}
    },
  })
}

if (!('matchMedia' in domWindow)) {
  Object.defineProperty(domWindow, 'matchMedia', {
    configurable: true,
    value: () => ({
      matches: false,
      media: '',
      onchange: null,
      addListener() {},
      removeListener() {},
      addEventListener() {},
      removeEventListener() {},
      dispatchEvent() {
        return false
      },
    }),
  })
}
Object.defineProperty(globalThis, 'matchMedia', {
  configurable: true,
  value: domWindow.matchMedia,
})

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { api } = await import('@/lib/api')
const { UsageAnalysis } = await import('../index')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: {} } },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

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

const usageData = {
  start_timestamp: 1,
  end_timestamp: 2,
  bucket_seconds: 3600,
  page: 1,
  page_size: 20,
  total: 0,
  summary: emptyMetrics,
  rows: [],
  trend: [],
}

function optionsData(
  rootUserId: number,
  additionalUsers: { id: number; name: string }[] = []
) {
  return {
    users: [
      ...(rootUserId > 0 ? [{ id: rootUserId, name: 'root-admin' }] : []),
      ...additionalUsers,
    ],
    tokens: [],
    models: [],
    channels: [],
    root_user_id: rootUserId,
  }
}

async function waitForCondition(
  condition: () => boolean,
  failureMessage: string
): Promise<void> {
  if (condition()) return

  await new Promise<void>((resolve, reject) => {
    let settled = false
    const finish = (callback: () => void) => {
      if (settled) return
      settled = true
      clearInterval(intervalId)
      clearTimeout(timeoutId)
      observer.disconnect()
      callback()
    }
    const check = () => {
      if (condition()) finish(resolve)
    }
    const observer = new MutationObserver(check)
    const intervalId = setInterval(check, 10)
    const timeoutId = setTimeout(
      () => finish(() => reject(new Error(failureMessage))),
      1500
    )

    observer.observe(document, {
      attributes: true,
      childList: true,
      characterData: true,
      subtree: true,
    })
  })
}

describe('usage analysis initial query scope', () => {
  after(() => {
    domWindow.close()
  })

  test('waits for options and scopes the first analysis query to Root', async () => {
    const originalGet = api.get
    const calls: string[] = []
    api.get = (async (...args: unknown[]) => {
      const url = String(args[0])
      calls.push(url)
      if (url === '/api/usage-analysis/options') {
        return { data: { success: true, data: optionsData(42) } }
      }
      return { data: { success: true, data: usageData } }
    }) as typeof api.get

    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })

    try {
      await act(async () => {
        root.render(
          <QueryClientProvider client={queryClient}>
            <I18nextProvider i18n={i18n}>
              <UsageAnalysis />
            </I18nextProvider>
          </QueryClientProvider>
        )
      })
      await act(async () => {
        await Promise.resolve()
        await Promise.resolve()
      })
      await act(async () => {
        await waitForCondition(
          () => calls.some((url) => url.startsWith('/api/usage-analysis?')),
          `usage analysis query was not started after options loaded: ${JSON.stringify(calls)} state=${JSON.stringify(queryClient.getQueryState(['usage-analysis', 'options']))}`
        )
      })

      const analysisURL = calls.find((url) =>
        url.startsWith('/api/usage-analysis?')
      )
      assert.ok(analysisURL)
      const query = new URLSearchParams(analysisURL.split('?')[1])
      assert.equal(calls[0], '/api/usage-analysis/options')
      assert.equal(query.get('user_id'), '42')
      assert.equal(query.has('token_id'), false)
      assert.match(container.textContent ?? '', /root-admin/)
    } finally {
      await act(async () => root.unmount())
      container.remove()
      queryClient.clear()
      api.get = originalGet
    }
  })

  test('falls back visibly to all users when Root metadata is unavailable', async () => {
    const originalGet = api.get
    const calls: string[] = []
    api.get = (async (...args: unknown[]) => {
      const url = String(args[0])
      calls.push(url)
      if (url === '/api/usage-analysis/options') {
        return { data: { success: true, data: optionsData(0) } }
      }
      return { data: { success: true, data: usageData } }
    }) as typeof api.get

    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })

    try {
      await act(async () => {
        root.render(
          <QueryClientProvider client={queryClient}>
            <I18nextProvider i18n={i18n}>
              <UsageAnalysis />
            </I18nextProvider>
          </QueryClientProvider>
        )
      })
      await act(async () => {
        await Promise.resolve()
        await Promise.resolve()
      })
      await act(async () => {
        await waitForCondition(
          () => calls.some((url) => url.startsWith('/api/usage-analysis?')),
          `usage analysis query was not started after options loaded: ${JSON.stringify(calls)}`
        )
      })

      const analysisURL = calls.find((url) =>
        url.startsWith('/api/usage-analysis?')
      )
      assert.ok(analysisURL)
      const query = new URLSearchParams(analysisURL.split('?')[1])
      assert.equal(query.has('user_id'), false)
      await act(async () => {
        await waitForCondition(
          () =>
            (container.textContent ?? '').includes(
              'Root user could not be resolved'
            ),
          'Root fallback warning was not rendered'
        )
      })
      assert.match(
        container.textContent ?? '',
        /Root user could not be resolved/
      )
    } finally {
      await act(async () => root.unmount())
      container.remove()
      queryClient.clear()
      api.get = originalGet
    }
  })

  test('preserves a manually selected user after Root initialization', async () => {
    const originalGet = api.get
    const calls: string[] = []
    api.get = (async (...args: unknown[]) => {
      const url = String(args[0])
      calls.push(url)
      if (url === '/api/usage-analysis/options') {
        return {
          data: {
            success: true,
            data: optionsData(42, [{ id: 7, name: 'member' }]),
          },
        }
      }
      return { data: { success: true, data: usageData } }
    }) as typeof api.get

    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })

    try {
      await act(async () => {
        root.render(
          <QueryClientProvider client={queryClient}>
            <I18nextProvider i18n={i18n}>
              <UsageAnalysis />
            </I18nextProvider>
          </QueryClientProvider>
        )
      })
      await act(async () => {
        await waitForCondition(
          () => calls.some((url) => url.startsWith('/api/usage-analysis?')),
          `initial usage analysis query was not started: ${JSON.stringify(calls)}`
        )
      })

      const userTrigger = container.querySelector<HTMLButtonElement>(
        'button[aria-label="Select User"]'
      )
      assert.ok(userTrigger)
      await act(async () => userTrigger.click())
      const memberOption = [
        ...document.querySelectorAll<HTMLElement>('[role="option"]'),
      ].find((option) => option.textContent === 'member')
      assert.ok(memberOption)
      await act(async () => memberOption.click())

      const refreshButton = container.querySelector<HTMLButtonElement>(
        'button[aria-label="Refresh usage analysis"]'
      )
      assert.ok(refreshButton)
      await act(async () => refreshButton.click())
      await act(async () => {
        await waitForCondition(
          () =>
            calls.some((url) => {
              if (!url.startsWith('/api/usage-analysis?')) return false
              return (
                new URLSearchParams(url.split('?')[1]).get('user_id') === '7'
              )
            }),
          `manual user query was not started: ${JSON.stringify(calls)}`
        )
      })

      const analysisURL = calls.find((url) => {
        if (!url.startsWith('/api/usage-analysis?')) return false
        return new URLSearchParams(url.split('?')[1]).get('user_id') === '7'
      })
      assert.ok(analysisURL)
      const query = new URLSearchParams(analysisURL.split('?')[1])
      assert.equal(query.get('user_id'), '7')
    } finally {
      await act(async () => root.unmount())
      container.remove()
      queryClient.clear()
      api.get = originalGet
    }
  })

  test('does not issue an all-user query when options fail', async () => {
    const originalGet = api.get
    const calls: string[] = []
    api.get = (async (...args: unknown[]) => {
      const url = String(args[0])
      calls.push(url)
      return {
        data:
          url === '/api/usage-analysis/options'
            ? { success: false, message: 'options failed' }
            : { success: true, data: usageData },
      }
    }) as typeof api.get

    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })

    try {
      await act(async () => {
        root.render(
          <QueryClientProvider client={queryClient}>
            <I18nextProvider i18n={i18n}>
              <UsageAnalysis />
            </I18nextProvider>
          </QueryClientProvider>
        )
      })
      await act(async () => {
        await Promise.resolve()
        await Promise.resolve()
      })
      await act(async () => {
        await waitForCondition(
          () => calls.includes('/api/usage-analysis/options'),
          `options query was not started: ${JSON.stringify(calls)}`
        )
      })
      await act(async () => {
        await waitForCondition(
          () => (container.textContent ?? '').includes('options failed'),
          'options failure was not rendered'
        )
      })

      assert.deepEqual(calls, ['/api/usage-analysis/options'])
      assert.match(container.textContent ?? '', /options failed/)
    } finally {
      await act(async () => root.unmount())
      container.remove()
      queryClient.clear()
      api.get = originalGet
    }
  })
})
