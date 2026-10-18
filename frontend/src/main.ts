import './style.css'
import { mount } from 'svelte'
import App from './App.svelte'
import { wire } from './shell/boot/wire'
import { LogWarning } from '../wailsjs/runtime/runtime'

// Верх подключается к основе до первого кадра (base/ его не импортирует).
wire()

// dev: derived_inert без стека не найти (075-9) — пишем его в журнал wails dev (.gotmp/wails-dev.log).
if (import.meta.env.DEV) {
  const warn = console.warn
  console.warn = (...args: unknown[]) => {
    warn(...args)
    if (!String(args[0]).includes('derived_inert')) return
    try {
      LogWarning(`derived_inert: ${args.map(String).join(' ')}\n${new Error().stack ?? ''}`)
    } catch {
      // вне Wails (чистый браузер без рантайма) журнала нет
    }
  }
}

export default mount(App, { target: document.getElementById('app')! })
