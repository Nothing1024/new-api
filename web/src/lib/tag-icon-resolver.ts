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
import * as LobeIcons from '@lobehub/icons'

// Matches the CUSTOM_ICONS map in lobe-icon.tsx; keep in sync when new entries are added.
const CUSTOM_ICON_KEYS: Record<string, true> = { Sub2API: true }

/**
 * Resolve a channel tag to a lobehub icon base key if the tag matches a
 * known provider icon (case-insensitive).
 *
 * Returns the bare key (e.g. "Kiro"), NOT "Kiro.Color" — callers append
 * the variant suffix themselves, consistent with getChannelTypeIcon().
 *
 * @returns Icon base key (e.g. "Kiro") or null when no match.
 */
export function resolveTagIcon(tag: string | null | undefined): string | null {
  if (!tag) return null
  const normalized = tag.trim()
  if (!normalized) return null

  // Try common capitalisation patterns:
  // 1. Exact casing  (user typed "XAI" or "DeepSeek")
  // 2. PascalCase    ("kiro" → "Kiro")
  // 3. ALL-CAPS      ("xai"  → "XAI")
  const candidates = [
    normalized,
    normalized.charAt(0).toUpperCase() + normalized.slice(1).toLowerCase(),
    normalized.toUpperCase(),
  ]

  for (const key of candidates) {
    if (CUSTOM_ICON_KEYS[key]) return key
    if ((LobeIcons as Record<string, unknown>)[key]) return key
  }
  return null
}
