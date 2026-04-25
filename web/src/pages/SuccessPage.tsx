import { useEffect, useState, useCallback, useMemo, lazy, Suspense } from 'react'
import { useSearchParams, useNavigate } from 'react-router-dom'
import QuotaBar from '../components/QuotaBar'
const UsageDashboard = lazy(() => import('../components/UsageDashboard'))
import DateRangePicker, { buildUsageQuery } from '../components/DateRangePicker'
import type { Preset } from '../components/DateRangePicker'

interface QuotaWindow {
  utilization: number
  resets_at?: string
}

interface AccountInfo {
  five_hour?: QuotaWindow
  seven_day?: QuotaWindow
  [key: string]: unknown
}

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

const SESSION_WINDOW_MS = 5 * 60 * 60 * 1000

function saveSession(email: string, token: string, proxyURL: string, expiresAt: string) {
  localStorage.setItem('cc-proxy-session', JSON.stringify({ email, token, proxyURL, expiresAt }))
}

export function clearSession() {
  localStorage.removeItem('cc-proxy-session')
}

export function loadSession(): { email: string; token: string; proxyURL: string; expiresAt: string } | null {
  try {
    const raw = localStorage.getItem('cc-proxy-session')
    if (!raw) return null
    const s = JSON.parse(raw)
    if (!s.token || !s.expiresAt) return null
    if (new Date(s.expiresAt) <= new Date()) {
      clearSession()
      return null
    }
    return s
  } catch {
    return null
  }
}

export default function SuccessPage() {
  const [params] = useSearchParams()
  const navigate = useNavigate()
  const stored = loadSession()
  const email = params.get('email') ?? stored?.email ?? ''
  const token = params.get('token') ?? stored?.token ?? ''
  const proxyURL = params.get('proxy_url') ?? stored?.proxyURL ?? ''
  const expiresAt = params.get('expires_at') ?? stored?.expiresAt ?? ''

  const invalidateSession = useCallback(() => {
    clearSession()
    navigate('/', { replace: true })
  }, [navigate])

  useEffect(() => {
    if (token && expiresAt) {
      saveSession(email, token, proxyURL, expiresAt)
    }
  }, [email, token, proxyURL, expiresAt])

  const apiBase = token ? `/p/${token}` : ''

  const [account, setAccount] = useState<AccountInfo | null>(null)
  const [users, setUsers] = useState<UserSummary[]>([])
  const [usageLoading, setUsageLoading] = useState(true)
  const [period, setPeriod] = useState<Preset>('this_week')
  const [customSince, setCustomSince] = useState<string | undefined>()
  const [customUntil, setCustomUntil] = useState<string | undefined>()

  const fetchAccount = useCallback(() => {
    fetch(`${apiBase}/api/account`)
      .then(r => {
        if (r.status === 401) { invalidateSession(); return }
        if (r.ok) return r.json()
        throw new Error(`${r.status}`)
      })
      .then(data => { if (data) setAccount(data) })
      .catch(() => {})
  }, [apiBase, invalidateSession])

  const fetchUsage = useCallback(() => {
    setUsageLoading(true)
    const qs = buildUsageQuery(period, customSince, customUntil)
    fetch(`${apiBase}/api/usage?${qs}`)
      .then(r => {
        if (r.status === 401) { invalidateSession(); return }
        return r.json()
      })
      .then(data => { if (data) setUsers(data.users ?? []) })
      .catch(() => setUsers([]))
      .finally(() => setUsageLoading(false))
  }, [apiBase, period, customSince, customUntil, invalidateSession])

  useEffect(() => {
    fetchAccount()
    const interval = setInterval(fetchAccount, 300_000)
    return () => clearInterval(interval)
  }, [fetchAccount])

  useEffect(() => {
    fetchUsage()
  }, [fetchUsage])

  const expiresMs = useMemo(() => expiresAt ? new Date(expiresAt).getTime() : 0, [expiresAt])
  const [now, setNow] = useState(Date.now())
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(id)
  }, [])
  const remaining = Math.max(0, expiresMs - now)
  const hours = Math.floor(remaining / 3_600_000)
  const mins = Math.floor((remaining % 3_600_000) / 60_000)
  const secs = Math.floor((remaining % 60_000) / 1000)
  const expired = remaining <= 0 && expiresMs > 0
  const sessionPct = expiresMs > 0 ? Math.max(0, Math.min(100, (remaining / SESSION_WINDOW_MS) * 100)) : 0

  return (
    <div className="min-h-screen bg-app-bg text-app-text font-sans p-4 md:p-6 lg:p-8" style={{ letterSpacing: '-0.005em' }}>
      <div className="max-w-[1400px] mx-auto flex flex-col lg:flex-row gap-5 lg:h-[calc(100vh-4rem)]">

        {/* Sidebar */}
        <aside className="lg:w-80 shrink-0 bg-app-surface border border-app-line rounded-xl px-5 py-6 flex flex-col gap-6 lg:overflow-y-auto">
          {/* Logo */}
          <div className="flex items-center gap-3 pb-4 border-b border-app-line">
            <div
              className="w-7 h-7 rounded-lg grid place-items-center text-[14px] font-extrabold"
              style={{
                background: 'linear-gradient(135deg, #5EEAD4 0%, #3FB6C5 100%)',
                color: '#062321',
              }}
            >
              k
            </div>
            <div className="leading-tight">
              <div className="text-sm font-semibold text-app-text">cc-proxy</div>
              <div className="text-[11px] text-app-mute mt-0.5">Dashboard</div>
            </div>
          </div>

          {/* Connection */}
          <SidebarBlock eyebrow="Connection">
            <div className="flex items-center gap-2 mb-2.5">
              <span
                className={`w-1.5 h-1.5 rounded-full ${expired ? 'bg-chart-rose' : 'bg-app-accent'}`}
                style={{ boxShadow: expired ? '0 0 10px #f43f5e' : '0 0 10px #5EEAD4' }}
              />
              <span className="text-sm text-app-text font-medium">{expired ? 'Expired' : 'Connected'}</span>
            </div>
            <div className="text-xs text-app-mute font-mono truncate">{email}</div>
          </SidebarBlock>

          {/* Session expires */}
          <SidebarBlock eyebrow="Session expires in">
            {expired ? (
              <p className="text-xs text-chart-rose">Session expired — please sign in again</p>
            ) : (
              <>
                <div className="flex items-baseline gap-1 font-mono">
                  <TimeUnit n={hours} unit="h" />
                  <span className="text-app-dim text-2xl font-light">:</span>
                  <TimeUnit n={mins} unit="m" />
                  <span className="text-app-dim text-2xl font-light">:</span>
                  <TimeUnit n={secs} unit="s" />
                </div>
                <div className="h-1 bg-app-surface-2 rounded-full mt-2.5 overflow-hidden">
                  <div
                    className="h-full bg-app-accent rounded-full transition-[width] duration-1000 ease-linear"
                    style={{ width: `${sessionPct}%` }}
                  />
                </div>
              </>
            )}
          </SidebarBlock>

          {/* Quota */}
          <SidebarBlock eyebrow="Quota">
            {account && (account.five_hour || account.seven_day) ? (
              <div className="flex flex-col gap-3">
                {account.five_hour && (
                  <QuotaBar
                    label="Session (5 hr)"
                    utilization={account.five_hour.utilization}
                    resetTime={account.five_hour.resets_at ?? ''}
                    status={account.five_hour.utilization >= 100 ? 'blocked' : 'ok'}
                  />
                )}
                {account.seven_day && (
                  <QuotaBar
                    label="Weekly (7 day)"
                    utilization={account.seven_day.utilization}
                    resetTime={account.seven_day.resets_at ?? ''}
                    status={account.seven_day.utilization >= 100 ? 'blocked' : 'ok'}
                  />
                )}
              </div>
            ) : (
              <p className="text-xs text-app-dim">No quota data yet</p>
            )}
          </SidebarBlock>

          {/* Setup snippet */}
          <SetupSnippet proxyURL={proxyURL} />

          {/* Logout */}
          <div className="mt-auto pt-4 border-t border-app-line">
            <button
              onClick={invalidateSession}
              className="w-full text-left text-xs text-app-mute hover:text-app-text transition-colors py-1.5 cursor-pointer"
            >
              Sign out
            </button>
          </div>
        </aside>

        {/* Main */}
        <main className="flex-1 min-w-0 bg-app-surface border border-app-line rounded-xl px-6 py-6 lg:overflow-y-auto">
          <div className="flex items-end justify-between mb-5 flex-wrap gap-3">
            <div>
              <h2 className="text-[22px] font-semibold text-app-text" style={{ letterSpacing: '-0.01em' }}>Usage Analytics</h2>
              <p className="text-[13px] text-app-mute mt-1">Detailed breakdown of proxy usage</p>
            </div>
            <DateRangePicker
              active={period}
              customSince={customSince}
              customUntil={customUntil}
              onChange={(preset, since, until) => {
                setPeriod(preset)
                setCustomSince(since)
                setCustomUntil(until)
              }}
            />
          </div>
          <Suspense fallback={<p className="text-sm text-app-mute py-8 text-center">Loading charts…</p>}>
            <UsageDashboard users={users} loading={usageLoading} />
          </Suspense>
        </main>

      </div>
    </div>
  )
}

function SidebarBlock({ eyebrow, children, action }: { eyebrow: string; children: React.ReactNode; action?: React.ReactNode }) {
  return (
    <div>
      <div className="flex items-center justify-between mb-3">
        <div className="text-[10px] font-semibold uppercase tracking-[0.14em] text-app-dim">{eyebrow}</div>
        {action}
      </div>
      {children}
    </div>
  )
}

function TimeUnit({ n, unit }: { n: number; unit: string }) {
  return (
    <span className="inline-flex items-baseline">
      <span className="text-[28px] font-medium text-app-text tabular-nums">
        {String(n).padStart(2, '0')}
      </span>
      <span className="text-xs text-app-dim ml-0.5">{unit}</span>
    </span>
  )
}

function SetupSnippet({ proxyURL }: { proxyURL: string }) {
  const [copied, setCopied] = useState(false)
  const handleCopy = () => {
    navigator.clipboard.writeText(`ANTHROPIC_BASE_URL="${proxyURL}" \\\nclaude`).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    })
  }
  return (
    <SidebarBlock
      eyebrow="Use with Claude Code"
      action={
        <button
          onClick={handleCopy}
          className="bg-transparent border border-app-line text-app-accent text-[10px] font-semibold px-2.5 py-0.5 rounded-md cursor-pointer hover:bg-app-accent/10 transition-colors"
          style={{ letterSpacing: '0.04em' }}
        >
          {copied ? '✓ Copied' : 'Copy'}
        </button>
      }
    >
      <div
        className="bg-app-code-bg border border-app-line rounded-lg px-3.5 py-3 font-mono text-[11px] leading-[1.7] whitespace-pre-wrap break-all"
        style={{ color: '#C5CCD7' }}
      >
        <span className="text-app-dim">$ </span>
        <span className="text-chart-violet">ANTHROPIC_BASE_URL</span>
        <span>=</span>
        <span className="text-app-accent">"{proxyURL}"</span>
        <span className="text-app-mute"> {'\\'}</span>
        {'\n  '}
        <span>claude</span>
      </div>
      <div className="text-[10px] text-app-dim mt-2">
        Routes Claude Code through your proxy
      </div>
    </SidebarBlock>
  )
}
