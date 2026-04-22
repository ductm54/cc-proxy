import { useMemo, useState } from 'react'
import {
  BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer, Cell,
  PieChart, Pie, Treemap,
} from 'recharts'

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

interface UsageDashboardProps {
  users: UserSummary[]
  loading: boolean
}

const PALETTE = [
  '#06b6d4', '#f59e0b', '#8b5cf6', '#f43f5e',
  '#10b981', '#0ea5e9', '#f97316', '#ec4899',
  '#a78bfa', '#34d399', '#fbbf24', '#fb7185',
]

function fmtTokens(n: number): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K'
  return String(n)
}

function shortEmail(email: string): string {
  const at = email.indexOf('@')
  if (at === -1) return email
  const local = email.substring(0, at)
  return local.length > 12 ? local.substring(0, 12) + '…' : local
}

function shortModel(model: string): string {
  return model
    .replace('claude-', '')
    .replace('-20250', '-')
    .replace('-latest', '')
}

type View = 'charts' | 'table'

export default function UsageDashboard({ users, loading }: UsageDashboardProps) {
  const [view, setView] = useState<View>('charts')

  const stats = useMemo(() => {
    if (!users.length) return null

    const totalCost = users.reduce((s, u) => s + u.total_cost_usd, 0)
    const totalRequests = users.reduce((s, u) => s + u.request_count, 0)
    const totalInput = users.reduce((s, u) => s + u.input_tokens, 0)
    const totalOutput = users.reduce((s, u) => s + u.output_tokens, 0)

    const uniqueEmails = new Set(users.map(u => u.email))
    const uniqueModels = new Set(users.map(u => u.model))

    const costByUser = new Map<string, number>()
    const requestsByUser = new Map<string, number>()
    const costByModel = new Map<string, number>()
    const requestsByModel = new Map<string, number>()
    const tokensByUser = new Map<string, { input: number; output: number; cacheWrite: number; cacheRead: number }>()

    for (const u of users) {
      costByUser.set(u.email, (costByUser.get(u.email) ?? 0) + u.total_cost_usd)
      requestsByUser.set(u.email, (requestsByUser.get(u.email) ?? 0) + u.request_count)
      costByModel.set(u.model, (costByModel.get(u.model) ?? 0) + u.total_cost_usd)
      requestsByModel.set(u.model, (requestsByModel.get(u.model) ?? 0) + u.request_count)

      const prev = tokensByUser.get(u.email) ?? { input: 0, output: 0, cacheWrite: 0, cacheRead: 0 }
      tokensByUser.set(u.email, {
        input: prev.input + u.input_tokens,
        output: prev.output + u.output_tokens,
        cacheWrite: prev.cacheWrite + u.cache_creation_tokens,
        cacheRead: prev.cacheRead + u.cache_read_tokens,
      })
    }

    const costByUserData = [...costByUser.entries()]
      .sort((a, b) => b[1] - a[1])
      .map(([email, cost]) => ({ email, shortEmail: shortEmail(email), cost: +cost.toFixed(2) }))

    const requestsByUserData = [...requestsByUser.entries()]
      .sort((a, b) => b[1] - a[1])
      .map(([email, count]) => ({ email, shortEmail: shortEmail(email), count }))

    const modelData = [...costByModel.entries()]
      .sort((a, b) => b[1] - a[1])
      .map(([model, cost]) => ({
        model,
        shortModel: shortModel(model),
        cost: +cost.toFixed(2),
        requests: requestsByModel.get(model) ?? 0,
      }))

    const tokenData = [...tokensByUser.entries()]
      .sort((a, b) => {
        const aTotal = a[1].input + a[1].output + a[1].cacheWrite + a[1].cacheRead
        const bTotal = b[1].input + b[1].output + b[1].cacheWrite + b[1].cacheRead
        return bTotal - aTotal
      })
      .map(([email, t]) => ({
        email,
        shortEmail: shortEmail(email),
        ...t,
      }))

    return {
      totalCost, totalRequests, totalInput, totalOutput,
      activeUsers: uniqueEmails.size,
      modelCount: uniqueModels.size,
      costByUserData, requestsByUserData, modelData, tokenData,
    }
  }, [users])

  if (loading) {
    return (
      <div className="flex items-center justify-center py-16">
        <div className="flex gap-1.5">
          {[0, 1, 2, 3, 4].map(i => (
            <div
              key={i}
              className="w-1.5 h-8 bg-chart-cyan/40 rounded-full"
              style={{
                animation: 'pulse-bar 1s ease-in-out infinite',
                animationDelay: `${i * 0.12}s`,
              }}
            />
          ))}
        </div>
        <style>{`
          @keyframes pulse-bar {
            0%, 100% { transform: scaleY(0.4); opacity: 0.4; }
            50% { transform: scaleY(1); opacity: 1; }
          }
        `}</style>
      </div>
    )
  }

  if (!users.length || !stats) {
    return (
      <div className="text-center py-16">
        <div className="text-slate-600 text-4xl mb-3 font-mono">∅</div>
        <p className="text-sm text-slate-500">No usage data for this period</p>
      </div>
    )
  }

  return (
    <div className="space-y-5">
      {/* View Toggle */}
      <div className="flex gap-1 bg-slate-900/60 rounded-lg p-0.5 w-fit">
        <button
          onClick={() => setView('charts')}
          className={`px-3.5 py-1.5 text-xs font-medium rounded-md transition-all cursor-pointer ${
            view === 'charts'
              ? 'bg-slate-700 text-slate-100 shadow-sm'
              : 'text-slate-500 hover:text-slate-300'
          }`}
        >
          Charts
        </button>
        <button
          onClick={() => setView('table')}
          className={`px-3.5 py-1.5 text-xs font-medium rounded-md transition-all cursor-pointer ${
            view === 'table'
              ? 'bg-slate-700 text-slate-100 shadow-sm'
              : 'text-slate-500 hover:text-slate-300'
          }`}
        >
          Table
        </button>
      </div>

      {/* Stat Cards */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
        <StatCard label="Total Cost" value={`$${stats.totalCost.toFixed(2)}`} accent="text-emerald-400" />
        <StatCard label="Requests" value={fmtTokens(stats.totalRequests)} accent="text-chart-cyan" />
        <StatCard label="Active Users" value={String(stats.activeUsers)} accent="text-chart-amber" />
        <StatCard label="Models Used" value={String(stats.modelCount)} accent="text-chart-violet" />
      </div>

      {view === 'charts' ? (
        <div className="space-y-5">
          {/* Row 1: Cost by User + Model Distribution */}
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
            <ChartPanel title="Cost by User" subtitle="Total spend per user">
              <ResponsiveContainer width="100%" height={Math.max(200, stats.costByUserData.length * 36 + 20)}>
                <BarChart
                  data={stats.costByUserData}
                  layout="vertical"
                  margin={{ top: 4, right: 40, bottom: 4, left: 4 }}
                  barCategoryGap="20%"
                >
                  <XAxis type="number" hide />
                  <YAxis
                    type="category"
                    dataKey="shortEmail"
                    width={100}
                    tick={{ fill: '#94a3b8', fontSize: 11, fontFamily: 'JetBrains Mono' }}
                    axisLine={false}
                    tickLine={false}
                  />
                  <Tooltip content={<CostTooltip />} cursor={{ fill: 'rgba(148,163,184,0.06)' }} />
                  <Bar dataKey="cost" radius={[0, 4, 4, 0]} maxBarSize={22}>
                    {stats.costByUserData.map((_, i) => (
                      <Cell key={i} fill={PALETTE[i % PALETTE.length]} fillOpacity={0.85} />
                    ))}
                  </Bar>
                </BarChart>
              </ResponsiveContainer>
            </ChartPanel>

            <ChartPanel title="Model Popularity" subtitle="Cost distribution across models">
              <div className="flex items-center justify-center h-full min-h-[200px]">
                <ResponsiveContainer width="100%" height={260}>
                  <PieChart>
                    <Pie
                      data={stats.modelData}
                      dataKey="cost"
                      nameKey="shortModel"
                      cx="50%"
                      cy="50%"
                      innerRadius={55}
                      outerRadius={95}
                      paddingAngle={2}
                      strokeWidth={0}
                    >
                      {stats.modelData.map((_, i) => (
                        <Cell key={i} fill={PALETTE[i % PALETTE.length]} fillOpacity={0.9} />
                      ))}
                    </Pie>
                    <Tooltip content={<ModelTooltip />} />
                  </PieChart>
                </ResponsiveContainer>
                <div className="flex flex-col gap-2 ml-2 shrink-0">
                  {stats.modelData.map((m, i) => (
                    <div key={m.model} className="flex items-center gap-2 text-xs">
                      <span
                        className="w-2.5 h-2.5 rounded-sm shrink-0"
                        style={{ backgroundColor: PALETTE[i % PALETTE.length] }}
                      />
                      <span className="text-slate-400 font-mono truncate max-w-[120px]">{m.shortModel}</span>
                    </div>
                  ))}
                </div>
              </div>
            </ChartPanel>
          </div>

          {/* Row 2: Most Active Users + Token Heatmap */}
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
            <ChartPanel title="Most Active" subtitle="Requests per user">
              <ResponsiveContainer width="100%" height={Math.max(200, stats.requestsByUserData.length * 36 + 20)}>
                <BarChart
                  data={stats.requestsByUserData}
                  layout="vertical"
                  margin={{ top: 4, right: 40, bottom: 4, left: 4 }}
                  barCategoryGap="20%"
                >
                  <XAxis type="number" hide />
                  <YAxis
                    type="category"
                    dataKey="shortEmail"
                    width={100}
                    tick={{ fill: '#94a3b8', fontSize: 11, fontFamily: 'JetBrains Mono' }}
                    axisLine={false}
                    tickLine={false}
                  />
                  <Tooltip content={<RequestTooltip />} cursor={{ fill: 'rgba(148,163,184,0.06)' }} />
                  <Bar dataKey="count" radius={[0, 4, 4, 0]} maxBarSize={22}>
                    {stats.requestsByUserData.map((_, i) => (
                      <Cell key={i} fill={PALETTE[i % PALETTE.length]} fillOpacity={0.85} />
                    ))}
                  </Bar>
                </BarChart>
              </ResponsiveContainer>
            </ChartPanel>

            <ChartPanel title="Token Breakdown" subtitle="Tokens by type per user">
              {stats.tokenData.length <= 8 ? (
                <ResponsiveContainer width="100%" height={Math.max(200, stats.tokenData.length * 36 + 20)}>
                  <BarChart
                    data={stats.tokenData}
                    layout="vertical"
                    margin={{ top: 4, right: 12, bottom: 4, left: 4 }}
                    barCategoryGap="20%"
                  >
                    <XAxis type="number" hide />
                    <YAxis
                      type="category"
                      dataKey="shortEmail"
                      width={100}
                      tick={{ fill: '#94a3b8', fontSize: 11, fontFamily: 'JetBrains Mono' }}
                      axisLine={false}
                      tickLine={false}
                    />
                    <Tooltip content={<TokenTooltip />} cursor={{ fill: 'rgba(148,163,184,0.06)' }} />
                    <Bar dataKey="input" stackId="t" fill="#06b6d4" fillOpacity={0.8} name="Input" maxBarSize={22} />
                    <Bar dataKey="output" stackId="t" fill="#f59e0b" fillOpacity={0.8} name="Output" maxBarSize={22} />
                    <Bar dataKey="cacheWrite" stackId="t" fill="#8b5cf6" fillOpacity={0.8} name="Cache Write" maxBarSize={22} />
                    <Bar dataKey="cacheRead" stackId="t" fill="#10b981" fillOpacity={0.8} name="Cache Read" radius={[0, 4, 4, 0]} maxBarSize={22} />
                  </BarChart>
                </ResponsiveContainer>
              ) : (
                <ResponsiveContainer width="100%" height={280}>
                  <Treemap
                    data={stats.tokenData.map((t, i) => ({
                      name: t.shortEmail,
                      size: t.input + t.output + t.cacheWrite + t.cacheRead,
                      fill: PALETTE[i % PALETTE.length],
                    }))}
                    dataKey="size"
                    aspectRatio={4 / 3}
                    stroke="rgb(30 41 59)"
                    content={<TreemapCell x={0} y={0} width={0} height={0} />}
                  >
                    <Tooltip content={<TreemapTooltip />} />
                  </Treemap>
                </ResponsiveContainer>
              )}
              <div className="flex gap-4 mt-3 justify-center">
                {[
                  { label: 'Input', color: '#06b6d4' },
                  { label: 'Output', color: '#f59e0b' },
                  { label: 'Cache Write', color: '#8b5cf6' },
                  { label: 'Cache Read', color: '#10b981' },
                ].map(l => (
                  <div key={l.label} className="flex items-center gap-1.5 text-[10px] text-slate-500">
                    <span className="w-2 h-2 rounded-sm" style={{ backgroundColor: l.color }} />
                    {l.label}
                  </div>
                ))}
              </div>
            </ChartPanel>
          </div>
        </div>
      ) : (
        /* Table View */
        <div className="overflow-x-auto">
          <table className="w-full text-sm text-left">
            <thead>
              <tr className="text-[11px] text-slate-500 uppercase tracking-wider border-b border-slate-700/60">
                <th className="py-2.5 pr-3 font-medium">Email</th>
                <th className="py-2.5 px-3 font-medium">Model</th>
                <th className="py-2.5 px-3 font-medium text-right">Req</th>
                <th className="py-2.5 px-3 font-medium text-right">Input</th>
                <th className="py-2.5 px-3 font-medium text-right">Output</th>
                <th className="py-2.5 px-3 font-medium text-right">Cache W</th>
                <th className="py-2.5 px-3 font-medium text-right">Cache R</th>
                <th className="py-2.5 pl-3 font-medium text-right">Cost</th>
              </tr>
            </thead>
            <tbody>
              {users.map((u, i) => (
                <tr key={`${u.email}-${u.model}-${i}`} className="border-b border-slate-700/30 hover:bg-slate-700/20 transition-colors">
                  <td className="py-2 pr-3 text-slate-300 truncate max-w-48">{u.email}</td>
                  <td className="py-2 px-3 text-slate-500 text-xs font-mono">{shortModel(u.model)}</td>
                  <td className="py-2 px-3 text-right text-slate-400 font-mono text-xs">{u.request_count}</td>
                  <td className="py-2 px-3 text-right text-slate-400 font-mono text-xs">{fmtTokens(u.input_tokens)}</td>
                  <td className="py-2 px-3 text-right text-slate-400 font-mono text-xs">{fmtTokens(u.output_tokens)}</td>
                  <td className="py-2 px-3 text-right text-slate-400 font-mono text-xs">{fmtTokens(u.cache_creation_tokens)}</td>
                  <td className="py-2 px-3 text-right text-slate-400 font-mono text-xs">{fmtTokens(u.cache_read_tokens)}</td>
                  <td className="py-2 pl-3 text-right text-emerald-400 font-mono text-xs font-medium">${u.total_cost_usd.toFixed(2)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

/* ---------- Sub-components ---------- */

function StatCard({ label, value, accent }: { label: string; value: string; accent: string }) {
  return (
    <div className="bg-slate-900/50 border border-slate-700/40 rounded-lg px-4 py-3 group hover:border-slate-600/60 transition-colors">
      <p className="text-[10px] uppercase tracking-widest text-slate-500 mb-1 font-medium">{label}</p>
      <p className={`text-xl font-mono font-semibold ${accent} tracking-tight`}>{value}</p>
    </div>
  )
}

function ChartPanel({ title, subtitle, children }: { title: string; subtitle: string; children: React.ReactNode }) {
  return (
    <div className="bg-slate-900/40 border border-slate-700/30 rounded-lg p-4">
      <div className="mb-3">
        <h3 className="text-sm font-semibold text-slate-200">{title}</h3>
        <p className="text-[10px] text-slate-500 mt-0.5">{subtitle}</p>
      </div>
      {children}
    </div>
  )
}

/* ---------- Tooltips ---------- */

function CostTooltip({ active, payload }: { active?: boolean; payload?: Array<{ payload: { email: string; cost: number } }> }) {
  if (!active || !payload?.[0]) return null
  const d = payload[0].payload
  return (
    <div className="bg-slate-800 border border-slate-600/50 rounded-lg px-3 py-2 shadow-xl text-xs">
      <p className="text-slate-300 mb-1">{d.email}</p>
      <p className="text-emerald-400 font-mono font-medium">${d.cost.toFixed(2)}</p>
    </div>
  )
}

function RequestTooltip({ active, payload }: { active?: boolean; payload?: Array<{ payload: { email: string; count: number } }> }) {
  if (!active || !payload?.[0]) return null
  const d = payload[0].payload
  return (
    <div className="bg-slate-800 border border-slate-600/50 rounded-lg px-3 py-2 shadow-xl text-xs">
      <p className="text-slate-300 mb-1">{d.email}</p>
      <p className="text-chart-cyan font-mono font-medium">{d.count.toLocaleString()} requests</p>
    </div>
  )
}

function ModelTooltip({ active, payload }: { active?: boolean; payload?: Array<{ payload: { model: string; cost: number; requests: number } }> }) {
  if (!active || !payload?.[0]) return null
  const d = payload[0].payload
  return (
    <div className="bg-slate-800 border border-slate-600/50 rounded-lg px-3 py-2 shadow-xl text-xs">
      <p className="text-slate-300 font-mono mb-1">{d.model}</p>
      <p className="text-emerald-400 font-mono">${d.cost.toFixed(2)}</p>
      <p className="text-slate-400">{d.requests.toLocaleString()} requests</p>
    </div>
  )
}

function TokenTooltip({ active, payload }: { active?: boolean; payload?: Array<{ payload: { email: string; input: number; output: number; cacheWrite: number; cacheRead: number } }> }) {
  if (!active || !payload?.[0]) return null
  const d = payload[0].payload
  return (
    <div className="bg-slate-800 border border-slate-600/50 rounded-lg px-3 py-2 shadow-xl text-xs space-y-0.5">
      <p className="text-slate-300 mb-1">{d.email}</p>
      <p><span className="text-slate-500">Input:</span> <span className="text-chart-cyan font-mono">{fmtTokens(d.input)}</span></p>
      <p><span className="text-slate-500">Output:</span> <span className="text-chart-amber font-mono">{fmtTokens(d.output)}</span></p>
      <p><span className="text-slate-500">Cache W:</span> <span className="text-chart-violet font-mono">{fmtTokens(d.cacheWrite)}</span></p>
      <p><span className="text-slate-500">Cache R:</span> <span className="text-chart-emerald font-mono">{fmtTokens(d.cacheRead)}</span></p>
    </div>
  )
}

function TreemapTooltip({ active, payload }: { active?: boolean; payload?: Array<{ payload: { name: string; size: number } }> }) {
  if (!active || !payload?.[0]) return null
  const d = payload[0].payload
  return (
    <div className="bg-slate-800 border border-slate-600/50 rounded-lg px-3 py-2 shadow-xl text-xs">
      <p className="text-slate-300 mb-1">{d.name}</p>
      <p className="text-chart-cyan font-mono">{fmtTokens(d.size)} tokens</p>
    </div>
  )
}

function TreemapCell(props: { x: number; y: number; width: number; height: number; name?: string; fill?: string }) {
  const { x, y, width, height, name, fill } = props
  if (width < 4 || height < 4) return null
  return (
    <g>
      <rect x={x} y={y} width={width} height={height} rx={4} fill={fill} fillOpacity={0.75} stroke="rgb(30 41 59)" strokeWidth={2} />
      {width > 40 && height > 24 && (
        <text x={x + width / 2} y={y + height / 2} textAnchor="middle" dominantBaseline="central" fill="#e2e8f0" fontSize={11} fontFamily="JetBrains Mono">
          {name}
        </text>
      )}
    </g>
  )
}
