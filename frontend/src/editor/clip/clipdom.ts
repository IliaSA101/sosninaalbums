// Второй путь буфера (099-13, U3): события copy/cut/paste браузера. Пункты системной «Правки» на Mac
// (Cmd+C тоже уходит в меню приложения) не проходят через keydown, поэтому слушаем сами события документа.
// Чистая часть — domCommand (под тест); установка слушателей — installClipDom. Контекст собирает App:
// editor/ не знает ни окон, ни уровней, ни полей ввода.
import { fresh, type ClipCmd } from './clip'

export type DomKind = 'copy' | 'cut' | 'paste'

const CMD: Readonly<Record<DomKind, ClipCmd>> = { copy: 'copyItems', cut: 'cutItems', paste: 'pasteItems' }

export interface ClipDomCtx {
  inTextField: boolean // цель — поле ввода или contenteditable: там родное копирование текста
  dialog: boolean // открыто окно: клавиши и команды программы под ним молчат
  nativeSel: boolean // выделен текст страницы вне холста (справочник, «О снимке»)
  canvas: boolean // последнее нажатие мыши было на холсте (store.zone)
  clipboardOn: boolean // функция clipboard есть на уровне интерфейса
  off: boolean // режим фона, галерея раскладок или «Во весь экран»
  copyable: boolean
  pasteable: boolean
  now: number
}

// Команда по событию или null — событие остаётся браузеру.
export function domCommand(kind: DomKind, c: ClipDomCtx): ClipCmd | null {
  const cmd = CMD[kind]
  if (c.inTextField || c.dialog || c.nativeSel || !c.canvas || !c.clipboardOn || c.off) return null
  if (!(kind === 'paste' ? c.pasteable : c.copyable)) return null
  // Клавиша уже выполнила команду: событие — её отголосок (WebView2 шлёт и то и другое), второй раз не делаем.
  if (!fresh(cmd, c.now)) return null
  return cmd
}

// Слушатели на документе; ctx читается в момент события. Возвращает снятие слушателей.
export function installClipDom(doc: Document, ctx: (target: Element | null) => Omit<ClipDomCtx, 'now'>, run: (cmd: ClipCmd) => void) {
  const off: Array<() => void> = []
  for (const kind of ['copy', 'cut', 'paste'] as const) {
    const on = (e: Event) => {
      const t = e.target instanceof Element ? e.target : null
      const cmd = domCommand(kind, { ...ctx(t), now: Date.now() })
      if (!cmd) return
      e.preventDefault() // буфер системы не трогаем: он свой (099-1)
      run(cmd)
    }
    doc.addEventListener(kind, on)
    off.push(() => doc.removeEventListener(kind, on))
  }
  return () => off.forEach((f) => f())
}
