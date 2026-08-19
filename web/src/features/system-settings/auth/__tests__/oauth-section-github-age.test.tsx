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
import { after, afterEach, describe, test } from 'node:test'

import { Window } from 'happy-dom'

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLButtonElement',
  'HTMLDivElement',
  'HTMLFormElement',
  'HTMLInputElement',
  'HTMLLabelElement',
  'HTMLSpanElement',
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

Object.defineProperty(globalThis, 'localStorage', {
  configurable: true,
  value: domWindow.localStorage,
})

Object.defineProperty(globalThis, 'ResizeObserver', {
  configurable: true,
  value: class ResizeObserver {
    observe() {}
    unobserve() {}
    disconnect() {}
  },
})

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
Object.defineProperty(globalThis, 'matchMedia', {
  configurable: true,
  value: domWindow.matchMedia,
})

const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { RouterProvider, createMemoryHistory, createRootRoute, createRouter } =
  await import('@tanstack/react-router')
const { cleanup, fireEvent, render, waitFor, within } =
  await import('@testing-library/react')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { api } = await import('@/lib/api')
const { SettingsPageProvider } =
  await import('../../components/settings-page-context')
const { OAuthSection } = await import('../oauth-section')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'Minimum GitHub Account Age': 'Minimum GitHub Account Age',
        'GitHub account age must be a whole number':
          'GitHub account age must be a whole number',
        'GitHub account age must be between 0 and 100':
          'GitHub account age must be between 0 and 100',
        'Save Changes': 'Save Changes',
        Reset: 'Reset',
        'Unsaved changes': 'Unsaved changes',
      },
    },
  },
})

const defaultValues = {
  GitHubOAuthEnabled: false,
  GitHubOAuthMinimumAgeYears: 1,
  GitHubClientId: '',
  GitHubClientSecret: '',
  'discord.enabled': false,
  'discord.client_id': '',
  'discord.client_secret': '',
  'oidc.enabled': false,
  'oidc.display_name': '',
  'oidc.client_id': '',
  'oidc.client_secret': '',
  'oidc.well_known': '',
  'oidc.authorization_endpoint': '',
  'oidc.token_endpoint': '',
  'oidc.user_info_endpoint': '',
  TelegramOAuthEnabled: false,
  TelegramBotToken: '',
  TelegramBotName: '',
  LinuxDOOAuthEnabled: false,
  LinuxDOClientId: '',
  LinuxDOClientSecret: '',
  LinuxDOMinimumTrustLevel: '0',
  WeChatAuthEnabled: false,
  WeChatServerAddress: '',
  WeChatServerToken: '',
  WeChatAccountQRCodeImageURL: '',
}

async function renderOAuthSection() {
  const actionsContainer = document.createElement('div')
  const titleStatusContainer = document.createElement('span')
  document.body.append(actionsContainer, titleStatusContainer)
  const queryClient = new QueryClient({
    defaultOptions: { mutations: { retry: false } },
  })

  const rootRoute = createRootRoute({
    component: () => (
      <SettingsPageProvider
        actionsContainer={actionsContainer}
        titleStatusContainer={titleStatusContainer}
      >
        <OAuthSection defaultValues={defaultValues} serverAddress='' />
      </SettingsPageProvider>
    ),
  })
  const router = createRouter({
    routeTree: rootRoute,
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  await router.load()
  const rendered = render(
    <QueryClientProvider client={queryClient}>
      <I18nextProvider i18n={i18n}>
        <RouterProvider router={router} />
      </I18nextProvider>
    </QueryClientProvider>
  )

  return {
    ...rendered,
    actionsContainer,
    titleStatusContainer,
    queryClient,
    dispose() {
      rendered.unmount()
      actionsContainer.remove()
      titleStatusContainer.remove()
      queryClient.clear()
    },
  }
}

function setAge(input: HTMLInputElement, value: string) {
  fireEvent.change(input, { target: { value } })
}

describe('GitHub OAuth account age form', () => {
  afterEach(() => cleanup())
  after(() => domWindow.close())

  test('saves zero and supported whole-year values', async () => {
    const originalPut = api.put
    const updates: unknown[] = []
    api.put = (async (_url: string, data: unknown) => {
      updates.push(data)
      return { data: { success: true, message: '' } }
    }) as typeof api.put
    const rendered = await renderOAuthSection()

    try {
      const input = rendered.getByRole('spinbutton', {
        name: 'Minimum GitHub Account Age',
      }) as HTMLInputElement
      const saveButton = within(rendered.actionsContainer).getByRole('button', {
        name: 'Save Changes',
      })

      setAge(input, '0')
      fireEvent.click(saveButton)
      await waitFor(() => assert.equal(updates.length, 1))
      assert.deepEqual(updates[0], {
        key: 'GitHubOAuthMinimumAgeYears',
        value: 0,
      })

      setAge(input, '3')
      fireEvent.click(saveButton)
      await waitFor(() => assert.equal(updates.length, 2))
      assert.deepEqual(updates[1], {
        key: 'GitHubOAuthMinimumAgeYears',
        value: 3,
      })
    } finally {
      rendered.dispose()
      api.put = originalPut
    }
  })

  test('shows localized validation and sends no request for invalid values', async () => {
    const originalPut = api.put
    const updates: unknown[] = []
    api.put = (async (_url: string, data: unknown) => {
      updates.push(data)
      return { data: { success: true, message: '' } }
    }) as typeof api.put
    const rendered = await renderOAuthSection()

    try {
      const input = rendered.getByRole('spinbutton', {
        name: 'Minimum GitHub Account Age',
      }) as HTMLInputElement
      const saveButton = within(rendered.actionsContainer).getByRole('button', {
        name: 'Save Changes',
      })
      const scenarios = [
        {
          value: '-1',
          message: 'GitHub account age must be between 0 and 100',
        },
        { value: '1.5', message: 'GitHub account age must be a whole number' },
        {
          value: '101',
          message: 'GitHub account age must be between 0 and 100',
        },
      ]

      for (const scenario of scenarios) {
        setAge(input, scenario.value)
        fireEvent.click(saveButton)
        await waitFor(() => {
          assert.equal(input.getAttribute('aria-invalid'), 'true')
          assert.match(
            rendered.container.textContent ?? '',
            new RegExp(scenario.message)
          )
        })
      }
      assert.deepEqual(updates, [])
    } finally {
      rendered.dispose()
      api.put = originalPut
    }
  })

  test('keeps the edit dirty when the API rejects the save', async () => {
    const originalPut = api.put
    const updates: unknown[] = []
    api.put = (async (_url: string, data: unknown) => {
      updates.push(data)
      return { data: { success: false, message: 'save failed' } }
    }) as typeof api.put
    const rendered = await renderOAuthSection()

    try {
      const input = rendered.getByRole('spinbutton', {
        name: 'Minimum GitHub Account Age',
      }) as HTMLInputElement
      const actions = within(rendered.actionsContainer)
      const saveButton = actions.getByRole('button', { name: 'Save Changes' })

      setAge(input, '5')
      fireEvent.click(saveButton)
      await waitFor(() => assert.equal(updates.length, 1))
      await waitFor(() =>
        assert.match(
          rendered.titleStatusContainer.textContent ?? '',
          /Unsaved changes/
        )
      )
      assert.equal(input.value, '5')

      fireEvent.click(actions.getByRole('button', { name: 'Reset' }))
      await waitFor(() => assert.equal(input.value, '1'))
    } finally {
      rendered.dispose()
      api.put = originalPut
    }
  })
})
