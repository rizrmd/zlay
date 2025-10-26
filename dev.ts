#!/usr/bin/env bun

import { spawn } from 'node:child_process'
import { join } from 'node:path'

const backend = spawn('bash', ['run_dev.sh'], {
  cwd: join(import.meta.dir, 'backend'),
  stdio: 'inherit'
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