import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import './index.css'

function applySystemTheme() {
  const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches
  document.documentElement.classList.toggle('dark', prefersDark)
}

applySystemTheme()
window
  .matchMedia('(prefers-color-scheme: dark)')
  .addEventListener('change', applySystemTheme)

ReactDOM.createRoot(document.getElementById('root') as HTMLElement).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
)
