import { useState } from 'react'

export type Preset = 'today' | 'this_week' | 'last_week' | 'this_month' | 'custom'

interface DateRangePickerProps {
  active: Preset
  customSince?: string
  customUntil?: string
  onChange: (preset: Preset, customSince?: string, customUntil?: string) => void
}

const presets: { key: Preset; label: string }[] = [
  { key: 'today', label: 'Today' },
  { key: 'this_week', label: 'This Week' },
  { key: 'last_week', label: 'Last Week' },
  { key: 'this_month', label: 'This Month' },
  { key: 'custom', label: 'Custom' },
]

export default function DateRangePicker({ active, customSince, customUntil, onChange }: DateRangePickerProps) {
  const [showCustom, setShowCustom] = useState(false)
  const [since, setSince] = useState(customSince ?? '')
  const [until, setUntil] = useState(customUntil ?? '')

  return (
    <div className="flex items-center gap-1 flex-wrap">
      {presets.map((p) => {
        const isActive = active === p.key
        return (
          <button
            key={p.key}
            onClick={() => {
              if (p.key === 'custom') {
                setShowCustom(!showCustom)
              } else {
                setShowCustom(false)
                onChange(p.key)
              }
            }}
            className={`px-3 py-1.5 text-xs font-medium rounded-full cursor-pointer transition-colors border ${
              isActive
                ? 'bg-app-accent/10 text-app-accent border-app-accent/30'
                : 'bg-transparent text-app-mute border-transparent hover:text-app-text'
            }`}
          >
            {p.label}
          </button>
        )
      })}
      {showCustom && (
        <div className="flex items-center gap-1.5 ml-1">
          <input
            type="date"
            value={since}
            onChange={(e) => setSince(e.target.value)}
            className="bg-app-surface-2 text-app-text text-xs rounded-md px-2 py-1 border border-app-line focus:outline-none focus:border-app-accent/40"
          />
          <span className="text-xs text-app-dim">to</span>
          <input
            type="date"
            value={until}
            onChange={(e) => setUntil(e.target.value)}
            className="bg-app-surface-2 text-app-text text-xs rounded-md px-2 py-1 border border-app-line focus:outline-none focus:border-app-accent/40"
          />
          <button
            onClick={() => {
              if (since) {
                onChange('custom', since, until || undefined)
                setShowCustom(false)
              }
            }}
            className="px-3 py-1 text-xs rounded-md bg-app-accent/10 text-app-accent border border-app-accent/30 hover:bg-app-accent/20 cursor-pointer transition-colors"
          >
            Apply
          </button>
        </div>
      )}
    </div>
  )
}

export function buildUsageQuery(preset: Preset, customSince?: string, customUntil?: string): string {
  if (preset === 'custom') {
    const params = new URLSearchParams()
    if (customSince) params.set('since', customSince)
    if (customUntil) params.set('until', customUntil)
    return params.toString()
  }
  return `range=${preset}`
}
