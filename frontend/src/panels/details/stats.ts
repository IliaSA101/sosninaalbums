// Сводка по альбому для панели деталей. Это счётчики, а не геометрия, поэтому на фронте.
import { baseOf } from '../../base/core/copies'
import type { Photo, SpreadView, View } from '../../base/core/types'

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
  p3: number // фото в Display P3 (iPhone, Xiaomi под любым именем; будут пересчитаны, 119)
  icc: number // фото с другим матричным RGB-профилем (будут пересчитаны, 119)
  iccName: string // единственное имя профиля среди icc; '' — имён несколько или нет
  other: number // фото с другим профилем (не пересчитываются)
}

// Строка предела разворотов (085): предупреждение Go или серая «Типография: предел». Нет поля — строки нет.
export interface LimitLine {
  text: string
  warn: boolean
}
export function limitLine(v: Pick<View, 'pageLimit'>): LimitLine | null {
  const l = v.pageLimit
  if (!l) return null
  if (l.warn) return { text: l.warn, warn: true }
  return { text: `${l.who}: ${l.range}`, warn: false }
}

// Шапка (спека 100 п. 7): только предупреждение о выходе за предел типографии; в пределах — null (счётчик как был).
export function topLimit(v: Pick<View, 'pageLimit'>): { warn: true; title: string } | null {
  const l = limitLine(v)
  return l?.warn ? { warn: true, title: l.text } : null
}

export function orientation(w: number, h: number): Orientation {
  const r = w / h
  // Без размеров не гадаем: считаем квадратным, чтобы сумма H+V+S всё равно сошлась.
  if (!Number.isFinite(r) || r <= 0) return 'sq'
  if (Math.abs(r - 1) <= SQUARE_TOLERANCE) return 'sq'
  return r > 1 ? 'h' : 'v'
}

// Вклад одного листа: по каждому фото — файл (baseOf), ориентация и цвет. Кэш по объекту листа и таблице фото:
// дельта вида (075) сохраняет неизменённые развороты теми же объектами, поэтому правка пересчитывает только
// изменённый, а не все фото альбома. Копии одного файла (098) в сумме — одно фото: считает albumStats по file.
interface Item {
  file: string
  o: Orientation
  color: Photo['color'] | undefined
  profile: string | undefined
}
const parts = new WeakMap<SpreadView, { byId: Map<string, Photo>; items: Item[] }>()

function partOf(s: SpreadView, byId: Map<string, Photo>): Item[] {
  const hit = parts.get(s)
  if (hit && hit.byId === byId) return hit.items
  const items = s.photos.map((id, i): Item => {
    const file = baseOf(id)
    // Фото из недоступной папки в библиотеке нет — берём аспект его ячейки. У копии в ленте её ID нет: смотрим файл.
    const lib = byId.get(file)
    const p = lib ?? s.rects?.[i]
    return { file, o: p ? orientation(p.w, p.h) : 'sq', color: lib?.color, profile: lib?.profile }
  })
  parts.set(s, { byId, items })
  return items
}

// sheets — листы ряда: [обложка?] + развороты, lead — сколько в начале обложек (0 или 1). Фото обложки считаются,
// а в развороты, страницы и раскладки она не входит (032). Файл, стоящий в нескольких местах (копии), считается один раз.
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
    p3: 0,
    icc: 0,
    iccName: '',
    other: 0,
  }
  const iccNames = new Set<string>()
  const keys = new Set<string>()
  const seen = new Set<string>()
  for (const [n, s] of sheets.entries()) {
    if (n >= lead && s.layout) keys.add(s.layout)
    for (const it of partOf(s, byId)) {
      if (seen.has(it.file)) continue
      seen.add(it.file)
      st.photos++
      st[it.o]++
      if (it.color === 'adobe') st.adobe++
      else if (it.color === 'p3') st.p3++
      else if (it.color === 'icc') {
        st.icc++
        iccNames.add(it.profile ?? '')
      } else if (it.color === 'other') st.other++
    }
  }
  if (iccNames.size === 1) st.iccName = [...iccNames][0]
  st.layouts = keys.size
  return st
}

// Строка «Цвет» для сведений об альбоме; null — показывать нечего.
// p3 — одна строка для iPhone и Xiaomi; у icc одно общее имя профиля стоит вместо «другие RGB-профили» (119).
export function colorLine(st: Pick<AlbumStats, 'adobe' | 'p3' | 'icc' | 'iccName' | 'other'>): string | null {
  const parts: string[] = []
  if (st.adobe > 0) parts.push(`Adobe RGB — ${st.adobe} (будут пересчитаны)`)
  if (st.p3 > 0) parts.push(`Display P3 — ${st.p3} (будут пересчитаны)`)
  if (st.icc > 0) parts.push(`${st.iccName || 'другие RGB-профили'} — ${st.icc} (будут пересчитаны)`)
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
