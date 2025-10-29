#!/usr/bin/env bun

import { spawn } from 'node:child_process'
import { join } from 'node:path'
import { existsSync } from 'node:fs'
import { readFile } from 'node:fs/promises'

console.log('🔍 Checking dependencies...')

// Pull Git submodules first
console.log('📥 Updating Git submodules...')
const submoduleUpdate = spawn('git', ['submodule', 'update', '--init', '--recursive'], {
  cwd: import.meta.dir,
  stdio: 'inherit'
})

await new Promise((resolve, reject) => {
  submoduleUpdate.on('close', (code) => {
    if (code === 0) {
      console.log('✅ Git submodules updated')
      resolve(void 0)
    } else {
      console.log('⚠️  Git submodule update failed, but continuing...')
      resolve(void 0) // Don't reject, allow it to continue
    }
  })
})

// Check and install frontend dependencies
const frontendPath = join(import.meta.dir, 'frontend')
const nodeModulesPath = join(frontendPath, 'node_modules')
const bunLockPath = join(frontendPath, 'bun.lock')

if (!existsSync(nodeModulesPath) && existsSync(bunLockPath)) {
  console.log('📦 Installing frontend dependencies...')
  const installFrontend = spawn('bun', ['install'], {
    cwd: frontendPath,
    stdio: 'inherit'
  })
  
  await new Promise((resolve, reject) => {
    installFrontend.on('close', (code) => {
      if (code === 0) {
        console.log('✅ Frontend dependencies installed')
        resolve(void 0)
      } else {
        reject(new Error(`Frontend install failed with code ${code}`))
      }
    })
  })
} else if (existsSync(nodeModulesPath)) {
  console.log('✅ Frontend dependencies already installed')
} else {
  console.log('⚠️  No bun.lock found, skipping frontend install')
}

// Check if Zig is installed
console.log('🔍 Checking Zig installation...')
const zigVersionCheck = spawn('zig', ['version'], {
  stdio: 'pipe'
})

let zigVersion = ''
zigVersionCheck.stdout?.on('data', (data) => {
  zigVersion += data.toString().trim()
})

await new Promise((resolve, reject) => {
  zigVersionCheck.on('close', (code) => {
    if (code === 0) {
      console.log(`✅ Zig found: ${zigVersion}`)
      resolve(void 0)
    } else {
      console.log('❌ Zig not found. Please install Zig from https://ziglang.org/download/')
      reject(new Error('Zig not installed'))
    }
  })
})

// Check backend dependencies (zig modules)
const backendPath = join(import.meta.dir, 'backend')
const buildZigPath = join(backendPath, 'build.zig')

if (existsSync(buildZigPath)) {
  console.log('🔧 Checking backend build...')
  
  // Try to build backend to check dependencies
  const buildCheck = spawn('zig', ['build'], {
    cwd: backendPath,
    stdio: 'pipe'
  })
  
  let output = ''
  buildCheck.stdout?.on('data', (data) => {
    output += data.toString()
  })
  
  buildCheck.stderr?.on('data', (data) => {
    output += data.toString()
  })
  
  await new Promise((resolve, reject) => {
    buildCheck.on('close', (code) => {
      if (code === 0) {
        console.log('✅ Backend dependencies ready')
        resolve(void 0)
      } else {
        console.log('⚠️  Backend build check failed, but will try to run anyway')
        if (output.includes('error:')) {
          console.log('Build errors detected:')
          console.log(output.split('\n').filter(line => line.includes('error:')).join('\n'))
        }
        resolve(void 0) // Don't reject, allow it to try running
      }
    })
  })
} else {
  console.log('⚠️  No build.zig found in backend')
}

// Load backend .env file
const backendEnvPath = join(backendPath, '.env')
let envVars = {}
if (existsSync(backendEnvPath)) {
  const backendEnv = await readFile(backendEnvPath, 'utf-8')
  envVars = Object.fromEntries(
    backendEnv.split('\n')
      .filter(line => line.trim() && !line.startsWith('#'))
      .map(line => line.split('='))
  )
  console.log('✅ Backend environment loaded')
} else {
  console.log('⚠️  No .env file found in backend')
}

console.log('🚀 Starting development servers...')

const backend = spawn('zig', ['build', 'run'], {
  cwd: backendPath,
  stdio: 'inherit',
  env: { ...process.env, ...envVars }
})

const frontend = spawn('bun', ['run', 'dev'], {
  cwd: frontendPath,
  stdio: 'inherit'
})

process.on('SIGINT', () => {
  console.log('\n🛑 Shutting down servers...')
  backend.kill('SIGINT')
  frontend.kill('SIGINT')
  process.exit(0)
})

Promise.all([
  new Promise((resolve) => backend.on('close', resolve)),
  new Promise((resolve) => frontend.on('close', resolve))
]).then(() => process.exit(0))