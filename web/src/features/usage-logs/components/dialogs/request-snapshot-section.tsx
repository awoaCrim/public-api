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
import { Check, Copy, Download, Eye, Loader2 } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { SecureVerificationDialog } from '@/features/auth/secure-verification/components/secure-verification-dialog'
import { useSecureVerification } from '@/features/auth/secure-verification/hooks/use-secure-verification'
import type { StartVerificationOptions } from '@/features/auth/secure-verification/types'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { cn } from '@/lib/utils'

import {
  REQUEST_SNAPSHOT_PROOF_SCOPE,
  decodeSnapshotContent,
  getRequestSnapshot,
  snapshotBytesToText,
  snapshotErrorKey,
  snapshotFileName,
  type RequestSnapshotPayload,
  type RequestSnapshotResponse,
} from '../../lib/request-snapshot'

export type SnapshotLoader = (
  requestId: string,
  proofToken: string
) => Promise<RequestSnapshotResponse>

type VerificationStarter = (
  apiCall: (proofToken?: string) => Promise<unknown>,
  config: StartVerificationOptions
) => Promise<boolean>

interface RequestSnapshotSectionProps {
  requestId: string
  /** Parent dialog open state; state is cleared whenever it closes. */
  parentOpen: boolean
  /** Test seam: overrides the real snapshot fetch. */
  fetchSnapshot?: SnapshotLoader
  /** Test seam: overrides the secure verification entry point. */
  startVerification?: VerificationStarter
}

/**
 * "View Request Body" control for usage log details. Content is fetched only
 * on click, through a secondary (2FA/passkey) verification with scope
 * request_snapshot.read. The decrypted body is shown in a scrollable monospace
 * section with copy and download actions. Nothing is cached globally or in
 * localStorage: all state is component-local and cleared on close.
 */
export function RequestSnapshotSection(props: RequestSnapshotSectionProps) {
  const { t } = useTranslation()
  const verification = useSecureVerification({ autoReset: true })
  const { copiedText, copyToClipboard } = useCopyToClipboard({ notify: false })

  const [payload, setPayload] = useState<RequestSnapshotPayload | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  const fetchSnapshot = props.fetchSnapshot ?? getRequestSnapshot
  const startVerification =
    props.startVerification ?? verification.startVerification

  // Clear all state whenever the parent dialog closes so no captured content
  // survives after the dialog is gone.
  useEffect(() => {
    if (!props.parentOpen) {
      setPayload(null)
      setError(null)
      setLoading(false)
    }
  }, [props.parentOpen])

  const loadSnapshot = useCallback(
    async (proofToken?: string) => {
      if (!proofToken) return
      setLoading(true)
      setError(null)
      try {
        const res = await fetchSnapshot(props.requestId, proofToken)
        if (res.success && res.data) {
          setPayload(res.data)
          return
        }
        const errorKey = snapshotErrorKey(res.code)
        setError(
          errorKey
            ? t(errorKey)
            : res.message || t('Failed to load request body')
        )
      } catch {
        setError(t('Failed to load request body'))
      } finally {
        setLoading(false)
      }
    },
    [props.requestId, fetchSnapshot, t]
  )

  const handleViewClick = useCallback(() => {
    void startVerification(loadSnapshot, {
      scope: REQUEST_SNAPSHOT_PROOF_SCOPE,
      title: t('View Request Body'),
      description: t(
        'Use Passkey or 2FA to confirm your identity before viewing the request body.'
      ),
    })
  }, [startVerification, loadSnapshot, t])

  const handleCopy = useCallback(() => {
    if (!payload) return
    const text = snapshotBytesToText(decodeSnapshotContent(payload))
    void copyToClipboard(text)
  }, [payload, copyToClipboard])

  const handleDownload = useCallback(() => {
    if (!payload) return
    const bytes = new Uint8Array(decodeSnapshotContent(payload))
    const blob = new Blob([bytes], {
      type: payload.content_type || 'application/octet-stream',
    })
    const url = URL.createObjectURL(blob)
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = snapshotFileName(payload.request_id)
    document.body.append(anchor)
    anchor.click()
    anchor.remove()
    URL.revokeObjectURL(url)
  }, [payload])

  const contentText = payload
    ? snapshotBytesToText(decodeSnapshotContent(payload))
    : ''

  return (
    <div className='min-w-0 space-y-1.5'>
      <Label className='text-xs font-semibold'>{t('Request Body')}</Label>
      {!payload && !loading && !error && (
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={() => void handleViewClick()}
          className='gap-1.5'
        >
          <Eye className='size-3.5' aria-hidden='true' />
          {t('View Request Body')}
        </Button>
      )}
      {loading && (
        <div className='bg-muted/30 text-muted-foreground flex items-center gap-2 rounded-md border p-2.5 text-xs'>
          <Loader2 className='size-3.5 animate-spin' aria-hidden='true' />
          {t('Loading request body...')}
        </div>
      )}
      {error && (
        <div className='space-y-2'>
          <p className='text-destructive text-xs'>{error}</p>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={() => void handleViewClick()}
            className='gap-1.5'
          >
            <Eye className='size-3.5' aria-hidden='true' />
            {t('Retry')}
          </Button>
        </div>
      )}
      {payload && (
        <div className='bg-muted/30 relative min-w-0 overflow-hidden rounded-md border p-2.5'>
          <div className='flex items-center justify-end gap-1 pb-1.5'>
            <Button
              variant='ghost'
              size='sm'
              className='h-6 gap-1 px-1.5 text-xs'
              onClick={handleCopy}
              title={t('Copy to clipboard')}
              aria-label={t('Copy to clipboard')}
            >
              {copiedText === contentText ? (
                <Check className='size-3 text-green-600' />
              ) : (
                <Copy className='size-3' />
              )}
            </Button>
            <Button
              variant='ghost'
              size='sm'
              className='h-6 gap-1 px-1.5 text-xs'
              onClick={handleDownload}
              title={t('Download request body')}
              aria-label={t('Download request body')}
            >
              <Download className='size-3' />
            </Button>
          </div>
          <pre
            data-request-snapshot-content='true'
            className={cn(
              'max-h-64 min-w-0 overflow-auto text-xs leading-relaxed break-all whitespace-pre-wrap',
              'font-mono'
            )}
          >
            {contentText}
          </pre>
        </div>
      )}

      <SecureVerificationDialog
        open={verification.open}
        onOpenChange={(open) => {
          if (!open) {
            verification.cancel()
          }
        }}
        methods={verification.methods}
        state={verification.state}
        onVerify={(method, code) => {
          void verification.executeVerification(method, code)
        }}
        onCancel={verification.cancel}
        onCodeChange={verification.setCode}
        onMethodChange={verification.switchMethod}
      />
    </div>
  )
}
