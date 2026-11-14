// Размер плиток окон оформления (133 §7): пределы, умолчания по виду окна и адрес картинки.
// Память — modes.decorTile (настройка окна, не проект); здесь решения без Svelte, под тест.
import { modes } from '../../base/prefs/view.svelte'
import { tileSrc, type DecorKind } from '../decor/decor'

// Вид окна-галереи: четыре окна оформления и «Макеты альбома» (163, те же плитки-листы, что у «Разворотов»).
export type GalleryKind = DecorKind | 'album'

// 96…220 px; «Развороты» и «Макеты» не мельче 128: на меньшем плитка листа нечитаема (0.28-133-3, 163).
export const TILE_MIN = 96
export const TILE_MAX = 220
const SHEET_MIN = 128

// Умолчание: стикеры и рамочки — рисунки, им хватает 120 (было 168); фоны и развороты — 168 как раньше.
const DEFAULT: Record<GalleryKind, number> = { sticker: 120, frame: 120, bg: 168, spread: 168, album: 168 }

export const tileRange = (kind: GalleryKind): { lo: number; hi: number } => ({
  lo: kind === 'spread' || kind === 'album' ? SHEET_MIN : TILE_MIN,
  hi: TILE_MAX,
})
export const tileDefault = (kind: GalleryKind): number => DEFAULT[kind]

// Запомненное → размер: не целое или за пределами вида (мусор, правка файла вручную) — умолчание.
export function tileValue(stored: unknown, kind: GalleryKind): number {
  const { lo, hi } = tileRange(kind)
  return Number.isInteger(stored) && (stored as number) >= lo && (stored as number) <= hi ? (stored as number) : DEFAULT[kind]
}

// Текущий размер окна вида (реактивно: читает modes).
export const tileOf = (kind: GalleryKind): number => tileValue(modes.decorTile[kind], kind)

// Ползунок и колесо: округлить, зажать в пределы вида, запомнить.
export function setTile(kind: GalleryKind, n: number) {
  if (!Number.isFinite(n)) return
  const { lo, hi } = tileRange(kind)
  modes.setDecorTile(kind, Math.max(lo, Math.min(hi, Math.round(n))))
}

// Адрес картинки плитки. Свои и наборы (/media/lib/…) — миниатюра 400 px, Go px не применяет, поэтому без него:
// протяжка размера не перезапрашивает картинки. Встроенные и шаблоны получают px.
export const tileUrl = (url: string, px: number): string => (url.startsWith('/media/lib/') ? url : tileSrc(url, px))

// Размер, от которого считается px: не уменьшается, пока окно открыто (pxHold) — иначе протяжка вниз
// и обратно перезапрашивала бы встроенные картинки.
export const holdSize = (hold: number, size: number): number => Math.max(hold, size)
