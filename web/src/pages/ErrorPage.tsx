import { useSearchParams } from 'react-router-dom'
import Card from '../components/Card'

export default function ErrorPage() {
  const [params] = useSearchParams()
  const status = params.get('status') ?? '500'
  const message = params.get('message') ?? 'An unexpected error occurred.'

  return (
    <Card>
      <div className="text-center">
        <p className="text-[10px] uppercase tracking-[0.14em] text-app-dim font-semibold mb-2">Error</p>
        <h1 className="text-3xl font-mono font-semibold text-chart-rose mb-3 tabular-nums">{status}</h1>
        <p className="text-sm text-app-mute leading-relaxed">{message}</p>
        <a
          href="/"
          className="inline-block mt-6 px-6 py-2.5 bg-app-surface-2 border border-app-line text-app-text rounded-lg text-sm font-medium hover:border-app-accent/40 hover:text-app-accent transition-colors"
        >
          Try again
        </a>
      </div>
    </Card>
  )
}
