// Сводка по альбому для панели деталей. Это счётчики, а не геометрия, поэтому на фронте.
import type { Photo, SpreadView } from '../../base/core/types'

// Квадратное — если пропорции отличаются от 1:1 не больше чем на 2 %.
export const SQUARE_TOLERANCE = 0.02

export type Orientation = 'h' | 'v' | 'sq'

export interface AlbumStats {
  spreads: number
  pages: number
  layouts: number // различных ключей раскладки
  photos: number // фото в альбоме
  total: number // фото в библиотеке
  h: number
  v: number
  sq: number
  adobe: number // фото в Adobe RGB (будут пересчитаны)
  other: number // фото с другим профилем (не пересчитываются)
}

export function orientation(w: number, h: number): Orientation {
  const r = w / h
  // Без размеров не гадаем: считаем квадратным, чтобы сумма H+V+S всё равно сошлась.
  if (!Number.isFinite(r) || r <= 0) return 'sq'
  if (Math.abs(r - 1) <= SQUARE_TOLERANCE) return 'sq'
  return r > 1 ? 'h' : 'v'
}

// sheets — листы ряда: [обложка?] + развороты, lead — сколько в начале обложек (0 или 1). Фото обложки считаются,
// а в развороты, страницы и раскладки она не входит (032).
export function albumStats(sheets: SpreadView[], byId: Map<string, Photo>, total: number, lead = 0): AlbumStats {
  const spreads = sheets.length - lead
  const st: AlbumStats = {
    spreads,
    pages: spreads * 2,
    layouts: 0,
    photos: 0,
    total,
    h: 0,
    v: 0,
    sq: 0,
    adobe: 0,
    other: 0,
  }
  const keys = new Set<string>()
  for (const [n, s] of sheets.entries()) {
    if (n >= lead && s.layout) keys.add(s.layout)
    s.photos.forEach((id, i) => {
      st.photos++
      // Фото из недоступной папки в библиотеке нет — берём аспект его ячейки.
      const lib = byId.get(id)
      if (lib?.color === 'adobe') st.adobe++
      else if (lib?.color === 'other') st.other++
      const p = lib ?? s.rects?.[i]
      st[p ? orientation(p.w, p.h) : 'sq']++
    })
  }
  st.layouts = keys.size
  return st
}

// Строка «Цвет» для сведений об альбоме; null — показывать нечего.
export function colorLine(st: Pick<AlbumStats, 'adobe' | 'other'>): string | null {
  const parts: string[] = []
  if (st.adobe > 0) parts.push(`Adobe RGB — ${st.adobe} (будут пересчитаны)`)
  if (st.other > 0) parts.push(`другой профиль — ${st.other}`)
  return parts.length ? 'Цвет: ' + parts.join(', ') : null
}

// Следующий после cur лист с малым разрешением (по кругу: после последнего — с первого); лист — обложка (032, первая) или разворот.
// −1 — таких нет. Если он один и это cur, вернёт cur.
export function nextLowSheet(sheets: Pick<SpreadView, 'lowDPI'>[], cur: number): number {
  const n = sheets.length
  for (let k = 1; k <= n; k++) {
    const i = (((cur + k) % n) + n) % n
    if (sheets[i].lowDPI > 0) return i
  }
  return -1
}
