import { QueryClientProvider } from '@tanstack/react-query'
import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { Toaster } from 'sonner'
import App from './App.tsx'
import './index.css'
import { queryClient } from './lib/queryClient'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Toaster
          theme="dark"
          richColors
          closeButton
          position="top-center"
          duration={2200}
          toastOptions={{
            className: 'nofx-toast',
            style: {
              background: '#0b0e11',
              border: '1px solid var(--panel-border)',
              color: 'var(--text-primary)',
            },
          }}
        />
        <App />
      </BrowserRouter>
    </QueryClientProvider>
  </React.StrictMode>
)
