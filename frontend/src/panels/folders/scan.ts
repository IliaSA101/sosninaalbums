import type { ScanProgress } from '../../base/core/types'
import { plural } from '../../base/words/plural'

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

// Событие Go exports:skipped (121): среди файлов папки альбома есть новые выгрузки программы; dir — имя папки без пути.
export interface ExportsSkipped {
  n: number
  dir: string
}

// Тост «выгрузка не в ленте»: файлы с меткой программы — это развороты, а не фото.
export function exportsText(n: number, dir: string): string {
  const files = plural(n, ['файл', 'файла', 'файлов'])
  const taken = plural(n, ['не взят', 'не взяты', 'не взяты'])
  const what = n % 10 === 1 && n % 100 !== 11 ? 'это разворот' : 'это развороты'
  return `В папке «${dir}» ${n} ${files} выгрузки Sosnina Albums — в ленту ${taken}: ${what}, а не фото`
}
