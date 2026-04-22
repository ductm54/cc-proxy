import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import Card from '../components/Card'
import GoogleIcon from '../components/GoogleIcon'
import { loadSession, clearSession } from './SuccessPage'

export default function LoginPage() {
  const [domain, setDomain] = useState('')
  const navigate = useNavigate()

  useEffect(() => {
    fetch('/api/auth/info')
      .then(r => r.json())
      .then(data => setDomain(data.domain ?? ''))
      .catch(() => {})

    const session = loadSession()
    if (session) {
      fetch(`/p/${session.token}/api/session`)
        .then(r => {
          if (r.ok) {
            navigate('/auth/success', { replace: true })
          } else {
            clearSession()
          }
        })
        .catch(() => clearSession())
    }
  }, [navigate])

  return (
    <Card>
      <div className="text-center">
        <h1 className="text-2xl font-bold text-slate-50 mb-2">cc-proxy</h1>
        <p className="text-sm text-slate-400 mb-8">Sign in to get your API proxy token</p>
        <a
          href="/auth/start"
          className="inline-flex items-center gap-2.5 px-7 py-3 bg-white text-slate-800 rounded-lg text-sm font-semibold hover:bg-slate-200 transition-colors"
        >
          <GoogleIcon className="w-5 h-5" />
          Sign in with Google
        </a>
        {domain && (
          <p className="mt-6 text-xs text-slate-500">
            Restricted to <strong className="text-slate-400">@{domain}</strong> accounts
          </p>
        )}
      </div>
    </Card>
  )
}
