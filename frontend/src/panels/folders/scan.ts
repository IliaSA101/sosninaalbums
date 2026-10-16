import type { ScanProgress } from '../../base/core/types'

// Текст строки чтения папок (спека 005): сначала обход, потом чтение файлов.
export function scanText(s: ScanProgress): string {
  if (s.walking) return `Ищу фото… найдено ${s.found}`
  return `Читаю папку… ${s.read} из ${s.found}`
}

// Доля прочитанного для тонкой полосы; при обходе итог неизвестен — null (полоса без заполнения).
export function scanFraction(s: ScanProgress): number | null {
  if (s.walking || s.found <= 0) return null
  return Math.min(1, Math.max(0, s.read / s.found))
}
