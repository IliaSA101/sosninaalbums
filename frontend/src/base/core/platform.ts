// Платформа и «правый щелчок» (073, п. 4): на Mac Ctrl+щелчок — это правый щелчок.
// Кнопку мыши в обработчиках проверяем только здесь (страж platform.guard.test.ts).

let forced: boolean | null = null

// Подмена для тестов; null — вернуть определение по userAgent.
export function setMacForTest(v: boolean | null) {
  forced = v
}

export function isMac(): boolean {
  if (forced !== null) return forced
  return typeof navigator !== 'undefined' && /Macintosh|Mac OS X/.test(navigator.userAgent)
}

type Press = { button: number; ctrlKey: boolean }
type Mods = { shiftKey: boolean; ctrlKey: boolean; metaKey: boolean }

// Номер кнопки (уже приведённый через secondaryPress) — левая; для чистой логики без события.
export const isLeft = (button: number): boolean => button === 0

// Обычное нажатие левой кнопкой: жест, выделение, захват указателя. Ctrl на Mac — не оно.
export const primaryPress = (e: Press): boolean => isLeft(e.button) && !(isMac() && e.ctrlKey)

// Правый щелчок: кнопка 2 или Ctrl+левая на Mac (WebKit сам пришлёт contextmenu).
export const secondaryPress = (e: Press): boolean => e.button === 2 || (isMac() && isLeft(e.button) && e.ctrlKey)

// Ctrl (Windows) или Cmd (Mac) — «переключить в выделении»; Ctrl на Mac сюда не входит.
export const toggleKey = (e: { ctrlKey: boolean; metaKey: boolean }): boolean => e.metaKey || (!isMac() && e.ctrlKey)

// Shift или Ctrl/Cmd на щелчке — добавить к выделению или убрать из него.
export const additive = (e: Mods): boolean => e.shiftKey || toggleKey(e)
