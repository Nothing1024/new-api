/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { api } from '@/lib/api'

import type {
  ApplyTemplateResult,
  AuditLogItem,
  AuditLogListParams,
  AuditLogListResponse,
  AuditTemplate,
  AuditWatchlistRule,
  AuditWatchlistRuleInput,
  DisableTemplateResult,
  EnableTemplateResult,
  RemoveTemplateResult,
  RescanStatus,
} from './types'

// ============================================================================
// Watchlist Rules CRUD
// ============================================================================

export async function listWatchlistRules(): Promise<AuditWatchlistRule[]> {
  const res = await api.get('/api/audit/watchlist')
  const data = res.data as { success: boolean; data: AuditWatchlistRule[] }
  return data.success ? data.data : []
}

export async function createWatchlistRule(
  input: AuditWatchlistRuleInput
): Promise<AuditWatchlistRule> {
  const res = await api.post('/api/audit/watchlist', input)
  return (res.data as { data: AuditWatchlistRule }).data
}

export async function updateWatchlistRule(
  id: number,
  input: AuditWatchlistRuleInput
): Promise<AuditWatchlistRule> {
  const res = await api.put(`/api/audit/watchlist/${id}`, input)
  return (res.data as { data: AuditWatchlistRule }).data
}

export async function deleteWatchlistRule(id: number): Promise<void> {
  await api.delete(`/api/audit/watchlist/${id}`)
}

// ============================================================================
// Rescan
// ============================================================================

export async function triggerRescan(): Promise<{ wl_version: number }> {
  const res = await api.post('/api/audit/rescan')
  return (res.data as { data: { wl_version: number } }).data
}

export async function getRescanStatus(): Promise<RescanStatus> {
  const res = await api.get('/api/audit/rescan/status')
  return (res.data as { data: RescanStatus }).data
}

// ============================================================================
// Template Packages
// ============================================================================

export async function listTemplates(): Promise<AuditTemplate[]> {
  const res = await api.get<{ success: boolean; data: AuditTemplate[] }>(
    '/api/audit/templates'
  )
  return res.data.data
}

export async function applyTemplate(id: string): Promise<ApplyTemplateResult> {
  const res = await api.post<{ success: boolean; data: ApplyTemplateResult }>(
    `/api/audit/templates/${id}/apply`
  )
  return res.data.data
}

export async function enableTemplate(id: string): Promise<EnableTemplateResult> {
  const res = await api.post<{ success: boolean; data: EnableTemplateResult }>(
    `/api/audit/templates/${id}/enable`
  )
  return res.data.data
}

export async function disableTemplate(id: string): Promise<DisableTemplateResult> {
  const res = await api.post<{ success: boolean; data: DisableTemplateResult }>(
    `/api/audit/templates/${id}/disable`
  )
  return res.data.data
}

export async function removeTemplate(id: string): Promise<RemoveTemplateResult> {
  const res = await api.delete<{ success: boolean; data: RemoveTemplateResult }>(
    `/api/audit/templates/${id}`
  )
  return res.data.data
}

// ============================================================================
// Import / Export
// ============================================================================

export async function exportWatchlist(): Promise<Blob> {
  const res = await api.get('/api/audit/watchlist/export', {
    responseType: 'blob',
  })
  return res.data as Blob
}

export async function importWatchlist(json: string): Promise<number> {
  const res = await api.post<{ success: boolean; data?: { imported?: number }; message?: string }>(
    '/api/audit/watchlist/import',
    json,
    { headers: { 'Content-Type': 'application/json' } }
  )
  return res.data?.data?.imported ?? 0
}

// ============================================================================
// Audit Log List
// ============================================================================

export async function listAuditLogs(
  params: AuditLogListParams
): Promise<{ items: AuditLogItem[]; total: number }> {
  const query = new URLSearchParams()
  if (params.severity) query.set('severity', params.severity)
  if (params.min_hit != null && params.min_hit > 0) {
    query.set('min_hit', String(params.min_hit))
  }
  if (params.start_timestamp != null) {
    query.set('start_timestamp', String(params.start_timestamp))
  }
  if (params.end_timestamp != null) {
    query.set('end_timestamp', String(params.end_timestamp))
  }
  if (params.user_id != null && params.user_id !== '') {
    query.set('user_id', String(params.user_id))
  }
  if (params.model_name) query.set('model_name', params.model_name)
  if (params.p != null && params.p > 0) query.set('p', String(params.p))
  if (params.page_size != null) query.set('page_size', String(params.page_size))
  const qs = query.toString()
  const res = await api.get(qs ? `/api/audit/logs?${qs}` : '/api/audit/logs')
  const data = res.data as AuditLogListResponse
  if (!data.success) {
    throw new Error(data.message || 'Failed to load')
  }
  // 后端 LogContent 无 json tag，字段为大写驼峰（RequestId/CreatedAt...）；
  // 映射为前端的 snake_case 契约（AuditLogItem）。
  const rawItems = (data.data?.items ?? []) as unknown as AuditLogRawItem[]
  const items: AuditLogItem[] = rawItems.map((raw) => ({
    request_id: raw.RequestId ?? '',
    user_id: raw.UserId ?? 0,
    model_name: raw.ModelName ?? '',
    fidelity: raw.Fidelity ?? '',
    hit_count: raw.HitCount ?? 0,
    hit_severity: raw.HitSeverity ?? '',
    created_at: raw.CreatedAt ?? 0,
  }))
  return { items, total: data.data?.total ?? 0 }
}

interface AuditLogRawItem {
  RequestId?: string
  UserId?: number
  CreatedAt?: number
  ModelName?: string
  Fidelity?: string
  HitSeverity?: string
  HitCount?: number
}
