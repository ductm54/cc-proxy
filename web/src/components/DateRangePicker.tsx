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
    <div className="flex items-center gap-1.5 flex-wrap">
      {presets.map((p) => (
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
          className={`px-3 py-1 text-xs rounded-md cursor-pointer transition-colors ${
            active === p.key
              ? 'bg-slate-600 text-slate-100'
              : 'bg-slate-700/50 text-slate-400 hover:bg-slate-700 hover:text-slate-200'
          }`}
        >
          {p.label}
        </button>
      ))}
      {showCustom && (
        <div className="flex items-center gap-1.5 ml-1">
          <input
            type="date"
            value={since}
            onChange={(e) => setSince(e.target.value)}
            className="bg-slate-700 text-slate-200 text-xs rounded px-2 py-1 border border-slate-600 focus:outline-none focus:border-slate-400"
          />
          <span className="text-xs text-slate-500">to</span>
          <input
            type="date"
            value={until}
            onChange={(e) => setUntil(e.target.value)}
            className="bg-slate-700 text-slate-200 text-xs rounded px-2 py-1 border border-slate-600 focus:outline-none focus:border-slate-400"
          />
          <button
            onClick={() => {
              if (since) {
                onChange('custom', since, until || undefined)
                setShowCustom(false)
              }
            }}
            className="px-2 py-1 text-xs rounded-md bg-emerald-600 text-white hover:bg-emerald-500 cursor-pointer transition-colors"
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
