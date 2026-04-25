import type { ReactNode } from 'react'

export default function Card({ children }: { children: ReactNode }) {
  return (
    <div
      className="min-h-screen flex items-center justify-center bg-app-bg text-app-text font-sans p-4"
      style={{ letterSpacing: '-0.005em' }}
    >
      <div className="bg-app-surface border border-app-line rounded-xl p-10 max-w-md w-full shadow-2xl">
        {children}
      </div>
    </div>
  )
}
