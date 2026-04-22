interface UserSummary {
  email: string
  model: string
  request_count: number
  input_tokens: number
  output_tokens: number
  cache_creation_tokens: number
  cache_read_tokens: number
  total_cost_usd: number
}

interface UsageTableProps {
  users: UserSummary[]
  loading: boolean
}

function fmtTokens(n: number): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K'
  return String(n)
}

export default function UsageTable({ users, loading }: UsageTableProps) {
  if (loading) {
    return <p className="text-sm text-slate-500 py-4 text-center">Loading...</p>
  }

  if (!users || users.length === 0) {
    return <p className="text-sm text-slate-500 py-4 text-center">No usage data for this period</p>
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm text-left">
        <thead>
          <tr className="text-xs text-slate-400 border-b border-slate-700">
            <th className="py-2 pr-3 font-medium">Email</th>
            <th className="py-2 px-3 font-medium">Model</th>
            <th className="py-2 px-3 font-medium text-right">Requests</th>
            <th className="py-2 px-3 font-medium text-right">Input</th>
            <th className="py-2 px-3 font-medium text-right">Output</th>
            <th className="py-2 px-3 font-medium text-right">Cache Write</th>
            <th className="py-2 px-3 font-medium text-right">Cache Read</th>
            <th className="py-2 pl-3 font-medium text-right">Cost</th>
          </tr>
        </thead>
        <tbody>
          {users.map((u, i) => (
            <tr key={`${u.email}-${u.model}-${i}`} className="border-b border-slate-700/50 hover:bg-slate-700/30">
              <td className="py-2 pr-3 text-slate-200 truncate max-w-48">{u.email}</td>
              <td className="py-2 px-3 text-slate-400 text-xs font-mono">{u.model}</td>
              <td className="py-2 px-3 text-right text-slate-300 font-mono">{u.request_count}</td>
              <td className="py-2 px-3 text-right text-slate-300 font-mono">{fmtTokens(u.input_tokens)}</td>
              <td className="py-2 px-3 text-right text-slate-300 font-mono">{fmtTokens(u.output_tokens)}</td>
              <td className="py-2 px-3 text-right text-slate-300 font-mono">{fmtTokens(u.cache_creation_tokens)}</td>
              <td className="py-2 px-3 text-right text-slate-300 font-mono">{fmtTokens(u.cache_read_tokens)}</td>
              <td className="py-2 pl-3 text-right text-emerald-400 font-mono">${u.total_cost_usd.toFixed(2)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
