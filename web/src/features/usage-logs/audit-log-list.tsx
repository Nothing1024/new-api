/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { ChevronLeft, ChevronRight, FileText, Loader2, X } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { useSystemOptions, getOptionValue } from '@/features/system-settings/hooks/use-system-options'
import { listAuditLogs } from '@/features/system-settings/security/audit/api'
import type { AuditLogItem } from '@/features/system-settings/security/audit/types'
import { formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'

import { AuditContentSection } from './components/dialogs/audit-content-section'

const PAGE_SIZE = 20

const SEVERITY_CLASS: Record<string, string> = {
  high: 'bg-rose-100 text-rose-700',
  medium: 'bg-amber-100 text-amber-700',
  low: 'bg-yellow-100 text-yellow-700',
}

function toEpochSeconds(value: string): number | undefined {
  if (!value) return undefined
  const timestamp = new Date(value).getTime()
  return Number.isNaN(timestamp) ? undefined : Math.floor(timestamp / 1000)
}

export function AuditLogListPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { data: systemOptionsData } = useSystemOptions()
  const auditEnabled = getOptionValue(systemOptionsData?.data, {
    AuditEnabled: false,
  }).AuditEnabled

  const [severity, setSeverity] = useState('')
  const [minHit, setMinHit] = useState('')
  const [startTime, setStartTime] = useState('')
  const [endTime, setEndTime] = useState('')
  const [userId, setUserId] = useState('')
  const [modelName, setModelName] = useState('')
  const [page, setPage] = useState(1)
  const [detail, setDetail] = useState<AuditLogItem | null>(null)

  const hasFilters =
    severity !== '' || minHit !== '' || startTime !== '' || endTime !== '' ||
    userId !== '' || modelName !== ''

  const { data, isLoading, isError, refetch, isFetching } = useQuery({
    queryKey: ['audit-logs', severity, minHit, startTime, endTime, userId, modelName, page],
    queryFn: () =>
      listAuditLogs({
        severity: severity || undefined,
        min_hit: minHit ? Number(minHit) : undefined,
        start_timestamp: toEpochSeconds(startTime),
        end_timestamp: toEpochSeconds(endTime),
        user_id: userId || undefined,
        model_name: modelName || undefined,
        p: page,
        page_size: PAGE_SIZE,
      }),
    retry: 1,
  })

  const items = data?.items ?? []
  const total = data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  const clearFilters = () => {
    setSeverity('')
    setMinHit('')
    setStartTime('')
    setEndTime('')
    setUserId('')
    setModelName('')
    setPage(1)
  }

  const alreadyCheckedAuditEnabled = !!systemOptionsData

  const loadingSpinner = (
    <div className='flex h-full items-center justify-center'>
      <div className='flex items-center gap-2 text-sm text-muted-foreground'>
        <Loader2 className='size-4 animate-spin' aria-hidden='true' />
        {t('Loading')}
      </div>
    </div>
  )

  let contentBody
  if (!alreadyCheckedAuditEnabled) {
    contentBody = loadingSpinner
  } else if (isError) {
    contentBody = (
      <div className='flex h-full flex-col items-center justify-center gap-3'>
        <p className='text-sm text-muted-foreground'>{t('Failed to load')}</p>
        <Button variant='outline' size='sm' onClick={() => void refetch()}>
          {t('Retry')}
        </Button>
      </div>
    )
  } else if (isLoading) {
    contentBody = loadingSpinner
  } else if (items.length === 0) {
    contentBody = (
      <div className='flex h-full items-center justify-center'>
        {!auditEnabled ? (
          <div className='flex max-w-sm flex-col items-center gap-3 rounded-lg border p-6 text-center'>
            <p className='text-muted-foreground text-sm'>
              {t('Audit never enabled hint')}
            </p>
            <Button
              size='sm'
              onClick={() =>
                void navigate({
                  to: '/system-settings/security/$section',
                  params: { section: 'audit' },
                })
              }
            >
              {t('Go to Audit Settings')}
            </Button>
          </div>
        ) : (
          <div className='flex max-w-sm flex-col items-center gap-3 rounded-lg border p-6 text-center'>
            <FileText className='size-6 text-muted-foreground' aria-hidden='true' />
            <p className='text-muted-foreground text-sm'>
              {t('No audit logs found')}
            </p>
            {hasFilters ? (
              <Button variant='outline' size='sm' onClick={clearFilters}>
                {t('Clear filters')}
              </Button>
            ) : null}
          </div>
        )}
      </div>
    )
  } else {
    contentBody = (
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Time')}</TableHead>
            <TableHead>{t('User')}</TableHead>
            <TableHead>{t('Model')}</TableHead>
            <TableHead>{t('Fidelity')}</TableHead>
            <TableHead>{t('Hits')}</TableHead>
            <TableHead>{t('Severity')}</TableHead>
            <TableHead>{t('Request ID')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {items.map((item) => (
            <TableRow
              key={item.request_id}
              className='cursor-pointer'
              onClick={() => setDetail(item)}
            >
              <TableCell className='text-xs whitespace-nowrap'>
                {formatTimestampToDate(item.created_at)}
              </TableCell>
              <TableCell className='text-xs'>
                {item.user_id > 0 ? `#${item.user_id}` : '—'}
              </TableCell>
              <TableCell className='max-w-[12rem] truncate text-xs'>
                {item.model_name || '—'}
              </TableCell>
              <TableCell className='text-xs uppercase'>{item.fidelity}</TableCell>
              <TableCell className='text-xs'>{item.hit_count}</TableCell>
              <TableCell>
                <span
                  className={cn(
                    'rounded px-1.5 py-0.5 text-[10px] font-medium',
                    SEVERITY_CLASS[item.hit_severity] ?? 'bg-muted'
                  )}
                >
                  {item.hit_severity || '—'}
                </span>
              </TableCell>
              <TableCell className='font-mono text-xs'>
                {item.request_id.slice(-8)}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    )
  }

  return (
    <div className='flex h-full min-h-0 flex-col gap-3'>
      <div className='flex flex-wrap items-center gap-2'>
        <Select value={severity} onValueChange={(v) => { setSeverity(v ?? ''); setPage(1) }}>
          <SelectTrigger className='w-32' size='sm'>
            <SelectValue placeholder={t('Severity')} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value=''>—</SelectItem>
            <SelectItem value='high'>high</SelectItem>
            <SelectItem value='medium'>medium</SelectItem>
            <SelectItem value='low'>low</SelectItem>
          </SelectContent>
        </Select>

        <Input
          type='number'
          min={0}
          placeholder={t('Min hits')}
          value={minHit}
          onChange={(e) => { setMinHit(e.target.value); setPage(1) }}
          className='w-24'
        />

        <Input
          type='datetime-local'
          placeholder={t('Start time')}
          value={startTime}
          onChange={(e) => { setStartTime(e.target.value); setPage(1) }}
          className='w-44'
        />

        <Input
          type='datetime-local'
          placeholder={t('End time')}
          value={endTime}
          onChange={(e) => { setEndTime(e.target.value); setPage(1) }}
          className='w-44'
        />

        <Input
          placeholder={t('User ID')}
          value={userId}
          onChange={(e) => { setUserId(e.target.value); setPage(1) }}
          className='w-28'
        />

        <Input
          placeholder={t('Model')}
          value={modelName}
          onChange={(e) => { setModelName(e.target.value); setPage(1) }}
          className='w-40'
        />

        <Button
          variant='outline'
          size='sm'
          onClick={clearFilters}
          disabled={!hasFilters}
        >
          <X className='size-3.5' aria-hidden='true' />
          {t('Clear filters')}
        </Button>
      </div>

      {hasFilters && isFetching ? (
        <div className='flex items-center gap-2 text-sm text-muted-foreground'>
          <Loader2 className='size-4 animate-spin' aria-hidden='true' />
          {t('Loading')}
        </div>
      ) : null}

      <div className='min-h-0 flex-1 overflow-auto'>
        {contentBody}
      </div>

      {alreadyCheckedAuditEnabled && items.length > 0 && (
        <div className='flex items-center justify-between text-xs text-muted-foreground'>
          <span>
            {t('Total')}: {total}
          </span>
          <div className='flex items-center gap-1'>
            <Button
              variant='outline'
              size='icon'
              disabled={page <= 1}
              onClick={() => setPage((p) => Math.max(1, p - 1))}
            >
              <ChevronLeft className='size-4' aria-hidden='true' />
            </Button>
            <span className='px-1'>
              {page} / {totalPages}
            </span>
            <Button
              variant='outline'
              size='icon'
              disabled={page >= totalPages}
              onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
            >
              <ChevronRight className='size-4' aria-hidden='true' />
            </Button>
          </div>
        </div>
      )}

      <Dialog
        open={detail != null}
        onOpenChange={(open) => !open && setDetail(null)}
        title={detail ? t('Audit log detail') : ''}
        contentClassName='sm:max-w-3xl'
      >
        {detail && (
          <AuditContentSection requestId={detail.request_id} />
        )}
      </Dialog>
    </div>
  )
}
