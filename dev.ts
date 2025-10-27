#!/usr/bin/env bun

import { spawn } from 'node:child_process'
import { join } from 'node:path'

import { readFile } from 'node:fs/promises'

// Load backend .env file
const backendEnvPath = join(import.meta.dir, 'backend', '.env')
const backendEnv = await readFile(backendEnvPath, 'utf-8')
const envVars = Object.fromEntries(
  backendEnv.split('\n')
    .filter(line => line.trim() && !line.startsWith('#'))
    .map(line => line.split('='))
)

const backend = spawn('zig', ['build', 'dev'], {
  cwd: join(import.meta.dir, 'backend'),
  stdio: 'inherit',
  env: { ...process.env, ...envVars }
})

const frontend = spawn('bun', ['run', 'dev'], {
  cwd: join(import.meta.dir, 'frontend'),
  stdio: 'inherit'
})

process.on('SIGINT', () => {
  backend.kill('SIGINT')
  frontend.kill('SIGINT')
  process.exit(0)
})

Promise.all([
  new Promise((resolve) => backend.on('close', resolve)),
  new Promise((resolve) => frontend.on('close', resolve))
]).then(() => process.exit(0))