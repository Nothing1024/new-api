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
