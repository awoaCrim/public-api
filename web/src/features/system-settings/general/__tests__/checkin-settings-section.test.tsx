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
  'HTMLFormElement',
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

Object.defineProperty(globalThis, 'localStorage', {
  configurable: true,
  value: domWindow.localStorage,
})

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { api } = await import('@/lib/api')
const { useSystemConfigStore } = await import('@/stores/system-config-store')
const { SettingsPageProvider } =
  await import('../../components/settings-page-context')
const { CheckinSettingsSection } = await import('../checkin-settings-section')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'Check-in balance threshold': 'Check-in balance threshold',
        'Check-in balance threshold must be greater than 0':
          'Check-in balance threshold must be greater than 0',
        'Enable check-in feature': 'Enable check-in feature',
        'Enable check-in balance threshold':
          'Enable check-in balance threshold',
        'Only users with a current balance below the threshold can check in':
          'Only users with a current balance below the threshold can check in',
        'Token display mode interprets this threshold as USD':
          'Token display mode interprets this threshold as USD',
        'Minimum check-in quota': 'Minimum check-in quota',
        'Maximum check-in quota': 'Maximum check-in quota',
        'Minimum quota amount awarded for check-in':
          'Minimum quota amount awarded for check-in',
        'Maximum quota amount awarded for check-in':
          'Maximum quota amount awarded for check-in',
        'Allow users to check in daily for random quota rewards':
          'Allow users to check in daily for random quota rewards',
        'Check-in Settings': 'Check-in Settings',
        'Save check-in settings': 'Save check-in settings',
        Saving: 'Saving...',
      },
    },
  },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

function changeInputValue(input: HTMLInputElement, value: string) {
  const valueSetter = Object.getOwnPropertyDescriptor(
    domWindow.HTMLInputElement.prototype,
    'value'
  )?.set
  assert.ok(valueSetter)
  valueSetter.call(input, value)
  input.dispatchEvent(
    new domWindow.Event('input', { bubbles: true }) as unknown as Event
  )
}

describe('check-in settings section', () => {
  after(() => {
    domWindow.close()
  })

  test('uses USD as the threshold unit in token display mode and validates positive decimals', async () => {
    const previousConfig = useSystemConfigStore.getState().config
    useSystemConfigStore.setState({
      config: {
        ...previousConfig,
        currency: {
          ...previousConfig.currency,
          quotaDisplayType: 'TOKENS',
        },
      },
    })

    const container = document.createElement('div')
    const actionsContainer = document.createElement('div')
    document.body.append(container, actionsContainer)
    const root = createRoot(container)
    const queryClient = new QueryClient({
      defaultOptions: { mutations: { retry: false } },
    })

    try {
      await act(async () => {
        root.render(
          <QueryClientProvider client={queryClient}>
            <I18nextProvider i18n={i18n}>
              <SettingsPageProvider actionsContainer={actionsContainer}>
                <CheckinSettingsSection
                  defaultValues={{
                    enabled: true,
                    minQuota: 1000,
                    maxQuota: 10000,
                    balanceThresholdEnabled: true,
                    balanceThreshold: 1,
                  }}
                />
              </SettingsPageProvider>
            </I18nextProvider>
          </QueryClientProvider>
        )
      })

      const thresholdInput = container.querySelector<HTMLInputElement>(
        'input[name="balanceThreshold"]'
      )
      assert.ok(thresholdInput)
      assert.match(
        container.textContent ?? '',
        /Check-in balance threshold \(USD\)/
      )
      assert.match(
        container.textContent ?? '',
        /Token display mode interprets this threshold as USD/
      )

      await act(async () => {
        changeInputValue(thresholdInput, '0')
      })
      const saveButton = actionsContainer.querySelector('button')
      assert.ok(saveButton)
      await act(async () => {
        saveButton.click()
      })
      assert.equal(thresholdInput.getAttribute('aria-invalid'), 'true')

      await act(async () => {
        changeInputValue(thresholdInput, '-1')
      })
      await act(async () => {
        saveButton.click()
      })
      assert.equal(thresholdInput.getAttribute('aria-invalid'), 'true')

      await act(async () => {
        changeInputValue(thresholdInput, '0.5')
      })
      assert.equal(thresholdInput.getAttribute('aria-invalid'), 'false')
    } finally {
      await act(async () => root.unmount())
      container.remove()
      actionsContainer.remove()
      queryClient.clear()
      useSystemConfigStore.setState({ config: previousConfig })
    }
  })

  test('saves the threshold switch independently and preserves decimal input', async () => {
    const previousConfig = useSystemConfigStore.getState().config
    const originalPut = api.put
    const updates: Array<{ key: string; value: string }> = []

    api.put = (async (_url: string, data: unknown) => {
      updates.push(data as { key: string; value: string })
      return { data: { success: true } }
    }) as typeof api.put
    useSystemConfigStore.setState({
      config: {
        ...previousConfig,
        currency: {
          ...previousConfig.currency,
          quotaDisplayType: 'CNY',
          usdExchangeRate: 7,
        },
      },
    })

    const container = document.createElement('div')
    const actionsContainer = document.createElement('div')
    document.body.append(container, actionsContainer)
    const root = createRoot(container)
    const queryClient = new QueryClient({
      defaultOptions: { mutations: { retry: false } },
    })

    try {
      await act(async () => {
        root.render(
          <QueryClientProvider client={queryClient}>
            <I18nextProvider i18n={i18n}>
              <SettingsPageProvider actionsContainer={actionsContainer}>
                <CheckinSettingsSection
                  defaultValues={{
                    enabled: false,
                    minQuota: 1000,
                    maxQuota: 10000,
                    balanceThresholdEnabled: false,
                    balanceThreshold: 1,
                  }}
                />
              </SettingsPageProvider>
            </I18nextProvider>
          </QueryClientProvider>
        )
      })

      assert.match(
        container.textContent ?? '',
        /Enable check-in balance threshold/
      )
      assert.equal(
        container.querySelector('input[name="balanceThreshold"]'),
        null
      )
      const switches =
        container.querySelectorAll<HTMLElement>('[role="switch"]')
      assert.equal(switches.length, 2)
      await act(async () => {
        switches[1].click()
      })

      const thresholdInput = container.querySelector<HTMLInputElement>(
        'input[name="balanceThreshold"]'
      )
      assert.ok(thresholdInput)
      assert.match(
        container.textContent ?? '',
        /Check-in balance threshold \(CNY\)/
      )
      await act(async () => {
        changeInputValue(thresholdInput, '0.75')
      })

      const saveButton = actionsContainer.querySelector('button')
      assert.ok(saveButton)
      await act(async () => {
        saveButton.click()
        await Promise.resolve()
        await Promise.resolve()
      })

      assert.deepEqual(updates, [
        {
          key: 'checkin_setting.balance_threshold_enabled',
          value: 'true',
        },
        { key: 'checkin_setting.balance_threshold', value: '0.75' },
      ])
    } finally {
      await act(async () => root.unmount())
      container.remove()
      actionsContainer.remove()
      queryClient.clear()
      api.put = originalPut
      useSystemConfigStore.setState({ config: previousConfig })
    }
  })
})
