#!/usr/bin/env bun

import { spawn } from 'node:child_process'
import { join } from 'node:path'
import { existsSync } from 'node:fs'
import { readFile } from 'node:fs/promises'
import { createWriteStream } from 'node:fs'

console.log('🚀 Starting dev servers...')

// Submodule update removed - run manually if needed

// Install frontend deps if needed
const frontendPath = join(import.meta.dir, 'frontend')
const nodeModulesPath = join(frontendPath, 'node_modules')
const bunLockPath = join(frontendPath, 'bun.lock')

if (!existsSync(nodeModulesPath) && existsSync(bunLockPath)) {
  console.log('📦 Installing frontend deps...')
  const installFrontend = spawn('bun', ['install'], {
    cwd: frontendPath,
    stdio: 'inherit'
  })
  await new Promise((resolve) => {
    installFrontend.on('close', () => resolve(void 0))
  })
}

// Check Go and backend deps
const backendPath = join(import.meta.dir, 'backend')
const goModPath = join(backendPath, 'go.mod')

if (existsSync(goModPath)) {
  spawn('go', ['mod', 'download'], {
    cwd: backendPath,
    stdio: 'pipe'
  })
}

// Load .env
const backendEnvPath = join(backendPath, '.env')
const backendEnvExamplePath = join(backendPath, '.env.example')
let envVars = {}

if (existsSync(backendEnvPath)) {
  const backendEnv = await readFile(backendEnvPath, 'utf-8')
  envVars = Object.fromEntries(
    backendEnv.split('\n')
      .filter(line => line.trim() && !line.startsWith('#'))
      .map(line => {
        const index = line.indexOf('=');
        if (index === -1) return ['', ''];
        return [line.substring(0, index), line.substring(index + 1)];
      })
  )
} else if (existsSync(backendEnvExamplePath)) {
  console.error('❌ No .env found. Copy .env.example to .env first.')
  process.exit(1)
}

// Kill processes on ports
const killProcessOnPort = async (port: number) => {
  try {
    const lsof = spawn('lsof', ['-ti', `:${port}`], { stdio: 'pipe' })
    let lsofOutput = ''
    lsof.stdout?.on('data', (data) => {
      lsofOutput += data.toString()
    })
    await new Promise<void>((resolve) => {
      lsof.on('close', () => resolve())
    })
    const pids = lsofOutput.trim().split('\n').filter(pid => pid)
    pids.forEach(pid => {
      try {
        process.kill(parseInt(pid), 9)
      } catch (e) {
        // Process might already be dead
      }
    })
  } catch (e) {
    // Ignore errors in port killing
  }
}

await killProcessOnPort(6060)
await killProcessOnPort(6070)
await new Promise(resolve => setTimeout(resolve, 1500))

// Kill processes on both ports
await killProcessOnPort(6060)
await killProcessOnPort(6070)
await new Promise(resolve => setTimeout(resolve, 1500))

// Clear and create log stream
const logStream = createWriteStream('backend.log', { flags: 'w' })

// Build backend
const buildProcess = spawn('go', ['build', '.'], {
  cwd: backendPath,
  stdio: 'inherit'
})

await new Promise((resolve, reject) => {
  buildProcess.on('close', (code) => {
    if (code === 0) {
      resolve(void 0)
    } else {
      reject(new Error(`Backend build failed with code ${code}`))
    }
  })
})

const backend = spawn('./zlay-backend', [], {
  cwd: backendPath,
  stdio: ['inherit', 'pipe', 'pipe'],
  env: { ...process.env, ...envVars }
})

backend.stdout?.on('data', (data) => {
  const output = `  \x1b[94m[BACKEND]\x1b[0m  ${data.toString()}`
  process.stdout.write(output)
  logStream.write(output)
})
backend.stderr?.on('data', (data) => {
  const output = `  \x1b[94m[BACKEND]\x1b[0m  ${data.toString()}`
  process.stderr.write(output)
  logStream.write(output)
})

const frontend = spawn('bun', ['run', 'dev'], {
  cwd: frontendPath,
  stdio: ['inherit', 'pipe', 'pipe']
})

frontend.stdout?.on('data', (data) => {
  const output = `  \x1b[92m[FRONTEND]\x1b[0m ${data.toString()}`
  process.stdout.write(output)
  logStream.write(output)
})
frontend.stderr?.on('data', (data) => {
  const output = `  \x1b[92m[FRONTEND]\x1b[0m ${data.toString()}`
  process.stderr.write(output)
  logStream.write(output)
})

process.on('SIGINT', () => {
  console.log('\n🛑 Shutting down...')
  backend.kill('SIGINT')
  frontend.kill('SIGINT')
  logStream.end()
  process.exit(0)
})

Promise.all([
  new Promise((resolve) => backend.on('close', resolve)),
  new Promise((resolve) => frontend.on('close', resolve))
]).then(() => process.exit(0))