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
import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { cn } from '@/lib/utils'

import {
  decodeSnapshotContent,
  getRequestSnapshot,
  snapshotBytesToText,
  snapshotErrorKey,
  snapshotFileName,
  type RequestSnapshotPayload,
  type RequestSnapshotResponse,
} from '../../lib/request-snapshot'

export type SnapshotLoader = (
  requestId: string
) => Promise<RequestSnapshotResponse>

interface RequestSnapshotSectionProps {
  requestId: string
  /** Parent dialog open state; state is cleared whenever it closes. */
  parentOpen: boolean
  /** Test seam: overrides the real snapshot fetch. */
  fetchSnapshot?: SnapshotLoader
}

/**
 * "View Request Body" control for usage log details. Root-only content is
 * fetched directly on click and shown in a scrollable monospace section with
 * copy and download actions. Nothing is cached globally or in localStorage:
 * all state is component-local and cleared on close.
 */
export function RequestSnapshotSection(props: RequestSnapshotSectionProps) {
  const { t } = useTranslation()
  const { copiedText, copyToClipboard } = useCopyToClipboard({ notify: false })

  const [payload, setPayload] = useState<RequestSnapshotPayload | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const requestGenerationRef = useRef(0)
  const parentOpenRef = useRef(props.parentOpen)

  const fetchSnapshot = props.fetchSnapshot ?? getRequestSnapshot

  // Invalidate in-flight reads and clear all local content whenever the parent
  // closes or switches to another log row. A late response must never restore
  // sensitive content after the dialog has been closed.
  useEffect(() => {
    parentOpenRef.current = props.parentOpen
    requestGenerationRef.current += 1
    setPayload(null)
    setError(null)
    setLoading(false)
  }, [props.parentOpen, props.requestId])

  useEffect(
    () => () => {
      requestGenerationRef.current += 1
    },
    []
  )

  const loadSnapshot = useCallback(async () => {
    const requestGeneration = requestGenerationRef.current + 1
    requestGenerationRef.current = requestGeneration
    setLoading(true)
    setError(null)
    try {
      const res = await fetchSnapshot(props.requestId)
      if (
        requestGeneration !== requestGenerationRef.current ||
        !parentOpenRef.current
      ) {
        return
      }
      if (res.success && res.data) {
        if (res.data.request_id !== props.requestId) {
          setError(t('Failed to load request body'))
          return
        }
        setPayload(res.data)
        return
      }
      const errorKey = snapshotErrorKey(res.code)
      setError(
        errorKey ? t(errorKey) : res.message || t('Failed to load request body')
      )
    } catch {
      if (
        requestGeneration === requestGenerationRef.current &&
        parentOpenRef.current
      ) {
        setError(t('Failed to load request body'))
      }
    } finally {
      if (
        requestGeneration === requestGenerationRef.current &&
        parentOpenRef.current
      ) {
        setLoading(false)
      }
    }
  }, [props.requestId, fetchSnapshot, t])

  // Never render bytes loaded for a previous row, even during the render before
  // the state-clearing effect runs after a request-id change.
  const visiblePayload =
    payload?.request_id === props.requestId ? payload : null

  const handleCopy = useCallback(() => {
    if (!visiblePayload) return
    const text = snapshotBytesToText(decodeSnapshotContent(visiblePayload))
    void copyToClipboard(text)
  }, [visiblePayload, copyToClipboard])

  const handleDownload = useCallback(() => {
    if (!visiblePayload) return
    const bytes = new Uint8Array(decodeSnapshotContent(visiblePayload))
    const blob = new Blob([bytes], {
      type: visiblePayload.content_type || 'application/octet-stream',
    })
    const url = URL.createObjectURL(blob)
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = snapshotFileName(visiblePayload.request_id)
    document.body.append(anchor)
    anchor.click()
    anchor.remove()
    URL.revokeObjectURL(url)
  }, [visiblePayload])

  const contentText = visiblePayload
    ? snapshotBytesToText(decodeSnapshotContent(visiblePayload))
    : ''

  return (
    <div className='flex min-w-0 flex-col gap-1.5'>
      <Label className='text-xs font-semibold'>{t('Request Body')}</Label>
      {!visiblePayload && !loading && !error && (
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={() => void loadSnapshot()}
          className='gap-1.5'
        >
          <Eye data-icon='inline-start' aria-hidden='true' />
          {t('View Request Body')}
        </Button>
      )}
      {loading && (
        <div
          className='bg-muted/30 text-muted-foreground flex items-center gap-2 rounded-md border p-2.5 text-xs'
          role='status'
          aria-live='polite'
        >
          <Loader2 className='size-3.5 animate-spin' aria-hidden='true' />
          {t('Loading request body...')}
        </div>
      )}
      {error && (
        <div className='flex flex-col gap-2'>
          <p className='text-destructive text-xs' role='alert'>
            {error}
          </p>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={() => void loadSnapshot()}
            className='gap-1.5'
          >
            <Eye data-icon='inline-start' aria-hidden='true' />
            {t('Retry')}
          </Button>
        </div>
      )}
      {visiblePayload && (
        <div className='bg-muted/30 relative min-w-0 overflow-hidden rounded-md border p-2.5'>
          <div className='flex items-center justify-end gap-1 pb-1.5'>
            <Button
              variant='ghost'
              size='icon-xs'
              onClick={handleCopy}
              title={t('Copy to clipboard')}
              aria-label={t('Copy to clipboard')}
            >
              {copiedText === contentText ? (
                <Check className='text-green-600' aria-hidden='true' />
              ) : (
                <Copy aria-hidden='true' />
              )}
            </Button>
            <Button
              variant='ghost'
              size='icon-xs'
              onClick={handleDownload}
              title={t('Download request body')}
              aria-label={t('Download request body')}
            >
              <Download aria-hidden='true' />
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
    </div>
  )
}
