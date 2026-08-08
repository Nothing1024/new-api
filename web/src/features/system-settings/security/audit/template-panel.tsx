/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Loader2, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

import {
  applyTemplate,
  disableTemplate,
  enableTemplate,
  listTemplates,
  removeTemplate,
} from './api'
import type { AuditTemplate } from './types'

export function TemplatePanel() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [pendingAction, setPendingAction] = useState<AuditTemplate | null>(null)
  const [busyId, setBusyId] = useState<string | null>(null)

  const { data: templates, isLoading, isError, refetch } = useQuery({
    queryKey: ['audit-templates'],
    queryFn: listTemplates,
    retry: 1,
  })

  const refresh = async () => {
    await queryClient.invalidateQueries({ queryKey: ['audit-templates'] })
    // Template operations change the applied rule set; refresh the rules list.
    await queryClient.invalidateQueries({ queryKey: ['watchlist-rules'] })
  }

  const handleApply = async (template: AuditTemplate) => {
    setBusyId(template.id)
    try {
      const result = await applyTemplate(template.id)
      const base = t('Applied N rules', { count: result.applied })
      const message =
        result.regex_disabled > 0
          ? `${base}. ${t('M regex rules disabled by default', {
              count: result.regex_disabled,
            })}`
          : base
      toast.success(message)
      await refresh()
    } catch (err) {
      toast.error(extractMessage(err, t('Apply failed')))
    } finally {
      setBusyId(null)
    }
  }

  const handleEnable = async (template: AuditTemplate) => {
    setBusyId(template.id)
    try {
      const result = await enableTemplate(template.id)
      const message =
        result.regex_skipped > 0
          ? `${t('Enabled N rules', { count: result.enabled })}. ${t(
              'M regex rules skipped (limit)',
              { count: result.regex_skipped }
            )}`
          : t('Enabled N rules', { count: result.enabled })
      toast.success(message)
      await refresh()
    } catch (err) {
      toast.error(extractMessage(err, t('Enable failed')))
    } finally {
      setBusyId(null)
    }
  }

  const handleDisable = async (template: AuditTemplate) => {
    setBusyId(template.id)
    try {
      const result = await disableTemplate(template.id)
      toast.success(t('Disabled N rules', { count: result.disabled }))
      await refresh()
    } catch (err) {
      toast.error(extractMessage(err, t('Disable failed')))
    } finally {
      setBusyId(null)
    }
  }

  const handleRemove = async (template: AuditTemplate | null) => {
    setPendingAction(null)
    if (!template) return
    setBusyId(template.id)
    try {
      const result = await removeTemplate(template.id)
      toast.success(t('Removed N rules', { count: result.removed }))
      await refresh()
    } catch (err) {
      toast.error(extractMessage(err, t('Remove failed')))
    } finally {
      setBusyId(null)
    }
  }

  if (isLoading) {
    return (
      <div className='flex items-center gap-2 text-sm text-muted-foreground'>
        <Loader2 className='size-4 animate-spin' aria-hidden='true' />
        {t('Loading')}
      </div>
    )
  }

  if (isError) {
    return (
      <div className='flex items-center gap-3 text-sm text-muted-foreground'>
        <span>{t('Failed to load')}</span>
        <Button variant='outline' size='sm' onClick={() => void refetch()}>
          {t('Retry')}
        </Button>
      </div>
    )
  }

  if (!templates || templates.length === 0) {
    return <p className='text-sm text-muted-foreground'>{t('No templates')}</p>
  }

  return (
    <div className='grid gap-3 sm:grid-cols-2'>
      {(templates ?? []).map((template) => (
        <TemplateCard
          key={template.id}
          template={template}
          busy={busyId === template.id}
          onApply={() => void handleApply(template)}
          onEnable={() => void handleEnable(template)}
          onDisable={() => void handleDisable(template)}
          onRemove={() => setPendingAction(template)}
        />
      ))}

      <Dialog
        open={pendingAction != null}
        onOpenChange={(open) => !open && setPendingAction(null)}
        title={t('Remove template confirm title')}
        footer={
          <>
            <Button variant='outline' onClick={() => setPendingAction(null)}>
              {t('Cancel')}
            </Button>
            <Button
              variant='destructive'
              onClick={() => void handleRemove(pendingAction)}
            >
              {t('Remove')}
            </Button>
          </>
        }
      >
        <p className='text-sm text-muted-foreground'>
          {t('Remove template confirm body')}
        </p>
      </Dialog>
    </div>
  )
}

function TemplateCard({
  template,
  busy,
  onApply,
  onEnable,
  onDisable,
  onRemove,
}: {
  template: AuditTemplate
  busy: boolean
  onApply: () => void
  onEnable: () => void
  onDisable: () => void
  onRemove: () => void
}) {
  const { t } = useTranslation()
  const applied = template.applied_count > 0

  return (
    <div
      className={cn(
        'flex flex-col gap-2 rounded-lg border p-4',
        applied && 'border-primary/40'
      )}
    >
      <div className='flex items-start justify-between gap-2'>
        <div className='min-w-0'>
          <h4 className='truncate text-sm font-medium'>{template.name}</h4>
          <p className='text-muted-foreground mt-0.5 line-clamp-2 text-xs'>
            {template.description || t('No description')}
          </p>
        </div>
      </div>

      <div className='flex items-center gap-2 text-xs text-muted-foreground'>
        <span>
          {t('Rules count')}: {template.rule_count}
        </span>
        <StatusBadge template={template} />
      </div>

      <div className='mt-auto flex flex-wrap gap-1.5 pt-2'>
        {!applied ? (
          <Button size='sm' onClick={onApply} disabled={busy}>
            {t('Apply')}
          </Button>
        ) : (
          <>
            <Button size='sm' onClick={onDisable} disabled={busy}>
              {t('Disable')}
            </Button>
            <Button
              size='sm'
              variant='outline'
              onClick={onEnable}
              disabled={busy}
            >
              {t('Enable')}
            </Button>
            <Button
              size='sm'
              variant='ghost'
              onClick={onRemove}
              disabled={busy}
              className='text-rose-500'
            >
              <Trash2 className='size-3.5' aria-hidden='true' />
              {t('Remove')}
            </Button>
          </>
        )}
      </div>
    </div>
  )
}

function StatusBadge({ template }: { template: AuditTemplate }) {
  const { t } = useTranslation()
  if (template.applied_count > 0) {
    return (
      <span className='bg-emerald-100 text-emerald-700 rounded px-1.5 py-0.5 text-[10px] font-medium'>
        {t('Applied N rules', { count: template.applied_count })}
      </span>
    )
  }
  if (template.status === 'disabled') {
    return (
      <span className='bg-muted text-muted-foreground rounded px-1.5 py-0.5 text-[10px] font-medium'>
        {t('Disabled')}
      </span>
    )
  }
  return (
    <span className='bg-muted text-muted-foreground rounded px-1.5 py-0.5 text-[10px] font-medium'>
      {t('Unapplied')}
    </span>
  )
}

function extractMessage(
  err: unknown,
  fallback: string
): string {
  if (
    err &&
    typeof err === 'object' &&
    'response' in err &&
    err.response &&
    typeof err.response === 'object' &&
    'data' in err.response &&
    err.response.data &&
    typeof err.response.data === 'object' &&
    'message' in err.response.data
  ) {
    const message = (err.response.data as { message?: unknown }).message
    if (typeof message === 'string' && message !== '') return message
  }
  return fallback
}
