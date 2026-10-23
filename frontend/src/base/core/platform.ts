// Платформа и «правый щелчок» (073, п. 4): на Mac Ctrl+щелчок — это правый щелчок.
// Кнопку мыши в обработчиках проверяем только здесь (страж platform.guard.test.ts).

export type Os = 'mac' | 'windows' | 'linux'

let forced: Os | null = null

// Подмена для тестов; null — вернуть определение по userAgent.
export function setOsForTest(v: Os | null) {
  forced = v
}

// Подмена «Mac или нет» (старый вход тестов): не-Mac — Windows.
export function setMacForTest(v: boolean | null) {
  forced = v === null ? null : v ? 'mac' : 'windows'
}

// ОС окна по userAgent; неизвестное (node, браузер без navigator) — Windows, как было до Linux.
export function os(): Os {
  if (forced !== null) return forced
  if (typeof navigator === 'undefined') return 'windows'
  const ua = navigator.userAgent
  if (/Macintosh|Mac OS X/.test(ua)) return 'mac'
  if (/Linux/.test(ua) && !/Android/.test(ua)) return 'linux'
  return 'windows'
}

export const isMac = (): boolean => os() === 'mac'

// Свёртка регистра для сравнения путей и имён папок: Windows и macOS не различают регистр, Linux — различает
// (Go делает так же). Расширения файлов сюда не отдавать — у них своя логика.
export const foldPath = (s: string): string => (os() === 'linux' ? s : s.toLowerCase())

// «Показать … в Проводнике» (101): у каждой ОС свой файловый менеджер, на Linux название не одно — «в папке».
export function revealLabel(): string {
  const o = os()
  return o === 'mac' ? 'Показать в Finder' : o === 'linux' ? 'Показать в папке' : 'Показать в Проводнике'
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
