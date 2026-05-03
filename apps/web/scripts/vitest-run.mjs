import { spawn } from 'node:child_process'

const args = process.argv.slice(2)
const normalizedArgs = args[0] === '--' ? args.slice(1) : args

const child = spawn('vitest', ['run', ...normalizedArgs], {
  shell: process.platform === 'win32',
  stdio: 'inherit'
})

child.on('exit', (code, signal) => {
  if (signal) {
    process.kill(process.pid, signal)
    return
  }
  process.exit(code ?? 1)
})
