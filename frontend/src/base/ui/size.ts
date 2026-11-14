// Размер узла на экране без синхронного чтения при монтировании (073, U1): `bind:clientWidth`
// заставляет браузер считать раскладку на каждый из тысяч узлов вперемешку со вставкой DOM.
// Один общий ResizeObserver на все узлы; размер читается в его колбэке, где раскладка уже чистая.
type SizeFn = (w: number, h: number) => void

const subs = new WeakMap<Element, SizeFn>()
let ro: ResizeObserver | undefined

// Следит за размером узла; возвращает снятие наблюдения (для `{@attach}`).
// Первый вызов колбэка — сразу после первой раскладки (так ведёт себя ResizeObserver).
export function watchSize(node: Element, cb: SizeFn): () => void {
  ro ??= new ResizeObserver((entries) => {
    for (const e of entries) {
      const t = e.target
      subs.get(t)?.(t.clientWidth, t.clientHeight)
    }
  })
  subs.set(node, cb)
  ro.observe(node)
  return () => {
    subs.delete(node)
    ro?.unobserve(node)
  }
}
