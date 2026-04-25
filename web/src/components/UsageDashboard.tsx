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

const CHART_COLORS = ['#5EEAD4', '#F2B45A', '#A78BFA', '#7DD3FC', '#10b981', '#f43f5e', '#f97316', '#ec4899']
const TOKEN_COLORS = {
  input: '#3FB6C5',
  output: '#F2B45A',
  cacheWrite: '#A78BFA',
  cacheRead: '#5EEAD4',
}

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
              className="w-1.5 h-8 bg-app-accent/40 rounded-full"
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
        <div className="text-app-dim text-4xl mb-3 font-mono">∅</div>
        <p className="text-sm text-app-mute">No usage data for this period</p>
      </div>
    )
  }

  return (
    <div className="space-y-4">
      {/* Charts/Table segmented toggle */}
      <Segmented
        options={['Charts', 'Table']}
        value={view === 'charts' ? 'Charts' : 'Table'}
        onChange={v => setView(v === 'Charts' ? 'charts' : 'table')}
      />

      {/* KPIs */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3.5">
        <Kpi label="Total Cost"    value={`$${stats.totalCost.toFixed(2)}`} accent="text-app-accent" />
        <Kpi label="Requests"      value={fmtTokens(stats.totalRequests)}   accent="text-chart-amber" />
        <Kpi label="Active Users"  value={String(stats.activeUsers)}        accent="text-chart-violet" />
        <Kpi label="Models Used"   value={String(stats.modelCount)}         accent="text-chart-sky" />
      </div>

      {view === 'charts' ? (
        <div className="space-y-3.5">
          {/* Row 1: Cost by User + Model Distribution */}
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-3.5">
            <ChartPanel title="Cost by user" subtitle="Total spend per user">
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
                    tick={{ fill: '#8A93A2', fontSize: 11, fontFamily: 'JetBrains Mono' }}
                    axisLine={false}
                    tickLine={false}
                  />
                  <Tooltip content={<CostTooltip />} cursor={{ fill: 'rgba(94,234,212,0.05)' }} />
                  <Bar dataKey="cost" radius={[0, 4, 4, 0]} maxBarSize={22}>
                    {stats.costByUserData.map((_, i) => (
                      <Cell key={i} fill={CHART_COLORS[i % CHART_COLORS.length]} fillOpacity={0.85} />
                    ))}
                  </Bar>
                </BarChart>
              </ResponsiveContainer>
            </ChartPanel>

            <ChartPanel title="Model popularity" subtitle="Cost distribution across models">
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
                        <Cell key={i} fill={CHART_COLORS[i % CHART_COLORS.length]} fillOpacity={0.9} />
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
                        style={{ backgroundColor: CHART_COLORS[i % CHART_COLORS.length] }}
                      />
                      <span className="text-app-mute font-mono truncate max-w-[120px]">{m.shortModel}</span>
                    </div>
                  ))}
                </div>
              </div>
            </ChartPanel>
          </div>

          {/* Row 2: Most Active + Token Breakdown */}
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-3.5">
            <ChartPanel title="Most active" subtitle="Requests per user">
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
                    tick={{ fill: '#8A93A2', fontSize: 11, fontFamily: 'JetBrains Mono' }}
                    axisLine={false}
                    tickLine={false}
                  />
                  <Tooltip content={<RequestTooltip />} cursor={{ fill: 'rgba(94,234,212,0.05)' }} />
                  <Bar dataKey="count" radius={[0, 4, 4, 0]} maxBarSize={22}>
                    {stats.requestsByUserData.map((_, i) => (
                      <Cell key={i} fill={CHART_COLORS[i % CHART_COLORS.length]} fillOpacity={0.85} />
                    ))}
                  </Bar>
                </BarChart>
              </ResponsiveContainer>
            </ChartPanel>

            <ChartPanel title="Token breakdown" subtitle="Tokens by type per user">
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
                      tick={{ fill: '#8A93A2', fontSize: 11, fontFamily: 'JetBrains Mono' }}
                      axisLine={false}
                      tickLine={false}
                    />
                    <Tooltip content={<TokenTooltip />} cursor={{ fill: 'rgba(94,234,212,0.05)' }} />
                    <Bar dataKey="input" stackId="t" fill={TOKEN_COLORS.input} fillOpacity={0.85} name="Input" maxBarSize={22} />
                    <Bar dataKey="output" stackId="t" fill={TOKEN_COLORS.output} fillOpacity={0.85} name="Output" maxBarSize={22} />
                    <Bar dataKey="cacheWrite" stackId="t" fill={TOKEN_COLORS.cacheWrite} fillOpacity={0.85} name="Cache Write" maxBarSize={22} />
                    <Bar dataKey="cacheRead" stackId="t" fill={TOKEN_COLORS.cacheRead} fillOpacity={0.85} name="Cache Read" radius={[0, 4, 4, 0]} maxBarSize={22} />
                  </BarChart>
                </ResponsiveContainer>
              ) : (
                <ResponsiveContainer width="100%" height={280}>
                  <Treemap
                    data={stats.tokenData.map((t, i) => ({
                      name: t.shortEmail,
                      size: t.input + t.output + t.cacheWrite + t.cacheRead,
                      fill: CHART_COLORS[i % CHART_COLORS.length],
                    }))}
                    dataKey="size"
                    aspectRatio={4 / 3}
                    stroke="#161A21"
                    content={<TreemapCell x={0} y={0} width={0} height={0} />}
                  >
                    <Tooltip content={<TreemapTooltip />} />
                  </Treemap>
                </ResponsiveContainer>
              )}
              <div className="flex gap-4 mt-3 justify-center flex-wrap">
                {[
                  { label: 'Input', color: TOKEN_COLORS.input },
                  { label: 'Output', color: TOKEN_COLORS.output },
                  { label: 'Cache Write', color: TOKEN_COLORS.cacheWrite },
                  { label: 'Cache Read', color: TOKEN_COLORS.cacheRead },
                ].map(l => (
                  <div key={l.label} className="flex items-center gap-1.5 text-[10px] text-app-mute">
                    <span className="w-2 h-2 rounded-full" style={{ backgroundColor: l.color }} />
                    {l.label}
                  </div>
                ))}
              </div>
            </ChartPanel>
          </div>
        </div>
      ) : (
        /* Table View */
        <div className="bg-app-surface-2/40 border border-app-line rounded-xl overflow-x-auto">
          <table className="w-full text-sm text-left">
            <thead>
              <tr className="text-[10px] text-app-dim uppercase tracking-[0.14em] border-b border-app-line">
                <th className="py-3 px-4 font-semibold">Email</th>
                <th className="py-3 px-3 font-semibold">Model</th>
                <th className="py-3 px-3 font-semibold text-right">Req</th>
                <th className="py-3 px-3 font-semibold text-right">Input</th>
                <th className="py-3 px-3 font-semibold text-right">Output</th>
                <th className="py-3 px-3 font-semibold text-right">Cache W</th>
                <th className="py-3 px-3 font-semibold text-right">Cache R</th>
                <th className="py-3 px-4 font-semibold text-right">Cost</th>
              </tr>
            </thead>
            <tbody>
              {users.map((u, i) => (
                <tr key={`${u.email}-${u.model}-${i}`} className="border-b border-app-line/60 last:border-0 hover:bg-app-surface/60 transition-colors">
                  <td className="py-2.5 px-4 text-app-text truncate max-w-48">{u.email}</td>
                  <td className="py-2.5 px-3 text-app-mute text-xs font-mono">{shortModel(u.model)}</td>
                  <td className="py-2.5 px-3 text-right text-app-mute font-mono text-xs tabular-nums">{u.request_count}</td>
                  <td className="py-2.5 px-3 text-right text-app-mute font-mono text-xs tabular-nums">{fmtTokens(u.input_tokens)}</td>
                  <td className="py-2.5 px-3 text-right text-app-mute font-mono text-xs tabular-nums">{fmtTokens(u.output_tokens)}</td>
                  <td className="py-2.5 px-3 text-right text-app-mute font-mono text-xs tabular-nums">{fmtTokens(u.cache_creation_tokens)}</td>
                  <td className="py-2.5 px-3 text-right text-app-mute font-mono text-xs tabular-nums">{fmtTokens(u.cache_read_tokens)}</td>
                  <td className="py-2.5 px-4 text-right text-app-accent font-mono text-xs font-medium tabular-nums">${u.total_cost_usd.toFixed(2)}</td>
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

function Segmented({ options, value, onChange }: { options: string[]; value: string; onChange: (v: string) => void }) {
  return (
    <div className="inline-flex bg-app-surface-2 border border-app-line rounded-lg p-0.5">
      {options.map(o => {
        const isActive = value === o
        return (
          <button
            key={o}
            onClick={() => onChange(o)}
            className={`px-3.5 py-1.5 text-xs font-medium rounded-md transition-all cursor-pointer border ${
              isActive
                ? 'bg-app-surface text-app-text border-app-line'
                : 'bg-transparent text-app-mute border-transparent hover:text-app-text'
            }`}
          >
            {o}
          </button>
        )
      })}
    </div>
  )
}

function Kpi({ label, value, accent }: { label: string; value: string; accent: string }) {
  return (
    <div className="bg-app-surface-2/60 border border-app-line rounded-xl px-4 py-3.5">
      <p className="text-[10px] uppercase tracking-[0.14em] text-app-dim mb-2 font-semibold">{label}</p>
      <p className={`text-[26px] font-mono font-semibold ${accent} tabular-nums`} style={{ letterSpacing: '-0.02em' }}>{value}</p>
    </div>
  )
}

function ChartPanel({ title, subtitle, children }: { title: string; subtitle: string; children: React.ReactNode }) {
  return (
    <div className="bg-app-surface-2/40 border border-app-line rounded-xl p-5">
      <div className="mb-4">
        <h3 className="text-[15px] font-semibold text-app-text">{title}</h3>
        <p className="text-xs text-app-mute mt-0.5">{subtitle}</p>
      </div>
      {children}
    </div>
  )
}

/* ---------- Tooltips ---------- */

function tooltipBase(): string {
  return 'bg-app-surface border border-app-line rounded-lg px-3 py-2 shadow-xl text-xs'
}

function CostTooltip({ active, payload }: { active?: boolean; payload?: Array<{ payload: { email: string; cost: number } }> }) {
  if (!active || !payload?.[0]) return null
  const d = payload[0].payload
  return (
    <div className={tooltipBase()}>
      <p className="text-app-text mb-1">{d.email}</p>
      <p className="text-app-accent font-mono font-medium tabular-nums">${d.cost.toFixed(2)}</p>
    </div>
  )
}

function RequestTooltip({ active, payload }: { active?: boolean; payload?: Array<{ payload: { email: string; count: number } }> }) {
  if (!active || !payload?.[0]) return null
  const d = payload[0].payload
  return (
    <div className={tooltipBase()}>
      <p className="text-app-text mb-1">{d.email}</p>
      <p className="text-chart-amber font-mono font-medium tabular-nums">{d.count.toLocaleString()} requests</p>
    </div>
  )
}

function ModelTooltip({ active, payload }: { active?: boolean; payload?: Array<{ payload: { model: string; cost: number; requests: number } }> }) {
  if (!active || !payload?.[0]) return null
  const d = payload[0].payload
  return (
    <div className={tooltipBase()}>
      <p className="text-app-text font-mono mb-1">{d.model}</p>
      <p className="text-app-accent font-mono tabular-nums">${d.cost.toFixed(2)}</p>
      <p className="text-app-mute tabular-nums">{d.requests.toLocaleString()} requests</p>
    </div>
  )
}

function TokenTooltip({ active, payload }: { active?: boolean; payload?: Array<{ payload: { email: string; input: number; output: number; cacheWrite: number; cacheRead: number } }> }) {
  if (!active || !payload?.[0]) return null
  const d = payload[0].payload
  return (
    <div className={`${tooltipBase()} space-y-0.5`}>
      <p className="text-app-text mb-1">{d.email}</p>
      <p><span className="text-app-dim">Input:</span> <span className="font-mono tabular-nums" style={{ color: TOKEN_COLORS.input }}>{fmtTokens(d.input)}</span></p>
      <p><span className="text-app-dim">Output:</span> <span className="font-mono tabular-nums" style={{ color: TOKEN_COLORS.output }}>{fmtTokens(d.output)}</span></p>
      <p><span className="text-app-dim">Cache W:</span> <span className="font-mono tabular-nums" style={{ color: TOKEN_COLORS.cacheWrite }}>{fmtTokens(d.cacheWrite)}</span></p>
      <p><span className="text-app-dim">Cache R:</span> <span className="font-mono tabular-nums" style={{ color: TOKEN_COLORS.cacheRead }}>{fmtTokens(d.cacheRead)}</span></p>
    </div>
  )
}

function TreemapTooltip({ active, payload }: { active?: boolean; payload?: Array<{ payload: { name: string; size: number } }> }) {
  if (!active || !payload?.[0]) return null
  const d = payload[0].payload
  return (
    <div className={tooltipBase()}>
      <p className="text-app-text mb-1">{d.name}</p>
      <p className="text-app-accent font-mono tabular-nums">{fmtTokens(d.size)} tokens</p>
    </div>
  )
}

function TreemapCell(props: { x: number; y: number; width: number; height: number; name?: string; fill?: string }) {
  const { x, y, width, height, name, fill } = props
  if (width < 4 || height < 4) return null
  return (
    <g>
      <rect x={x} y={y} width={width} height={height} rx={4} fill={fill} fillOpacity={0.75} stroke="#161A21" strokeWidth={2} />
      {width > 40 && height > 24 && (
        <text x={x + width / 2} y={y + height / 2} textAnchor="middle" dominantBaseline="central" fill="#E6E9EE" fontSize={11} fontFamily="JetBrains Mono">
          {name}
        </text>
      )}
    </g>
  )
}
