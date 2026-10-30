import { createSubscriber } from 'svelte/reactivity'

// Масштаб экрана (devicePixelRatio) как реактивное значение: смена масштаба Windows или Ctrl+колесо в WebView2
// меняют его на лету без события окна. Единственный сигнал — media query (resolution: Ndppx): при смене она
// перестаёт совпадать; тогда обновляем подписчиков и вешаем запрос заново на новое значение.
// Подписка ленивая (createSubscriber): слушатель живёт, пока значение читает хоть один $derived/$effect.
const now = (): number => globalThis.devicePixelRatio || 1

const subscribe = createSubscriber((update) => {
  let mql: MediaQueryList | null = null
  const onChange = () => {
    update()
    arm()
  }
  const arm = () => {
    mql?.removeEventListener('change', onChange)
    mql = typeof matchMedia === 'function' ? matchMedia(`(resolution: ${now()}dppx)`) : null
    mql?.addEventListener('change', onChange)
  }
  arm()
  return () => mql?.removeEventListener('change', onChange)
})

export const screenDpr = {
  get value(): number {
    subscribe()
    return now()
  },
}
