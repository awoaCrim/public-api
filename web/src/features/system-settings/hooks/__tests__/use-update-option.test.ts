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
import { after, describe, test } from 'node:test'

import { Window } from 'happy-dom'

import type { UpdateOptionRequest, UpdateOptionResponse } from '../../types'

const domWindow = new Window()
for (const key of [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLDivElement',
  'Node',
  'Element',
  'Event',
  'MutationObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'getComputedStyle',
] as const) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}

Object.defineProperty(globalThis, 'localStorage', {
  configurable: true,
  value: domWindow.localStorage,
})

const { createElement } = await import('react')
const { createRoot } = await import('react-dom/client')
const { flushSync } = await import('react-dom')
const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { api } = await import('@/lib/api')
const { useUpdateOption } = await import('../use-update-option')

type MutateAsync = ReturnType<typeof useUpdateOption>['mutateAsync']

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

type HookProbeProps = {
  onReady: (mutateAsync: MutateAsync) => void
}

function HookProbe(props: HookProbeProps) {
  const mutation = useUpdateOption()
  props.onReady(mutation.mutateAsync)
  return null
}

async function mountHook() {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  const queryClient = new QueryClient({
    defaultOptions: { mutations: { retry: false } },
  })
  let mutateAsync!: MutateAsync

  flushSync(() => {
    root.render(
      createElement(
        QueryClientProvider,
        { client: queryClient },
        createElement(HookProbe, {
          onReady: (mutation) => (mutateAsync = mutation),
        })
      )
    )
  })

  assert.ok(mutateAsync)
  return {
    mutateAsync,
    queryClient,
    cleanup: () => {
      root.unmount()
      queryClient.clear()
      container.remove()
    },
  }
}

describe('useUpdateOption group cache invalidation', () => {
  after(() => {
    domWindow.close()
  })

  test('invalidates groups after a successful GroupRatio update', async () => {
    const originalPut = api.put
    const response: UpdateOptionResponse = {
      success: true,
      message: '',
    }
    api.put = (async () => ({ data: response })) as typeof api.put
    const mounted = await mountHook()

    try {
      mounted.queryClient.setQueryData(['groups'], ['stale'])
      await mounted.mutateAsync({ key: 'GroupRatio', value: '{}' })

      const groupsQuery = mounted.queryClient
        .getQueryCache()
        .find({ queryKey: ['groups'] })
      assert.equal(groupsQuery?.state.isInvalidated, true)
    } finally {
      await mounted.cleanup()
      api.put = originalPut
    }
  })

  test('does not invalidate groups after a failed GroupRatio update', async () => {
    const originalPut = api.put
    const response: UpdateOptionResponse = {
      success: false,
      message: 'failed',
    }
    api.put = (async () => ({ data: response })) as typeof api.put
    const mounted = await mountHook()

    try {
      mounted.queryClient.setQueryData(['groups'], ['stale'])
      await mounted.mutateAsync({ key: 'GroupRatio', value: '{}' })

      const groupsQuery = mounted.queryClient
        .getQueryCache()
        .find({ queryKey: ['groups'] })
      assert.equal(groupsQuery?.state.isInvalidated, false)
    } finally {
      await mounted.cleanup()
      api.put = originalPut
    }
  })

  test('does not invalidate groups after an unrelated successful update', async () => {
    const originalPut = api.put
    const response: UpdateOptionResponse = {
      success: true,
      message: '',
    }
    api.put = (async () => ({ data: response })) as typeof api.put
    const mounted = await mountHook()

    try {
      const unrelatedRequest: UpdateOptionRequest = {
        key: 'Notice',
        value: 'updated',
      }
      mounted.queryClient.setQueryData(['groups'], ['stale'])
      await mounted.mutateAsync(unrelatedRequest)

      const groupsQuery = mounted.queryClient
        .getQueryCache()
        .find({ queryKey: ['groups'] })
      assert.equal(groupsQuery?.state.isInvalidated, false)
    } finally {
      await mounted.cleanup()
      api.put = originalPut
    }
  })
})
