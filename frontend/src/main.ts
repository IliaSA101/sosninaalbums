import './style.css'
import { mount } from 'svelte'
import App from './App.svelte'
import { wire } from './shell/boot/wire'
import { installUiErrors } from './shell/boot/uierrors'
import { LogWarning } from '../wailsjs/runtime/runtime'

// Верх подключается к основе до первого кадра (base/ его не импортирует).
wire()
// Ошибки скриптов — в log.txt (092 §3). Не в wire(): тесты верха подменяют window заглушкой без слушателей.
installUiErrors()

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
