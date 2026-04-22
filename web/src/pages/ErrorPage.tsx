import { useSearchParams } from 'react-router-dom'
import Card from '../components/Card'

export default function ErrorPage() {
  const [params] = useSearchParams()
  const status = params.get('status') ?? '500'
  const message = params.get('message') ?? 'An unexpected error occurred.'

  return (
    <Card>
      <div className="text-center">
        <h1 className="text-xl font-bold text-red-400 mb-3">Error {status}</h1>
        <p className="text-sm text-slate-400 leading-relaxed">{message}</p>
        <a
          href="/"
          className="inline-block mt-6 px-6 py-2.5 bg-slate-700 text-slate-200 rounded-lg text-sm font-semibold hover:bg-slate-600 transition-colors"
        >
          Try Again
        </a>
      </div>
    </Card>
  )
}
