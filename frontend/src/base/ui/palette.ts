// Палитра: проверка HEX, образцы и «последние цвета» (настройка окна, не проекта — в .salbum не попадает).

const KEY = 'recentColors'
export const RECENT_MAX = 8

// 12 образцов: нейтральные для фона альбома и несколько акцентов.
export const SWATCHES = [
  '#ffffff',
  '#f4efe6',
  '#d9d9d9',
  '#8a8a8a',
  '#3a3a3a',
  '#000000',
  '#7a1f2b',
  '#c0392b',
  '#d9a441',
  '#4f7a5a',
  '#2f5d8a',
  '#5b3f7a',
] as const

// '#rrggbb' или 'rrggbb' (регистр любой) -> '#rrggbb' в нижнем регистре; иначе null.
export function normalizeHex(s: string): string | null {
  const m = /^#?([0-9a-f]{6})$/i.exec(s.trim())
  return m ? '#' + m[1].toLowerCase() : null
}

// Текст ошибки для поля HEX; null — ввод верный.
export function hexError(s: string): string | null {
  return normalizeHex(s) ? null : 'Нужен цвет вида #rrggbb'
}

// Новый цвет — в начало, повтор поднимается, длина не больше RECENT_MAX. Мусор отбрасывается.
export function pushRecent(list: readonly string[], hex: string): string[] {
  const h = normalizeHex(hex)
  if (!h) return list.slice(0, RECENT_MAX)
  return [h, ...list.filter((c) => c !== h)].slice(0, RECENT_MAX)
}

export function loadRecent(): string[] {
  try {
    const v: unknown = JSON.parse(localStorage.getItem(KEY) ?? '[]')
    if (!Array.isArray(v)) return []
    const out: string[] = []
    for (const c of v) {
      const h = typeof c === 'string' ? normalizeHex(c) : null
      if (h && !out.includes(h)) out.push(h)
    }
    return out.slice(0, RECENT_MAX)
  } catch {
    return []
  }
}

// Запоминает выбранный цвет; возвращает новый список (без хранилища — список хотя бы этого вызова).
export function rememberColor(hex: string): string[] {
  const next = pushRecent(loadRecent(), hex)
  try {
    localStorage.setItem(KEY, JSON.stringify(next))
  } catch {
    // Без хранилища последние цвета не запомнятся.
  }
  return next
}
