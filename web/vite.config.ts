import net from 'node:net'
import path from 'node:path'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

const defaultApiProxyTarget = 'http://127.0.0.1:8080'
const autoProbeApiProxyTargets = ['http://127.0.0.1:18081', defaultApiProxyTarget, 'http://localhost:8080']

function isReachable(target: string, timeoutMs = 250): Promise<boolean> {
  return new Promise((resolve) => {
    let settled = false
    const finish = (reachable: boolean) => {
      if (settled) {
        return
      }
      settled = true
      resolve(reachable)
    }

    let url: URL
    try {
      url = new URL(target)
    } catch {
      finish(false)
      return
    }

    const port = Number(url.port || (url.protocol === 'https:' ? 443 : 80))
    const socket = net.createConnection({ host: url.hostname, port })

    socket.once('connect', () => {
      socket.destroy()
      finish(true)
    })
    socket.once('error', () => {
      socket.destroy()
      finish(false)
    })
    socket.setTimeout(timeoutMs, () => {
      socket.destroy()
      finish(false)
    })
  })
}

async function resolveApiProxyTarget(): Promise<string> {
  const explicitTarget = process.env.VITE_API_PROXY_TARGET
  if (explicitTarget) {
    return explicitTarget
  }

  for (const target of autoProbeApiProxyTargets) {
    if (await isReachable(target)) {
      return target
    }
  }

  return defaultApiProxyTarget
}

export default defineConfig(async () => {
  const apiProxyTarget = await resolveApiProxyTarget()
  console.log(`[vite] proxying /api to ${apiProxyTarget}`)

  return {
    plugins: [react()],
    resolve: {
      alias: {
        '@': path.resolve(__dirname, './src'),
      },
    },
    server: {
      proxy: {
        '/api': {
          target: apiProxyTarget,
          changeOrigin: true,
        },
      },
    },
  }
})
