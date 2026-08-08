/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

export type WatchlistKind = 'domain' | 'keyword' | 'regex'
export type WatchlistSeverity = 'low' | 'medium' | 'high'

export interface AuditWatchlistRule {
  id: number
  kind: WatchlistKind
  pattern: string
  severity: WatchlistSeverity
  enabled: boolean
  note: string
  created_at: number
  updated_at: number
}

export interface AuditWatchlistRuleInput {
  kind: WatchlistKind
  pattern: string
  severity: WatchlistSeverity
  enabled: boolean
  note?: string
}

export interface RescanStatus {
  processed: number
  total: number
  status: '' | 'running' | 'done' | 'error' | 'no_op'
  wl_version: number
  message?: string
}

// ============================================================================
// Template Package Types
// ============================================================================

export interface AuditTemplate {
  id: string
  name: string
  description: string
  rule_count: number
  applied_count: number
  enabled_count: number
  status: 'unapplied' | 'applied' | 'disabled'
}

export interface ApplyTemplateResult {
  applied: number
  skipped: number
  regex_disabled: number
  message?: string
}

export interface EnableTemplateResult {
  enabled: number
  regex_skipped: number
  message?: string
}

export interface DisableTemplateResult {
  disabled: number
}

export interface RemoveTemplateResult {
  removed: number
}

// ============================================================================
// Audit Log List Types
// ============================================================================

export interface AuditLogItem {
  request_id: string
  user_id: number
  model_name: string
  fidelity: string
  hit_count: number
  hit_severity: string
  created_at: number
}

export interface AuditLogListParams {
  severity?: string
  min_hit?: number
  start_timestamp?: number
  end_timestamp?: number
  user_id?: number | string
  model_name?: string
  p?: number
  page_size?: number
}

export interface AuditLogListResponse {
  success: boolean
  message?: string
  data?: {
    items: AuditLogItem[]
    total: number
    page: number
    page_size: number
  }
}
