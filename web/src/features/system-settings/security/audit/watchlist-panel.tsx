/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useQuery } from '@tanstack/react-query'
import { useEffect, useRef, useState, type ChangeEvent } from 'react'
import { useTranslation } from 'react-i18next'
import {
  Download,
  Loader2,
  Pencil,
  Plus,
  RefreshCw,
  Trash2,
  Upload,
} from 'lucide-react'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { cn } from '@/lib/utils'

import {
  createWatchlistRule,
  deleteWatchlistRule,
  exportWatchlist,
  getRescanStatus,
  importWatchlist,
  listWatchlistRules,
  triggerRescan,
  updateWatchlistRule,
} from './api'
import type {
  AuditWatchlistRule,
  AuditWatchlistRuleInput,
  RescanStatus,
  WatchlistKind,
  WatchlistSeverity,
} from './types'

const KINDS: WatchlistKind[] = ['domain', 'keyword', 'regex']
const SEVERITIES: WatchlistSeverity[] = ['low', 'medium', 'high']

const SEVERITY_CLASS: Record<string, string> = {
  high: 'bg-rose-100 text-rose-700',
  medium: 'bg-amber-100 text-amber-700',
  low: 'bg-yellow-100 text-yellow-700',
}

type EditorState = {
  open: boolean
  rule?: AuditWatchlistRule
}

function emptyForm(): AuditWatchlistRuleInput {
  return { kind: 'keyword', pattern: '', severity: 'medium', enabled: true }
}

export function WatchlistPanel() {
  const { t } = useTranslation()
  const [editor, setEditor] = useState<EditorState>({ open: false })
  const [rescan, setRescan] = useState<RescanStatus | null>(null)
  const [rescanRunning, setRescanRunning] = useState(false)
  const [exporting, setExporting] = useState(false)
  const [importing, setImporting] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const {
    data: rules,
    isLoading: loading,
    refetch: refreshRules,
  } = useQuery({
    queryKey: ['watchlist-rules'],
    queryFn: listWatchlistRules,
    retry: 1,
  })

  const loadRules = () => void refreshRules()


  const onExport = async () => {
    setExporting(true)
    try {
      const blob = await exportWatchlist()
      const url = URL.createObjectURL(blob)
      const anchor = document.createElement('a')
      anchor.href = url
      const filename = `audit-rules-${new Date().toISOString().replaceAll(/[:.]/g, '-')}.json`
      anchor.download = filename
      document.body.appendChild(anchor)
      anchor.click()
      anchor.remove()
      URL.revokeObjectURL(url)
    } catch {
      toast.error(t('Export failed'))
    } finally {
      setExporting(false)
    }
  }

  const onImportFile = async (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    e.target.value = ''
    if (!file) return
    setImporting(true)
    try {
      const text = await file.text()
      const imported = await importWatchlist(text)
      toast.success(t('Imported X rules', { count: imported }))
      void loadRules()
    } catch (err) {
      const message =
        (err as { response?: { data?: { message?: string } } })?.response?.data
          ?.message ?? t('Import failed')
      toast.error(message)
    } finally {
      setImporting(false)
    }
  }

  useEffect(() => {
    void loadRules()
    // 页面加载时若已有重扫在跑（其他会话触发），恢复进度轮询。
    getRescanStatus()
      .then((st) => {
        if (st.status === 'running') {
          setRescan(st)
          setRescanRunning(true)
        }
      })
      .catch(() => {})
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    if (!rescanRunning) return
    const timer = window.setInterval(async () => {
      try {
        const st = await getRescanStatus()
        setRescan(st)
        if (st.status === 'done') {
          setRescanRunning(false)
          window.clearInterval(timer)
          toast.success(t('Rescan complete'))
          void loadRules()
        } else if (st.status === 'error') {
          setRescanRunning(false)
          window.clearInterval(timer)
          toast.error(t('Rescan failed'))
        } else if (st.status === 'no_op') {
          setRescanRunning(false)
          window.clearInterval(timer)
          toast.info(t('Nothing to rescan'))
        }
      } catch {
        setRescanRunning(false)
        window.clearInterval(timer)
      }
    }, 2000)
    return () => window.clearInterval(timer)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [rescanRunning])

  const onSave = async (input: AuditWatchlistRuleInput, id?: number) => {
    try {
      if (id != null) {
        await updateWatchlistRule(id, input)
      } else {
        await createWatchlistRule(input)
      }
      toast.success(t('Saved'))
      setEditor({ open: false })
      void loadRules()
    } catch (err) {
      const message =
        (err as { response?: { data?: { message?: string } } })?.response?.data
          ?.message ?? t('Save failed')
      toast.error(message)
    }
  }

  const onDelete = async (rule: AuditWatchlistRule) => {
    if (!window.confirm(t('Delete rule confirm'))) return
    try {
      await deleteWatchlistRule(rule.id)
      toast.success(t('Deleted'))
      void loadRules()
    } catch {
      toast.error(t('Delete failed'))
    }
  }

  const onRescan = async () => {
    if (!window.confirm(t('Rescan confirm'))) return
    setRescanRunning(true)
    setRescan({ processed: 0, total: 0, status: 'running', wl_version: 0 })
    try {
      await triggerRescan()
    } catch (err) {
      const message =
        (err as { response?: { data?: { message?: string } } })?.response?.data
          ?.message ?? ''
      if (message.includes('already running')) {
        toast.info(t('Rescan in progress'))
      } else {
        setRescanRunning(false)
        toast.error(message || t('Rescan failed'))
      }
    }
  }

  return (
    <>
    <div className='flex flex-col gap-3'>
      <div className='flex flex-wrap items-center gap-2'>
        <Button
          variant='outline'
          size='sm'
          onClick={onRescan}
          disabled={rescanRunning}
        >
          <RefreshCw
            className={cn('size-4', rescanRunning && 'animate-spin')}
            aria-hidden='true'
          />
          {t('Rescan')}
        </Button>
        <Button variant='outline' size='sm' onClick={onExport} disabled={exporting}>
          <Download className='size-4' aria-hidden='true' />
          {t('Export rules')}
        </Button>
        <Button variant='outline' size='sm' onClick={() => fileInputRef.current?.click()} disabled={importing}>
          <Upload className='size-4' aria-hidden='true' />
          {t('Import rules')}
        </Button>
        <input
          ref={fileInputRef}
          type='file'
          accept='.json,application/json'
          className='hidden'
          onChange={onImportFile}
        />
        <div className='ml-auto'>
          <Button size='sm' onClick={() => setEditor({ open: true, rule: undefined })}>
            <Plus className='size-4' aria-hidden='true' />
            {t('Add rule')}
          </Button>
        </div>
      </div>
      <div className='flex flex-col gap-2'>
        {rescanRunning && rescan && (
          <div className='mb-3 flex items-center gap-2 text-xs'>
            <span className='text-muted-foreground'>{t('Rescanning')}</span>
            <div className='bg-muted h-2 w-full max-w-xs overflow-hidden rounded-full'>
              <div
                className='bg-primary h-full rounded-full transition-all'
                style={{
                  width: `${
                    rescan.total > 0
                      ? Math.min(100, (rescan.processed / rescan.total) * 100)
                      : 10
                  }%`,
                }}
              />
            </div>
            <span className='text-muted-foreground'>
              {rescan.processed}/{rescan.total}
            </span>
          </div>
        )}

        {loading ? (
          <div className='flex items-center gap-2 text-sm text-muted-foreground'>
            <Loader2 className='size-4 animate-spin' aria-hidden='true' />
            {t('Loading')}
          </div>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('ID')}</TableHead>
                <TableHead>{t('Kind')}</TableHead>
                <TableHead>{t('Pattern')}</TableHead>
                <TableHead>{t('Severity')}</TableHead>
                <TableHead>{t('Enabled')}</TableHead>
                <TableHead>{t('Note')}</TableHead>
                <TableHead>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {(rules ?? []).length === 0 && (
                <TableRow>
                  <TableCell colSpan={7} className='text-muted-foreground text-center'>
                    {t('No rules')}
                  </TableCell>
                </TableRow>
              )}
              {(rules ?? []).map((rule) => (
                <TableRow key={rule.id}>
                  <TableCell>{rule.id}</TableCell>
                  <TableCell className='text-xs uppercase'>{rule.kind}</TableCell>
                  <TableCell className='font-mono text-xs'>{rule.pattern}</TableCell>
                  <TableCell>
                    <span
                      className={cn(
                        'rounded px-1.5 py-0.5 text-[10px] font-medium',
                        SEVERITY_CLASS[rule.severity] ?? 'bg-muted'
                      )}
                    >
                      {rule.severity}
                    </span>
                  </TableCell>
                  <TableCell>
                    <span
                      className={cn(
                        'text-xs',
                        rule.enabled ? 'text-emerald-600' : 'text-muted-foreground'
                      )}
                    >
                      {rule.enabled ? t('Yes') : t('No')}
                    </span>
                  </TableCell>
                  <TableCell className='max-w-[16rem] truncate text-xs text-muted-foreground' title={rule.note}>
                    {rule.note || '—'}
                  </TableCell>
                  <TableCell>
                    <div className='flex items-center gap-1'>
                      <Button
                        variant='ghost'
                        size='icon'
                        onClick={() => setEditor({ open: true, rule })}
                        title={t('Edit')}
                      >
                        <Pencil className='size-4' aria-hidden='true' />
                      </Button>
                      <Button
                        variant='ghost'
                        size='icon'
                        onClick={() => void onDelete(rule)}
                        title={t('Delete')}
                      >
                        <Trash2 className='size-4 text-rose-500' aria-hidden='true' />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </div>
    </div>

    <RuleEditorDialog
      state={editor}
      onClose={() => setEditor({ open: false })}
      onSave={onSave}
    />
    </>
  )
}

function RuleEditorDialog({
  state,
  onClose,
  onSave,
}: {
  state: EditorState
  onClose: () => void
  onSave: (input: AuditWatchlistRuleInput, id?: number) => void
}) {
  const { t } = useTranslation()
  const [form, setForm] = useState<AuditWatchlistRuleInput>(emptyForm())

  useEffect(() => {
    if (state.open) {
      setForm(
        state.rule
          ? {
              kind: state.rule.kind,
              pattern: state.rule.pattern,
              severity: state.rule.severity,
              enabled: state.rule.enabled,
              note: state.rule.note,
            }
          : emptyForm()
      )
    }
  }, [state])

  const patternError = form.pattern.trim() === ''

  return (
    <Dialog
      open={state.open}
      onOpenChange={(open) => !open && onClose()}
      title={state.rule ? t('Edit rule') : t('Add rule')}
    >
      <div className='space-y-3'>
        <div className='space-y-1.5'>
          <Label>{t('Kind')}</Label>
          <Select
            value={form.kind}
            onValueChange={(v) =>
              setForm((f) => ({ ...f, kind: v as WatchlistKind }))
            }
          >
            <SelectTrigger className='w-full'>
              <SelectValue placeholder={t('Kind')} />
            </SelectTrigger>
            <SelectContent>
              {KINDS.map((k) => (
                <SelectItem key={k} value={k}>
                  {k}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className='space-y-1.5'>
          <Label>{t('Pattern')}</Label>
          <Input
            value={form.pattern}
            onChange={(e) =>
              setForm((f) => ({ ...f, pattern: e.target.value }))
            }
            placeholder={t('Pattern placeholder')}
            aria-invalid={patternError}
          />
          {patternError && (
            <p className='text-destructive text-xs'>{t('Pattern required')}</p>
          )}
        </div>

        <div className='space-y-1.5'>
          <Label>{t('Severity')}</Label>
          <Select
            value={form.severity}
            onValueChange={(v) =>
              setForm((f) => ({ ...f, severity: v as WatchlistSeverity }))
            }
          >
            <SelectTrigger className='w-full'>
              <SelectValue placeholder={t('Severity')} />
            </SelectTrigger>
            <SelectContent>
              {SEVERITIES.map((s) => (
                <SelectItem key={s} value={s}>
                  {s}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className='space-y-1.5'>
          <Label>{t('Note')}</Label>
          <Input
            value={form.note ?? ''}
            onChange={(e) =>
              setForm((f) => ({ ...f, note: e.target.value }))
            }
            placeholder={t('Note (optional)')}
          />
        </div>

        <div className='flex items-center gap-2'>
          <Switch
            checked={form.enabled}
            onCheckedChange={(checked) =>
              setForm((f) => ({ ...f, enabled: checked }))
            }
          />
          <Label>{t('Enabled')}</Label>
        </div>

        <div className='flex justify-end gap-2 pt-2'>
          <Button variant='outline' onClick={onClose}>
            {t('Cancel')}
          </Button>
          <Button
            disabled={patternError}
            onClick={() => void onSave(form, state.rule?.id)}
          >
            {t('Save')}
          </Button>
        </div>
      </div>
    </Dialog>
  )
}
