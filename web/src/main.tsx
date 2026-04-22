import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter, Routes, Route } from 'react-router-dom'
import './index.css'
import LoginPage from './pages/LoginPage'
import SuccessPage from './pages/SuccessPage'
import ErrorPage from './pages/ErrorPage'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<LoginPage />} />
        <Route path="/auth/success" element={<SuccessPage />} />
        <Route path="/auth/error" element={<ErrorPage />} />
      </Routes>
    </BrowserRouter>
  </StrictMode>,
)
