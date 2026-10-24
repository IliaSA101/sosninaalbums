// Ошибки скриптов — в log.txt через App.LogUI (092 §3). Ставится один раз из wire().
// Очистка, маскирование путей и лимит строк — в Go; здесь только сбор текста и защита от зацикливания.
import * as Api from '../../../wailsjs/go/app/App'

const MAX_LEN = 2000
const STACK_LINES = 5

// Только оконные события: ошибки загрузки картинок до window не всплывают (захвата нет).
type Target = Pick<Window, 'addEventListener' | 'removeEventListener'>

function cut(text: string): string {
  return text.length > MAX_LEN ? text.slice(0, MAX_LEN) : text
}

function stackOf(err: unknown): string {
  if (!(err instanceof Error) || !err.stack) return ''
  return '\n' + err.stack.split('\n').slice(0, STACK_LINES).join('\n')
}

export function errorText(e: ErrorEvent): string {
  const place = e.filename ? ` @ ${e.filename}:${e.lineno}:${e.colno}` : ''
  return cut(`${e.message}${place}${stackOf(e.error)}`)
}

export function rejectionText(reason: unknown): string {
  if (reason instanceof Error) return cut(`отказ: ${reason.message}${stackOf(reason)}`)
  return cut(`отказ: ${String(reason)}`)
}

// Шум браузера без последствий: наблюдатель размеров не успел доставить кадр.
function noise(text: string): boolean {
  return text.startsWith('ResizeObserver loop')
}

export function installUiErrors(w: Target = window, send: (t: string) => Promise<void> = (t) => Api.LogUI(t)): () => void {
  let sending = false // ошибка внутри самой отправки не должна вызвать новую отправку
  const report = (text: string) => {
    if (sending || noise(text)) return
    sending = true
    try {
      // Без await: обработчик не ждёт Go. Вне Wails (чистый браузер) LogUI бросает — молчим.
      send(text).catch(() => {})
    } catch {
      // нет рантайма Go — журнала нет
    }
    sending = false
  }
  const onError = (ev: Event) => report(errorText(ev as ErrorEvent))
  const onReject = (ev: Event) => report(rejectionText((ev as PromiseRejectionEvent).reason))
  w.addEventListener('error', onError)
  w.addEventListener('unhandledrejection', onReject)
  return () => {
    w.removeEventListener('error', onError)
    w.removeEventListener('unhandledrejection', onReject)
  }
}
