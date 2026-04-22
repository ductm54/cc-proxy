import { useState } from 'react'

export default function CopyBlock({ id, label, children }: { id: string; label: string; children: string }) {
  const [copied, setCopied] = useState(false)

  const handleCopy = () => {
    navigator.clipboard.writeText(children).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
  }

  return (
    <div className="mb-4">
      <div className="text-xs text-slate-400 mb-1.5 font-semibold">{label}</div>
      <div className="relative">
        <pre id={id} className="bg-slate-900 border border-slate-700 rounded-lg p-3.5 font-mono text-sm leading-relaxed overflow-x-auto text-slate-200 whitespace-pre-wrap break-all">
          {children}
        </pre>
        <button
          onClick={handleCopy}
          className={`absolute top-2 right-2 text-xs px-2.5 py-1 rounded cursor-pointer transition-colors ${
            copied
              ? 'bg-emerald-900 text-emerald-400'
              : 'bg-slate-700 text-slate-400 hover:bg-slate-600 hover:text-slate-200'
          }`}
        >
          {copied ? 'Copied!' : 'Copy'}
        </button>
      </div>
    </div>
  )
}
