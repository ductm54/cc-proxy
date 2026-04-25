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
        <div className="flex items-center justify-center gap-3 mb-4">
          <div
            className="w-9 h-9 rounded-lg grid place-items-center text-base font-extrabold"
            style={{
              background: 'linear-gradient(135deg, #5EEAD4 0%, #3FB6C5 100%)',
              color: '#062321',
            }}
          >
            k
          </div>
          <h1 className="text-2xl font-semibold text-app-text" style={{ letterSpacing: '-0.01em' }}>cc-proxy</h1>
        </div>
        <p className="text-sm text-app-mute mb-8">Sign in to get your API proxy token</p>
        <a
          href="/auth/start"
          className="inline-flex items-center gap-2.5 px-7 py-3 bg-app-text text-app-bg rounded-lg text-sm font-semibold hover:bg-white transition-colors"
        >
          <GoogleIcon className="w-5 h-5" />
          Sign in with Google
        </a>
        {domain && (
          <p className="mt-6 text-xs text-app-dim">
            Restricted to <strong className="text-app-mute font-mono">@{domain}</strong> accounts
          </p>
        )}
      </div>
    </Card>
  )
}
