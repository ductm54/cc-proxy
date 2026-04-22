import { useEffect, useState, useCallback, useMemo, lazy, Suspense } from 'react'
import { useSearchParams, useNavigate } from 'react-router-dom'
import CopyBlock from '../components/CopyBlock'
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

  return (
    <div className="min-h-screen bg-slate-900 text-slate-200 font-sans p-4 md:p-6 lg:p-8">
      <div className="max-w-[1400px] mx-auto flex flex-col lg:flex-row gap-5 lg:h-[calc(100vh-4rem)]">

        {/* Left sidebar: Connection + Quota */}
        <aside className="lg:w-72 xl:w-80 shrink-0 flex flex-col gap-4 lg:overflow-y-auto">
          {/* Connection Info */}
          <div className="bg-slate-800 rounded-xl p-5 shadow-2xl border border-slate-700/30">
            <div className="flex items-center gap-2.5 mb-1">
              <span className={`flex h-2 w-2 rounded-full ${expired ? 'bg-red-400 shadow-[0_0_8px_rgba(248,113,113,0.6)]' : 'bg-emerald-400 shadow-[0_0_8px_rgba(16,185,129,0.6)]'}`} />
              <h1 className="text-sm font-semibold text-slate-100">{expired ? 'Expired' : 'Connected'}</h1>
            </div>
            <p className="text-xs text-slate-400 mb-4">
              <strong className="text-slate-200">{email}</strong>
            </p>

            {/* Countdown */}
            <div className={`rounded-lg p-3 mb-4 text-center ${expired ? 'bg-red-950/40 border border-red-800/30' : 'bg-slate-900/50 border border-slate-700/30'}`}>
              {expired ? (
                <p className="text-xs text-red-400 font-medium">Session expired — please sign in again</p>
              ) : (
                <>
                  <p className="text-[10px] uppercase tracking-widest text-slate-500 mb-1.5">Expires in</p>
                  <div className="flex items-center justify-center gap-1.5 font-mono">
                    <TimeUnit value={hours} label="h" />
                    <span className="text-slate-600 text-sm">:</span>
                    <TimeUnit value={mins} label="m" />
                    <span className="text-slate-600 text-sm">:</span>
                    <TimeUnit value={secs} label="s" />
                  </div>
                </>
              )}
            </div>

            <CopyBlock id="code-claude" label="Use with Claude Code">
              {`ANTHROPIC_BASE_URL="${proxyURL}" \\\nclaude`}
            </CopyBlock>
          </div>

          {/* Quota */}
          <div className="bg-slate-800 rounded-xl p-5 shadow-2xl border border-slate-700/30">
            <h2 className="text-sm font-semibold text-slate-100 mb-4">Quota</h2>
            {account && (account.five_hour || account.seven_day) ? (
              <>
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
              </>
            ) : (
              <p className="text-xs text-slate-500">No quota data yet</p>
            )}
          </div>
        </aside>

        {/* Right main area: Usage Analytics */}
        <main className="flex-1 min-w-0 bg-slate-800 rounded-xl p-5 md:p-6 shadow-2xl border border-slate-700/30 lg:overflow-y-auto">
          <div className="flex items-center justify-between mb-5 flex-wrap gap-3">
            <div>
              <h2 className="text-sm font-semibold text-slate-100">Usage Analytics</h2>
              <p className="text-[11px] text-slate-500 mt-0.5">Detailed breakdown of proxy usage</p>
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
          <Suspense fallback={<p className="text-sm text-slate-500 py-8 text-center">Loading charts…</p>}>
            <UsageDashboard users={users} loading={usageLoading} />
          </Suspense>
        </main>

      </div>
    </div>
  )
}

function TimeUnit({ value, label }: { value: number; label: string }) {
  return (
    <span className="inline-flex items-baseline gap-0.5">
      <span className="text-lg font-semibold text-slate-100 tabular-nums w-7 text-center">
        {String(value).padStart(2, '0')}
      </span>
      <span className="text-[10px] text-slate-500">{label}</span>
    </span>
  )
}
