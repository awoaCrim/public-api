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
import { BarChart3, Database } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'
import { formatPercent } from '@/lib/format'

import type { UsageAnalysisRow } from '../api'

type UsageAnalysisBreakdownProps = {
  rows: UsageAnalysisRow[]
  page: number
  totalPages: number
  isLoading: boolean
  isFetching: boolean
  onPrevious: () => void
  onNext: () => void
}

function formatInteger(value: number): string {
  return Math.round(value).toLocaleString()
}

const loadingRows = ['a', 'b', 'c', 'd', 'e'] as const

export function UsageAnalysisBreakdown(props: UsageAnalysisBreakdownProps) {
  const { t } = useTranslation()

  return (
    <Card aria-busy={props.isFetching}>
      <CardHeader>
        <CardTitle className='flex items-center gap-2 text-base'>
          <BarChart3 className='size-4' aria-hidden='true' />
          {t('Token Usage Breakdown')}
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className='overflow-x-auto rounded-xl border'>
          <table className='w-full min-w-[1220px] text-sm'>
            <caption className='sr-only'>{t('Token Usage Breakdown')}</caption>
            <thead className='bg-muted/50'>
              <tr className='border-b text-left'>
                <th scope='col' className='px-3 py-2.5 font-medium'>
                  {t('User')}
                </th>
                <th scope='col' className='px-3 py-2.5 font-medium'>
                  {t('API Key')}
                </th>
                <th scope='col' className='px-3 py-2.5 font-medium'>
                  {t('Model')}
                </th>
                <th scope='col' className='px-3 py-2.5 font-medium'>
                  {t('Channel')}
                </th>
                <th scope='col' className='px-3 py-2.5 text-right font-medium'>
                  {t('Requests')}
                </th>
                <th scope='col' className='px-3 py-2.5 text-right font-medium'>
                  {t('Input Tokens')}
                </th>
                <th scope='col' className='px-3 py-2.5 text-right font-medium'>
                  {t('Output Tokens')}
                </th>
                <th scope='col' className='px-3 py-2.5 text-right font-medium'>
                  {t('Total Tokens')}
                </th>
                <th scope='col' className='px-3 py-2.5 text-right font-medium'>
                  {t('Cache Read Tokens')}
                </th>
                <th scope='col' className='px-3 py-2.5 text-right font-medium'>
                  {t('Cache Write Tokens')}
                </th>
                <th scope='col' className='px-3 py-2.5 text-right font-medium'>
                  {t('Cache Rate')}
                </th>
              </tr>
            </thead>
            <tbody>
              {props.isLoading &&
                loadingRows.map((key) => (
                  <tr key={key} className='border-b last:border-0'>
                    <td colSpan={11} className='px-3 py-2.5'>
                      <Skeleton className='h-7 w-full' />
                    </td>
                  </tr>
                ))}
              {!props.isLoading &&
                props.rows.map((row) => (
                  <tr
                    key={`${row.user_id}-${row.token_id}-${row.model_name}-${row.channel_id}`}
                    className='hover:bg-muted/40 border-b transition-colors last:border-0'
                  >
                    <td className='px-3 py-2.5'>
                      {row.username || `#${row.user_id}`}
                    </td>
                    <td className='px-3 py-2.5'>
                      {row.token_name || `#${row.token_id}`}
                    </td>
                    <td className='px-3 py-2.5'>{row.model_name || '-'}</td>
                    <td className='px-3 py-2.5'>
                      {row.channel_name || `#${row.channel_id}`}
                    </td>
                    <td className='px-3 py-2.5 text-right tabular-nums'>
                      {formatInteger(row.request_count)}
                    </td>
                    <td className='px-3 py-2.5 text-right tabular-nums'>
                      {formatInteger(row.prompt_tokens)}
                    </td>
                    <td className='px-3 py-2.5 text-right tabular-nums'>
                      {formatInteger(row.completion_tokens)}
                    </td>
                    <td className='px-3 py-2.5 text-right font-medium tabular-nums'>
                      {formatInteger(row.total_tokens)}
                    </td>
                    <td className='px-3 py-2.5 text-right tabular-nums'>
                      {formatInteger(row.cache_read_tokens)}
                    </td>
                    <td className='px-3 py-2.5 text-right tabular-nums'>
                      {formatInteger(row.cache_write_tokens)}
                    </td>
                    <td className='px-3 py-2.5 text-right tabular-nums'>
                      {formatPercent(row.cache_rate)}
                    </td>
                  </tr>
                ))}
              {!props.isLoading && props.rows.length === 0 && (
                <tr>
                  <td colSpan={11}>
                    <Empty className='min-h-36 border-0'>
                      <EmptyHeader>
                        <EmptyMedia variant='icon'>
                          <Database aria-hidden='true' />
                        </EmptyMedia>
                        <EmptyTitle>{t('No usage data found.')}</EmptyTitle>
                        <EmptyDescription>
                          {t('View AI model usage and cost statistics')}
                        </EmptyDescription>
                      </EmptyHeader>
                    </Empty>
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>

        <div className='mt-3 flex flex-wrap items-center justify-between gap-3'>
          <span className='text-muted-foreground text-sm'>
            {t('Page {{current}} of {{total}}', {
              current: props.page,
              total: props.totalPages,
            })}
          </span>
          <div className='flex gap-2'>
            <Button
              variant='outline'
              disabled={props.page <= 1 || props.isFetching}
              onClick={props.onPrevious}
            >
              {t('Previous')}
            </Button>
            <Button
              variant='outline'
              disabled={props.page >= props.totalPages || props.isFetching}
              onClick={props.onNext}
            >
              {t('Next')}
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
