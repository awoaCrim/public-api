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
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { DataTablePage, useDataTable } from '@/components/data-table'
import { useTableUrlState } from '@/hooks/use-table-url-state'

import { getLLMReviewTasks } from '../api'
import type { ReviewTask } from '../types'
import { useLLMReviewLogsColumns } from './llm-review-logs-columns'
import { ReviewDetailDrawer } from './review-detail-drawer'

const route = getRouteApi('/_authenticated/llm-review-logs/')

export function LLMReviewLogsTable() {
  const { t } = useTranslation()
  const [selectedTask, setSelectedTask] = useState<ReviewTask | null>(null)
  const [detailOpen, setDetailOpen] = useState(false)

  const {
    globalFilter,
    onGlobalFilterChange,
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    ensurePageInRange,
  } = useTableUrlState({
    search: route.useSearch(),
    navigate: route.useNavigate(),
    pagination: { defaultPage: 1, defaultPageSize: 20 },
    globalFilter: { enabled: true, key: 'keyword' },
    columnFilters: [
      { columnId: 'status', searchKey: 'status', type: 'string' as const },
      {
        columnId: 'trigger_type',
        searchKey: 'trigger_type',
        type: 'string' as const,
      },
      { columnId: 'category', searchKey: 'category', type: 'string' as const },
      { columnId: 'username', searchKey: 'username', type: 'string' as const },
    ],
  })

  const statusFilter =
    (columnFilters.find((f) => f.id === 'status')?.value as string) ?? ''
  const triggerFilter =
    (columnFilters.find((f) => f.id === 'trigger_type')?.value as string) ?? ''
  const categoryFilter =
    (columnFilters.find((f) => f.id === 'category')?.value as string) ?? ''
  const usernameFilter =
    (columnFilters.find((f) => f.id === 'username')?.value as string) ?? ''

  const { data, isLoading, isFetching } = useQuery({
    queryKey: [
      'llm-review-logs',
      pagination.pageIndex + 1,
      pagination.pageSize,
      globalFilter,
      statusFilter,
      triggerFilter,
      categoryFilter,
      usernameFilter,
    ],
    queryFn: async () => {
      const result = await getLLMReviewTasks({
        p: pagination.pageIndex + 1,
        page_size: pagination.pageSize,
        keyword: globalFilter || undefined,
        status: statusFilter || undefined,
        trigger_type: triggerFilter || undefined,
        category: categoryFilter || undefined,
        username: usernameFilter || undefined,
      })

      if (!result.success) {
        toast.error(result.message || t('Failed to load review logs'))
        return { items: [], total: 0 }
      }

      return { items: result.data || [], total: result.total || 0 }
    },
    placeholderData: keepPreviousData,
  })

  const logs = data?.items || []
  const columns = useLLMReviewLogsColumns((task) => {
    setSelectedTask(task)
    setDetailOpen(true)
  })

  const { table } = useDataTable({
    data: logs,
    columns,
    columnFilters,
    globalFilter,
    pagination,
    onPaginationChange,
    onGlobalFilterChange,
    onColumnFiltersChange,
    manualPagination: true,
    manualFiltering: true,
    totalCount: data?.total || 0,
    ensurePageInRange,
  })

  return (
    <>
      <DataTablePage
        table={table}
        columns={columns}
        isLoading={isLoading}
        isFetching={isFetching}
        emptyTitle={t('No Review Logs Found')}
        emptyDescription={t('No LLM compliance review records yet.')}
        skeletonKeyPrefix='llm-review-skeleton'
        applyHeaderSize
        toolbarProps={{
          searchPlaceholder: t('Search by username or model...'),
          filters: [
            {
              columnId: 'status',
              title: t('Status'),
              singleSelect: true,
              options: [
                { label: t('Pending'), value: 'pending' },
                { label: t('Reviewing'), value: 'reviewing' },
                { label: t('Compliant'), value: 'compliant' },
                { label: t('Violation'), value: 'violation' },
                { label: t('Uncertain'), value: 'uncertain' },
                { label: t('Skipped'), value: 'skipped' },
                { label: t('Failed'), value: 'failed' },
                { label: t('Superseded'), value: 'superseded' },
              ],
            },
            {
              columnId: 'trigger_type',
              title: t('Trigger Type'),
              singleSelect: true,
              options: [
                { label: 'RPM', value: 'rpm' },
                { label: t('Input Tokens'), value: 'input_token' },
                { label: t('Output Tokens'), value: 'output_token' },
              ],
            },
            {
              columnId: 'category',
              title: t('Category'),
              singleSelect: true,
              options: [
                { label: t('Commercial Use'), value: 'commercial_use' },
                { label: t('Account Sharing'), value: 'account_sharing' },
                {
                  label: t('Unauthorized Client'),
                  value: 'unauthorized_client',
                },
                { label: t('Stress Test'), value: 'stress_test' },
                {
                  label: t('Abnormal Automation'),
                  value: 'abnormal_automation',
                },
                { label: t('Limit Bypass'), value: 'limit_bypass' },
                {
                  label: t('Harmful Resource Use'),
                  value: 'harmful_resource_use',
                },
                { label: t('Code Generation'), value: 'code_generation' },
                { label: t('Other'), value: 'other' },
              ],
            },
          ],
        }}
      />

      <ReviewDetailDrawer
        task={selectedTask}
        open={detailOpen}
        onOpenChange={setDetailOpen}
        onRetried={() => {
          setDetailOpen(false)
          setSelectedTask(null)
        }}
      />
    </>
  )
}
