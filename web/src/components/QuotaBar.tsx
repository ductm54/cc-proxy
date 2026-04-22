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

function barColor(pct: number): string {
  if (pct >= 80) return 'bg-red-500'
  if (pct >= 60) return 'bg-yellow-500'
  return 'bg-emerald-500'
}

export default function QuotaBar({ label, utilization, resetTime, status }: QuotaBarProps) {
  const pct = Math.round(utilization)
  const blocked = status === 'blocked'

  return (
    <div className="mb-4">
      <div className="flex justify-between items-baseline mb-1.5">
        <span className="text-sm text-slate-300">{label}</span>
        <span className={`text-xs font-mono ${blocked ? 'text-red-400' : 'text-slate-400'}`}>
          {blocked ? 'BLOCKED' : `${pct}% used`}
        </span>
      </div>
      <div className="w-full bg-slate-700 rounded-full h-2.5">
        <div
          className={`h-2.5 rounded-full transition-all duration-500 ${barColor(pct)}`}
          style={{ width: `${Math.min(pct, 100)}%` }}
        />
      </div>
      {resetTime && (
        <p className="text-xs text-slate-500 mt-1">{formatReset(resetTime)}</p>
      )}
    </div>
  )
}
