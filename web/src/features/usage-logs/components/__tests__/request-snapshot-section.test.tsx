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

import type { RequestSnapshotResponse } from '../../lib/request-snapshot'

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLButtonElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'CustomEvent',
  'MutationObserver',
  'ResizeObserver',
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
const { RequestSnapshotSection } =
  await import('../dialogs/request-snapshot-section')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'Request Body': 'Request Body',
        'View Request Body': 'View Request Body',
        'Failed to load request body': 'Failed to load request body',
        'Loading request body...': 'Loading request body...',
        Retry: 'Retry',
        'Copy to clipboard': 'Copy to clipboard',
        'Download request body': 'Download request body',
      },
    },
  },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

const SNAPSHOT_PAYLOAD = {
  request_id: 'req-1',
  content_type: 'application/json',
  size: 27,
  content_base64: 'eyJtZXNzYWdlcyI6W119', // {"messages":[]}
}

function harness({
  onFetch,
}: {
  onFetch?: (requestId: string) => Promise<RequestSnapshotResponse>
}) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  const fetchCalls: string[] = []

  const fetchSnapshot =
    onFetch ??
    (async (requestId: string) => {
      fetchCalls.push(requestId)
      return { success: true, data: SNAPSHOT_PAYLOAD }
    })

  const renderAt = async (nextParentOpen: boolean, requestId = 'req-1') => {
    await act(async () => {
      root.render(
        <I18nextProvider i18n={i18n}>
          <RequestSnapshotSection
            requestId={requestId}
            parentOpen={nextParentOpen}
            fetchSnapshot={fetchSnapshot}
          />
        </I18nextProvider>
      )
    })
  }

  return {
    container,
    root,
    renderAt,
    get fetchCalls() {
      return fetchCalls
    },
  }
}

async function unmount(rendered: {
  root: ReturnType<typeof createRoot>
  container: HTMLDivElement
}) {
  await act(async () => rendered.root.unmount())
  rendered.container.remove()
}

describe('request snapshot section', () => {
  after(() => {
    domWindow.close()
  })

  test('fetches directly on click and renders the exact body text', async () => {
    const rendered = harness({})
    await rendered.renderAt(true)

    assert.equal(rendered.fetchCalls.length, 0)
    const initialButton = rendered.container.querySelector('button')
    assert.ok(initialButton)
    assert.match(initialButton.textContent ?? '', /View Request Body/)

    await act(async () => {
      initialButton.click()
    })

    assert.deepEqual(rendered.fetchCalls, ['req-1'])
    assert.equal(
      rendered.container.querySelector('[role="dialog"]'),
      null,
      'direct root access must not open a secondary verification dialog'
    )

    const content = rendered.container.querySelector(
      '[data-request-snapshot-content="true"]'
    )
    assert.ok(content, 'captured body must be rendered')
    assert.equal(content.textContent, '{"messages":[]}')

    const actionLabels = new Set(
      [...rendered.container.querySelectorAll('button')].map((button) =>
        button.getAttribute('aria-label')
      )
    )
    assert.ok(actionLabels.has('Copy to clipboard'))
    assert.ok(actionLabels.has('Download request body'))

    await unmount(rendered)
  })

  test('clears all state when the parent dialog closes', async () => {
    const rendered = harness({})
    await rendered.renderAt(true)

    await act(async () => {
      ;(rendered.container.querySelector('button') as HTMLButtonElement).click()
    })
    assert.ok(
      rendered.container.querySelector('[data-request-snapshot-content="true"]')
    )

    await rendered.renderAt(false)

    assert.equal(
      rendered.container.querySelector(
        '[data-request-snapshot-content="true"]'
      ),
      null,
      'content must be cleared on close'
    )
    assert.ok(
      rendered.container.querySelector('button'),
      'the trigger button returns after state is cleared'
    )

    await unmount(rendered)
  })

  test('ignores an in-flight response after the parent dialog closes', async () => {
    let resolveFetch: ((value: RequestSnapshotResponse) => void) | undefined
    const pendingFetch = new Promise<RequestSnapshotResponse>((resolve) => {
      resolveFetch = resolve
    })
    const rendered = harness({ onFetch: async () => pendingFetch })
    await rendered.renderAt(true)

    await act(async () => {
      ;(rendered.container.querySelector('button') as HTMLButtonElement).click()
    })
    await rendered.renderAt(false)

    await act(async () => {
      assert.ok(resolveFetch)
      resolveFetch({ success: true, data: SNAPSHOT_PAYLOAD })
      await pendingFetch
    })
    await rendered.renderAt(true)

    assert.equal(
      rendered.container.querySelector(
        '[data-request-snapshot-content="true"]'
      ),
      null,
      'a response from the closed dialog must not restore sensitive content'
    )
    assert.match(rendered.container.textContent ?? '', /View Request Body/)

    await unmount(rendered)
  })

  test('never renders a payload returned for a different request id', async () => {
    const rendered = harness({
      onFetch: async () => ({ success: true, data: SNAPSHOT_PAYLOAD }),
    })
    await rendered.renderAt(true, 'req-2')

    await act(async () => {
      ;(rendered.container.querySelector('button') as HTMLButtonElement).click()
    })

    assert.equal(
      rendered.container.querySelector(
        '[data-request-snapshot-content="true"]'
      ),
      null,
      'a mismatched response must not expose another log row body'
    )
    assert.match(
      rendered.container.textContent ?? '',
      /Failed to load request body/
    )

    await unmount(rendered)
  })

  test('surfaces stable backend error codes', async () => {
    const rendered = harness({
      onFetch: async () => ({
        success: false,
        code: 'SNAPSHOT_NOT_FOUND',
        message: 'Snapshot not found',
      }),
    })
    await rendered.renderAt(true)

    await act(async () => {
      ;(rendered.container.querySelector('button') as HTMLButtonElement).click()
    })

    assert.match(rendered.container.textContent ?? '', /Snapshot not found/)
    assert.equal(
      rendered.container.querySelector('[role="alert"]')?.textContent,
      'Snapshot not found'
    )
    assert.equal(
      rendered.container.querySelector(
        '[data-request-snapshot-content="true"]'
      ),
      null
    )

    await unmount(rendered)
  })
})
