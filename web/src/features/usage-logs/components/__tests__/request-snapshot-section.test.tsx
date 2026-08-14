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
const { toast } = await import('sonner')

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

// The secure-verification hook probes 2FA/passkey status on mount (a real
// network call that fails inside the harness); the hook already swallows the
// failure but logs it. Silence the expected console noise.
// eslint-disable-next-line no-console
const originalConsoleError = console.error
// eslint-disable-next-line no-console
console.error = (...args: unknown[]) => {
  if (String(args[0]).includes('[Secure Verification]')) return
  originalConsoleError(...args)
}

const SNAPSHOT_PAYLOAD = {
  request_id: 'req-1',
  content_type: 'application/json',
  size: 27,
  content_base64: 'eyJtZXNzYWdlcyI6W119', // {"messages":[]}
}

function harness({
  onFetch,
  onStartVerification,
}: {
  onFetch?: (
    requestId: string,
    proofToken: string
  ) => Promise<RequestSnapshotResponse>
  onStartVerification?: (
    apiCall: (proofToken?: string) => Promise<unknown>,
    _config: unknown
  ) => Promise<boolean>
}) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  const fetchCalls: Array<{ requestId: string; proofToken: string }> = []

  const fetchSnapshot =
    onFetch ??
    (async (requestId: string, proofToken: string) => {
      fetchCalls.push({ requestId, proofToken })
      return { success: true, data: SNAPSHOT_PAYLOAD }
    })

  const startVerification =
    onStartVerification ??
    (async (
      apiCall: (proofToken?: string) => Promise<unknown>,
      _config: unknown
    ) => {
      await apiCall('fake-proof-token')
      return true
    })

  const renderAt = async (nextParentOpen: boolean) => {
    await act(async () => {
      root.render(
        <I18nextProvider i18n={i18n}>
          <RequestSnapshotSection
            requestId='req-1'
            parentOpen={nextParentOpen}
            fetchSnapshot={fetchSnapshot}
            startVerification={startVerification}
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
    // eslint-disable-next-line no-console
    console.error = originalConsoleError
    domWindow.close()
  })

  test('fetches only on click and renders the exact body text', async () => {
    const rendered = harness({})
    await rendered.renderAt(true)

    // Nothing is fetched and nothing is shown until the button is clicked.
    assert.equal(rendered.fetchCalls.length, 0)
    const initialButton = rendered.container.querySelector('button')
    assert.ok(initialButton)
    assert.match(initialButton.textContent ?? '', /View Request Body/)

    await act(async () => {
      initialButton.click()
    })

    // The injected startVerification invoked the api call with the proof.
    assert.equal(rendered.fetchCalls.length, 1)
    assert.equal(rendered.fetchCalls[0].requestId, 'req-1')
    assert.equal(rendered.fetchCalls[0].proofToken, 'fake-proof-token')

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
      rendered.container.querySelector(
        '[data-request-snapshot-content="true"]'
      ),
      null
    )

    await unmount(rendered)
  })

  test('does not fetch when verification never completes', async () => {
    const rendered = harness({
      onStartVerification: async () => false, // user cancelled / no methods
    })
    await rendered.renderAt(true)

    await act(async () => {
      ;(rendered.container.querySelector('button') as HTMLButtonElement).click()
    })

    assert.equal(rendered.fetchCalls.length, 0)
    assert.equal(
      rendered.container.querySelector(
        '[data-request-snapshot-content="true"]'
      ),
      null
    )

    await unmount(rendered)
  })

  void toast
})
