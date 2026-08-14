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
import type { ColumnDef } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

import {
  formatReviewTime,
  getReviewCategoryLabel,
  getReviewStatusLabel,
  getReviewTriggerLabel,
} from '../lib/format'
import type { ReviewTask, ReviewTaskStatus } from '../types'

const statusVariantMap: Record<
  ReviewTaskStatus,
  'default' | 'secondary' | 'destructive' | 'outline'
> = {
  pending: 'secondary',
  reviewing: 'default',
  compliant: 'default',
  violation: 'destructive',
  uncertain: 'secondary',
  skipped: 'outline',
  failed: 'destructive',
  superseded: 'outline',
}

function formatNumber(value: number | undefined | null): string {
  if (value == null) return '-'
  return value.toLocaleString()
}

export function useLLMReviewLogsColumns(
  onViewDetail: (task: ReviewTask) => void
): ColumnDef<ReviewTask>[] {
  const { t } = useTranslation()

  return [
    {
      accessorKey: 'review_no',
      header: t('Review No'),
      size: 140,
      cell: ({ row }) => (
        <button
          type='button'
          className='text-primary font-mono text-xs hover:underline'
          onClick={() => onViewDetail(row.original)}
        >
          {row.original.review_no || `#${row.original.id}`}
        </button>
      ),
    },
    {
      accessorKey: 'status',
      header: t('Status'),
      size: 100,
      cell: ({ row }) => {
        const status = row.original.status
        return (
          <Badge variant={statusVariantMap[status] ?? 'default'}>
            {getReviewStatusLabel(status, t)}
          </Badge>
        )
      },
    },
    {
      accessorKey: 'trigger_type',
      header: t('Trigger Type'),
      size: 110,
      cell: ({ row }) => (
        <Badge variant='secondary'>
          {getReviewTriggerLabel(row.original.trigger_type, t)}
        </Badge>
      ),
    },
    {
      accessorKey: 'username',
      header: t('User'),
      size: 120,
      cell: ({ row }) => (
        <span className='text-sm'>
          {row.original.display_name || row.original.username || '-'}
        </span>
      ),
    },
    {
      accessorKey: 'model_name',
      header: t('Model'),
      size: 140,
      cell: ({ row }) => (
        <span className='text-xs'>{row.original.model_name || '-'}</span>
      ),
    },
    {
      accessorKey: 'channel_name',
      header: t('Channel'),
      size: 110,
      cell: ({ row }) => {
        if (row.original.channel_name) {
          return <span className='text-xs'>{row.original.channel_name}</span>
        }
        if (row.original.channel_assignment === 'unassigned_preflight') {
          return (
            <Tooltip>
              <TooltipTrigger
                render={<span className='text-muted-foreground text-xs' />}
              >
                {t('Unassigned (RPM preflight block)')}
              </TooltipTrigger>
              {row.original.recent_channel_name && (
                <TooltipContent>
                  {t('Recently used channel')}:{' '}
                  {row.original.recent_channel_name}
                </TooltipContent>
              )}
            </Tooltip>
          )
        }
        return (
          <span className='text-muted-foreground text-xs'>
            {t('Unassigned')}
          </span>
        )
      },
    },
    {
      accessorKey: 'api_endpoint',
      header: t('API Endpoint'),
      size: 160,
      cell: ({ row }) => {
        const value = row.original.api_endpoint
        if (!value) {
          return <span className='text-muted-foreground text-xs'>-</span>
        }
        return (
          <Tooltip>
            <TooltipTrigger
              render={
                <span className='text-muted-foreground block max-w-[160px] truncate font-mono text-[11px]' />
              }
            >
              {value}
            </TooltipTrigger>
            <TooltipContent className='max-w-[320px] font-mono text-[11px] break-all'>
              {value}
            </TooltipContent>
          </Tooltip>
        )
      },
    },
    {
      accessorKey: 'current_value',
      header: t('Current / Limit'),
      size: 110,
      cell: ({ row }) => {
        const { trigger_type, current_value, limit_value } = row.original
        const unit = trigger_type === 'rpm' ? ' req/min' : ' tokens'
        return (
          <span className='font-mono text-xs whitespace-nowrap tabular-nums'>
            {formatNumber(current_value)}
            {unit} / {formatNumber(limit_value)}
          </span>
        )
      },
    },
    {
      accessorKey: 'verdict',
      header: t('LLM Verdict'),
      size: 100,
      cell: ({ row }) => {
        const verdict = row.original.verdict
        if (!verdict) {
          return <span className='text-muted-foreground text-xs'>-</span>
        }
        const variantMap: Record<
          string,
          'default' | 'secondary' | 'destructive' | 'outline'
        > = {
          violation: 'destructive',
          compliant: 'default',
          uncertain: 'outline',
        }
        return (
          <Badge variant={variantMap[verdict] ?? 'outline'}>
            {t(verdict.charAt(0).toUpperCase() + verdict.slice(1))}
          </Badge>
        )
      },
    },
    {
      accessorKey: 'category',
      header: t('Category'),
      size: 120,
      cell: ({ row }) => {
        const category = row.original.category
        if (!category) {
          return <span className='text-muted-foreground text-xs'>-</span>
        }
        return (
          <span className='text-xs'>{getReviewCategoryLabel(category, t)}</span>
        )
      },
    },
    {
      accessorKey: 'confidence',
      header: t('Confidence'),
      size: 90,
      cell: ({ row }) => {
        const value = row.original.confidence
        if (value == null) {
          return <span className='text-muted-foreground text-xs'>-</span>
        }
        return (
          <span className='font-mono text-xs tabular-nums'>
            {(value * 100).toFixed(0)}%
          </span>
        )
      },
    },
    {
      accessorKey: 'short_reason',
      header: t('Reason'),
      size: 200,
      cell: ({ row }) => {
        const value = row.original.short_reason
        if (!value) {
          return <span className='text-muted-foreground text-xs'>-</span>
        }
        return (
          <span className='block max-w-[200px] truncate text-xs'>{value}</span>
        )
      },
    },
    {
      accessorKey: 'ip_masked',
      header: 'IP',
      size: 110,
      cell: ({ row }) => (
        <span className='font-mono text-xs'>
          {row.original.ip_masked || '-'}
        </span>
      ),
    },
    {
      accessorKey: 'created_at',
      header: t('Created At'),
      size: 150,
      cell: ({ row }) => (
        <span className='text-xs'>
          {formatReviewTime(row.original.created_at)}
        </span>
      ),
    },
    {
      accessorKey: 'finished_at',
      header: t('Finished At'),
      size: 150,
      cell: ({ row }) => (
        <span className='text-xs'>
          {formatReviewTime(row.original.finished_at)}
        </span>
      ),
    },
  ]
}
