interface QuotaBarProps {
  label: string
  utilization: number
  resetTime: string
  status: string
}

function formatReset(iso: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (isNaN(d.getTime())) return ''
  const now = new Date()
  const diffMs = d.getTime() - now.getTime()
  const diffH = Math.floor(diffMs / 3600000)
  const diffM = Math.floor((diffMs % 3600000) / 60000)

  const timeStr = d.toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit', timeZone: 'UTC' }) + ' (UTC)'

  if (diffH < 24) {
    return `Resets in ${diffH}h ${diffM}m — ${timeStr}`
  }
  const dateStr = d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', timeZone: 'UTC' })
  return `Resets ${dateStr}, ${timeStr}`
}

export default function QuotaBar({ label, utilization, resetTime, status }: QuotaBarProps) {
  const pct = Math.round(utilization)
  const blocked = status === 'blocked'
  const danger = pct >= 80 || blocked

  return (
    <div>
      <div className="flex justify-between items-baseline mb-1.5">
        <span className="text-xs text-app-text">{label}</span>
        <span className={`text-[11px] font-mono tabular-nums ${danger ? 'text-chart-rose' : 'text-app-mute'}`}>
          {blocked ? 'BLOCKED' : `${pct}% used`}
        </span>
      </div>
      <div className="h-1.5 bg-app-surface-2 rounded-full overflow-hidden">
        <div
          className={`h-full rounded-full transition-all duration-500 ${danger ? 'bg-chart-rose' : pct > 0 ? 'bg-app-accent' : 'bg-app-dim'}`}
          style={{ width: `${Math.max(pct === 0 ? 0 : pct, 1.5)}%` }}
        />
      </div>
      {resetTime && (
        <p className="text-[10px] text-app-dim mt-1.5 font-mono">{formatReset(resetTime)}</p>
      )}
    </div>
  )
}
