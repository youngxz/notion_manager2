export function avatarColor(_name: string): string {
  return 'rgba(255, 255, 255, 0.08)'
}

export function avatarLetter(name: string): string {
  return (name || '?')[0].toUpperCase()
}

export function fmt(n: number): string {
  return (n || 0).toLocaleString()
}

export type QuotaStatus = 'ok' | 'low' | 'exhausted'

export function getQuotaStatus(exhausted: boolean, permanent: boolean, usage?: number, limit?: number): QuotaStatus {
  if (permanent || exhausted) return 'exhausted'
  if (limit && usage && usage / limit >= 0.9) return 'low'
  return 'ok'
}

export function getQuotaStatusByUsage(usage?: number, limit?: number): QuotaStatus {
  if (limit && (usage || 0) >= limit) return 'exhausted'
  if (limit && usage && usage / limit >= 0.9) return 'low'
  return 'ok'
}

export function getQuotaPct(usage?: number, limit?: number): number {
  if (!limit) return 0
  return Math.min(((usage || 0) / limit) * 100, 100)
}

export function formatCheckedAt(iso?: string): string {
  if (!iso) return '—'
  try {
    return new Date(iso).toLocaleString('zh-CN', {
      month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit',
    })
  } catch {
    return '—'
  }
}

export function formatTimestampMs(ms?: number): string {
  if (!ms) return '—'
  try {
    return new Date(ms).toLocaleString('zh-CN', {
      month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit',
    })
  } catch {
    return '—'
  }
}
