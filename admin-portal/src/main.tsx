import React from 'react'
import ReactDOM from 'react-dom/client'
import { App } from './App'
import { bootstrapSessionFromUrl } from './lib/api'
import './index.css'

// Adopt a session passed as ?session_id=... before anything renders, so the
// first telemetry/media request already carries credentials.
bootstrapSessionFromUrl()

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
)
