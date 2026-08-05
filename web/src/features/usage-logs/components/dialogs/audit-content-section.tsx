/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Check, Copy, FileText, Loader2, ShieldAlert } from 'lucide-react'

import { api } from '@/lib/api'
import { cn } from '@/lib/utils'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'

import type {
  AuditDerivedFacts,
  AuditSegment,
  LogContent,
} from '../../types'

const SEGMENT_KIND_LABELS: Record<string, string> = {
  system: 'System',
  user: 'User',
  assistant: 'Assistant',
  tool_call: 'Tool Call',
  tool_result: 'Tool Result',
  image: 'Image',
  audio: 'Audio',
}

const SEVERITY_CLASS: Record<string, string> = {
  high: 'bg-rose-100 text-rose-700',
  medium: 'bg-amber-100 text-amber-700',
  low: 'bg-yellow-100 text-yellow-700',
}

function DerivedChips({ derived }: { derived?: AuditDerivedFacts }) {
  if (!derived) return null
  const chips: string[] = [
    ...(derived.domains ?? []),
    ...(derived.urls ?? []),
    ...(derived.tools ?? []),
  ]
  if (chips.length === 0 && derived.chars != null) {
    chips.push(`${derived.chars} chars`)
  }
  if (chips.length === 0) return null
  return (
    <div className='mt-1 flex flex-wrap gap-1'>
      {chips.slice(0, 12).map((c) => (
        <span
          key={c}
          className='bg-muted text-muted-foreground max-w-[16rem] truncate rounded px-1.5 py-0.5 text-[10px]'
          title={c}
        >
          {c}
        </span>
      ))}
    </div>
  )
}

function SegmentRow({
  segment,
  copiedText,
  onCopy,
}: {
  segment: AuditSegment
  copiedText: string | null
  onCopy: (text: string) => void
}) {
  const { t } = useTranslation()
  const kindLabel = SEGMENT_KIND_LABELS[segment.kind] ?? segment.kind
  const hasCopyableText =
    segment.text != null &&
    (segment.mode === 'full' || segment.mode === 'preview')
  const isCopied = copiedText === segment.text

  return (
    <div className='bg-background/60 flex min-w-0 flex-col gap-1.5 rounded border p-2'>
      <div className='flex min-w-0 items-center gap-2 text-xs'>
        <span className='text-foreground shrink-0 font-medium'>
          {kindLabel}
        </span>
        <span className='bg-muted text-muted-foreground rounded px-1.5 py-0.5 text-[10px] uppercase'>
          {segment.mode}
        </span>
        {segment.truncated && (
          <span className='text-muted-foreground text-[10px]'>
            {t('truncated')}
          </span>
        )}
        <span className='text-muted-foreground ml-auto text-[10px]'>
          {segment.bytes} B
        </span>
        {hasCopyableText && (
          <button
            type='button'
            onClick={() => onCopy(segment.text ?? '')}
            className='text-muted-foreground hover:text-foreground shrink-0'
            title={t('Copy')}
          >
            {isCopied ? (
              <Check className='size-3.5 text-emerald-500' aria-hidden='true' />
            ) : (
              <Copy className='size-3.5' aria-hidden='true' />
            )}
          </button>
        )}
      </div>
      {segment.text ? (
        <pre className='text-muted-foreground max-h-40 min-w-0 overflow-auto whitespace-pre-wrap break-words text-xs leading-relaxed'>
          {segment.text}
        </pre>
      ) : (
        segment.reason && (
          <span className='text-muted-foreground text-[10px] italic'>
            {segment.reason}
          </span>
        )
      )}
      <DerivedChips derived={segment.derived} />
    </div>
  )
}

export function AuditContentSection({
  requestId,
}: {
  requestId: string
}) {
  const { t } = useTranslation()
  const { copiedText, copyToClipboard } = useCopyToClipboard({ notify: false })
  const [content, setContent] = useState<LogContent | null>(null)
  const [status, setStatus] = useState<'loading' | 'ok' | 'error' | 'empty'>(
    'loading'
  )

  useEffect(() => {
    let cancelled = false
    setStatus('loading')
    setContent(null)
    api
      .get('/api/log/content', { params: { request_id: requestId } })
      .then((res) => {
        if (cancelled) return
        const data = res.data as { success: boolean; data?: LogContent }
        if (!data.success || !data.data) {
          setStatus('empty')
          return
        }
        setContent(data.data)
        setStatus('ok')
      })
      .catch(() => {
        if (!cancelled) setStatus('error')
      })
    return () => {
      cancelled = true
    }
  }, [requestId])

  if (status === 'loading') {
    return (
      <div className='flex items-center gap-2 py-3 text-sm text-muted-foreground'>
        <Loader2 className='size-4 animate-spin' aria-hidden='true' />
        {t('Loading')}
      </div>
    )
  }

  if (status === 'error') {
    return (
      <div className='flex flex-col items-start gap-2 py-3 text-sm'>
        <span className='text-muted-foreground'>{t('Failed to load')}</span>
        <button
          type='button'
          onClick={() => {
            setStatus('loading')
            setContent(null)
            api
              .get('/api/log/content', { params: { request_id: requestId } })
              .then((res) => {
                const data = res.data as {
                  success: boolean
                  data?: LogContent
                }
                if (!data.success || !data.data) {
                  setStatus('empty')
                  return
                }
                setContent(data.data)
                setStatus('ok')
              })
              .catch(() => setStatus('error'))
          }}
          className='text-primary text-xs underline'
        >
          {t('Retry')}
        </button>
      </div>
    )
  }

  if (status === 'empty' || !content) {
    return (
      <div className='flex items-center gap-2 py-3 text-sm text-muted-foreground'>
        <FileText className='size-4' aria-hidden='true' />
        {t('No audit content')}
      </div>
    )
  }

  return (
    <div className='min-w-0 space-y-2'>
      <div className='flex flex-wrap items-center gap-2 text-xs'>
        <span className='bg-muted text-muted-foreground rounded px-1.5 py-0.5 uppercase'>
          {content.fidelity}
        </span>
        {content.hit_count > 0 && (
          <span
            className={cn(
              'flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px] font-medium',
              SEVERITY_CLASS[content.hit_severity] ?? 'bg-muted'
            )}
          >
            <ShieldAlert className='size-3' aria-hidden='true' />
            {t('Audit hits')}: {content.hit_count}
          </span>
        )}
      </div>

      <div className='min-w-0 space-y-1.5'>
        {(content.segments ?? []).map((segment, i) => (
          <SegmentRow
            key={`${i}-${segment.kind}-${segment.mode}`}
            segment={segment}
            copiedText={copiedText}
            onCopy={(text) => void copyToClipboard(text)}
          />
        ))}
      </div>

      {(content.flags ?? []).length > 0 && (
        <div className='min-w-0 space-y-1 pt-1'>
          <div className='text-muted-foreground text-xs font-medium'>
            {t('Matched rules')}
          </div>
          {(content.flags ?? []).map((flag) => (
            <div
              key={`${flag.rule_id}-${flag.pattern_snapshot}`}
              className='bg-background/60 flex min-w-0 items-center gap-2 rounded border px-2 py-1 text-xs'
            >
              <span
                className={cn(
                  'rounded px-1.5 py-0.5 text-[10px] font-medium',
                  SEVERITY_CLASS[flag.severity] ?? 'bg-muted'
                )}
              >
                {flag.severity}
              </span>
              <span className='text-muted-foreground font-mono text-[10px]'>
                #{flag.rule_id}
              </span>
              <span className='min-w-0 truncate' title={flag.pattern_snapshot}>
                {flag.pattern_snapshot}
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
