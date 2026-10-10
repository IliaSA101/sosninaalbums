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
