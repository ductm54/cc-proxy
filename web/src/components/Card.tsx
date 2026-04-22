import type { ReactNode } from 'react'

export default function Card({ children }: { children: ReactNode }) {
  return (
    <div className="min-h-screen flex items-center justify-center bg-slate-900 text-slate-200 p-4">
      <div className="bg-slate-800 rounded-xl p-10 max-w-md w-full shadow-2xl">
        {children}
      </div>
    </div>
  )
}
