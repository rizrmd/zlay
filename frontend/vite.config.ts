import { fileURLToPath, URL } from 'node:url'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import vueJsx from '@vitejs/plugin-vue-jsx'

import nightwatchPlugin from 'vite-plugin-nightwatch'

// Read backend ports from .env
const backendEnvPath = join(__dirname, '../backend/.env')
const backendEnv = readFileSync(backendEnvPath, 'utf-8')
const portMatch = backendEnv.match(/PORT=(\d+)/)
const backendPort = portMatch ? portMatch[1] : '8080'
const wsPortMatch = backendEnv.match(/WS_PORT=(\d+)/)
const wsPort = wsPortMatch ? wsPortMatch[1] : '6070'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue(), vueJsx(), nightwatchPlugin()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 6060,
    logLevel: 'warn',
    proxy: {
      '/api': {
        target: `http://localhost:${backendPort}`,
        changeOrigin: true,
        secure: false,
        configure: (proxy, options) => {
          proxy.on('proxyReq', (proxyReq, req, res) => {
            const origin = req.headers.origin || req.headers.referer || `http://${req.headers.host}`;
            proxyReq.setHeader('X-Original-Origin', origin);
          });
        },
      },
      '/ws': {
        target: `http://localhost:${wsPort}`,
        changeOrigin: true,
        ws: true,
      },
    },
    allowedHosts: ['dev.maldevta.local', 'domain.example.local', 'cobodomain.com'],
  },
  clearScreen: true,
})
