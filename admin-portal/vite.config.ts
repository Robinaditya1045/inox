import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { fileURLToPath, URL } from 'node:url'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    },
  },
  server: {
    port: 3000,
    host: '0.0.0.0',
    allowedHosts: true,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        ws: true,
        configure: (proxy, _options) => {
          proxy.on('error', (err, _req, _res) => {
            // Suppress ECONNRESET / ECONNREFUSED terminal spam when backend reconnects or restarts
            if (err.message && (err.message.includes('ECONNRESET') || err.message.includes('ECONNREFUSED') || err.message.includes('EPIPE'))) {
              return
            }
            console.warn(`[vite proxy error] ${err.message}`)
          })
          proxy.on('proxyReqWs', (proxyReq, _req, socket, _options, _head) => {
            socket.on('error', (err) => {
              if (err.message && (err.message.includes('ECONNRESET') || err.message.includes('ECONNREFUSED') || err.message.includes('EPIPE'))) {
                return
              }
              console.warn(`[vite ws proxy socket error] ${err.message}`)
            })
          })
        },
      },
      '/metrics': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        configure: (proxy, _options) => {
          proxy.on('error', (err, _req, _res) => {
            if (err.message && (err.message.includes('ECONNRESET') || err.message.includes('ECONNREFUSED'))) {
              return
            }
          })
        },
      },
      '/media': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        configure: (proxy, _options) => {
          proxy.on('error', (err, _req, _res) => {
            if (err.message && (err.message.includes('ECONNRESET') || err.message.includes('ECONNREFUSED'))) {
              return
            }
          })
        },
      }
    }
  }
})
