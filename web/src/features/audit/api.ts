/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { api } from '@/lib/api'

import type {
  AuditWatchlistRule,
  AuditWatchlistRuleInput,
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
