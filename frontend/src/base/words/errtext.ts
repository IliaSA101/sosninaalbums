// Текст ошибки для тоста: Wails отклоняет промис строкой (или Error), без префикса «Error: ».
export function errText(e: unknown): string {
  const s = e instanceof Error ? e.message : typeof e === 'string' ? e : String(e ?? '')
  return s.replace(/^\s*Error:\s*/i, '').trim()
}

// Ошибки Go пишутся со строчной буквы; в тосте первая буква заглавная.
export function capFirst(s: string): string {
  return s ? s[0].toUpperCase() + s.slice(1) : s
}

// Текст тоста + «Отклонённые не взяты: N» (если пропущены).
export function withSkipped(text: string, skipped: number): string {
  return [text, skipped > 0 ? `Отклонённые не взяты: ${skipped}` : ''].filter(Boolean).join('. ')
}

// Тост после сбора проекта в папку (069): готовые фразы пишет Go (err, note), здесь — только вид тоста.
// Ошибка и остановка — красный (как в макете), успех — обычный с «Открыть папку» (open).
export function collectDoneText(err: string, note: string, stopped: boolean): { text: string; kind: 'info' | 'error'; open: boolean } {
  if (err) return { text: err, kind: 'error', open: false }
  if (stopped) return { text: note || 'Сбор остановлен', kind: 'error', open: false }
  return { text: note || 'Проект собран', kind: 'info', open: true }
}

// Тост после выгрузки (035 U9): число файлов — последний total из export:progress (0/нет — Go не сообщил),
// сводка цвета от Go — через точку.
export function exportDoneText(total: number | undefined, note?: string): string {
  const head = total && total > 0 ? `Выгружено файлов: ${total}` : 'Выгрузка готова'
  return note ? `${head}. ${note}` : head
}

// Тост остановленной выгрузки (073-4): готовую фразу пишет Go (note), вид — как у остановленного сбора: красный, без «Открыть папку».
export const exportStoppedText = (note: string | undefined): string => note || 'Экспорт остановлен'
