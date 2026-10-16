// Меню правого щелчка по фото (спека 008, research-edit.md «Меню SmartAlbums и наше»): какие пункты
// показать в данном контексте. Чистая логика; действия — в PhotoMenu.svelte.
import { canFitPhoto, type SelBox } from '../../base/geom/gridedit'
import { actualAspect, isDefaultFrame } from '../../base/geom/frame'
import type { FeatureId } from '../../base/prefs/levels'
import type { SpreadView } from '../../base/core/types'

export type MenuId =
  | 'preview'
  | 'copy'
  | 'explorer'
  | 'backdrop'
  | 'relink'
  | 'fitPhoto'
  | 'contain'
  | 'cover'
  | 'round'
  | 'rect'
  | 'resetFrame'
  | 'flipH'
  | 'flipV'
  | 'turnCw'
  | 'turnCcw'
  | 'straighten'
  | 'layer'
  | 'caption'
  | 'replace'
  | 'delete'

export interface MenuItem {
  id: MenuId
  label: string
  hint?: string // клавиша справа
  danger?: boolean
  sep?: boolean // линия перед пунктом
  sub?: boolean // раскрывает подменю («Слой ▸»: пункты LAYER_OPS; «На фон ▸»: области SCOPE_LABEL)
}

// Что выделено на текущем развороте.
export interface MenuCtx {
  grid: boolean
  places: number // выделенных рамок (фото и пустых мест)
  texts: number // блоков текста в группе
  stickers: number // выделенных стикеров (048); без мест и текстов меню — стикера
  photos: number // из них с фото
  missing: number // из них фото без файла на диске
  differ: boolean // у кого-то пропорции места не совпадают с фото
  allContain: boolean
  allCover: boolean
  allOval: boolean // форма (044): у всех выделенных круг
  allRect: boolean // у всех прямоугольник (в «Авто» всегда)
  framed: boolean // одно фото и его кадр изменён
}

export function menuContext(s: SpreadView | undefined, sel: SelBox[], missing: readonly string[], texts = 0, stickers = 0): MenuCtx {
  const withPhoto = sel.filter((b) => b.photo)
  const frameOf = (photo: string) => s?.frames?.[s.photos.indexOf(photo)]
  const differ = withPhoto.some((b) => {
    const fv = frameOf(b.photo)
    return !!fv && fv.img.h > 0 && !canFitPhoto(b, actualAspect(fv))
  })
  const fv = withPhoto.length === 1 && sel.length === 1 ? frameOf(withPhoto[0].photo) : undefined
  return {
    grid: !!s?.grid,
    places: sel.length,
    texts,
    stickers,
    photos: withPhoto.length,
    missing: withPhoto.filter((b) => missing.includes(b.photo)).length,
    differ,
    allContain: sel.length > 0 && sel.every((b) => b.fit === 'contain'),
    allCover: sel.length > 0 && sel.every((b) => b.fit !== 'contain'),
    allOval: sel.length > 0 && sel.every((b) => b.shape === 'oval'),
    allRect: sel.length > 0 && sel.every((b) => b.shape !== 'oval'),
    framed: !!fv && !isDefaultFrame(fv),
  }
}

// Функция уровня для пункта (levels.ts); пункта нет в таблице — он есть всегда, пока есть само меню.
const FEATURE: Partial<Record<MenuId, FeatureId>> = {
  backdrop: 'bgPhoto',
  fitPhoto: 'frame',
  contain: 'frame',
  cover: 'frame',
  round: 'frame',
  rect: 'frame',
  resetFrame: 'frame',
  flipH: 'flip',
  flipV: 'flip',
  turnCw: 'turn',
  turnCcw: 'turn',
  straighten: 'tilt',
  layer: 'layers',
  caption: 'captions',
}

// can — уровень интерфейса (не задан — всё доступно). В «Простом» меню правой кнопки нет (photoMenu).
export function photoMenu(c: MenuCtx, can: (f: FeatureId) => boolean = () => true): MenuItem[] {
  // Только стикеры (048): своё меню под функцией decor, а не photoMenu — стикер не фото.
  if (c.places === 0 && c.stickers > 0) return can('decor') ? stickerMenu(c, can) : []
  if (c.places === 0 || !can('photoMenu')) return []
  // Линия стоит перед первым пунктом группы: скрыт он — линию получает следующий видимый.
  const out: MenuItem[] = []
  let line = false
  for (const it of build(c)) {
    line ||= !!it.sep
    const f = FEATURE[it.id]
    if (f && !can(f)) continue
    out.push({ ...it, sep: out.length > 0 && line ? true : undefined })
    line = false
  }
  return out
}

const item = (id: MenuId, label: string, extra: Partial<MenuItem> = {}): MenuItem => ({ id, label, ...extra })

// Меню стикеров (048): «Заменить…» и «Зеркало» — у одного стикера; «Слой ▸» — функция слоёв (на «Авто» тоже: мест в цели нет); «Удалить» — всегда.
function stickerMenu(c: MenuCtx, can: (f: FeatureId) => boolean): MenuItem[] {
  const solo = c.stickers === 1 && c.texts === 0
  const n = c.stickers + c.texts
  const list: (MenuItem | false)[] = [
    solo && item('replace', 'Заменить…'),
    solo && item('flipH', 'Зеркало ↔', { sep: true }),
    solo && item('flipV', 'Зеркало ↕'),
    can('layers') && item('layer', 'Слой', { sub: true, sep: true }),
    item('delete', n > 1 ? `Удалить (${n})` : 'Удалить', { hint: 'Del', danger: true, sep: true }),
  ]
  const out = list.filter((it): it is MenuItem => !!it)
  out[0] = { ...out[0], sep: undefined } // линия — только между пунктами
  return out
}

// Просмотр, Проводник, имена и перепривязка — про файл.
function fileItems(c: MenuCtx, one: boolean): MenuItem[] {
  // Просмотр и Проводник нужны живому файлу. Имена копируются только у найденных: у пропавшего
  // фото имени во фронте нет (store.photos без них), если найденных нет — пункта нет.
  const found = c.photos - c.missing
  const alive = one && c.missing === 0
  const out: MenuItem[] = []
  if (alive) out.push(item('preview', 'Крупный просмотр'))
  if (found > 0) out.push(item('copy', found > 1 ? 'Копировать имена файлов' : 'Копировать имя файла'))
  if (alive) out.push(item('explorer', 'Показать в Проводнике'))
  // Фото может стоять и в месте, и на фоне: из места оно не уходит (спека 009).
  if (alive) out.push(item('backdrop', 'На фон', { sub: true }))
  if (c.missing > 0) out.push(item('relink', 'Перепривязать…'))
  return out
}

// Выпрямлять можно одно фото: режим привязан к выделенному фото, группа с текстом ему мешает.
const canStraighten = (c: MenuCtx) => c.places === 1 && c.photos === 1 && c.texts === 0 && c.stickers === 0

// Форма (044) — у любого места, в том числе пустого: показан пункт, которого нет у всех выделенных.
const shapeItems = (c: MenuCtx): MenuItem[] => [
  ...(c.allOval ? [] : [item('round', 'Круглое')]),
  ...(c.allRect ? [] : [item('rect', 'Прямоугольное')]),
]

// Место и кадр; линия — перед первым пунктом группы.
function frameItems(c: MenuCtx): MenuItem[] {
  // Зеркало — фото; «Слой ▸» — любое выделенное место, только «Ручная» (у «Авто» места не перекрываются).
  const hasPhoto = c.photos > 0
  const list: (MenuItem | false)[] = [
    hasPhoto && item('turnCw', 'Повернуть ↻ 90°'),
    hasPhoto && item('turnCcw', 'Повернуть ↺ 90°'),
    // Выпрямить по линии (045): одно фото, без текстов в группе — режим привязан к выделенному фото.
    canStraighten(c) && item('straighten', 'Выпрямить по линии'),
    hasPhoto && item('flipH', 'Зеркало ↔'),
    hasPhoto && item('flipV', 'Зеркало ↕'),
    hasPhoto && c.differ && item('fitPhoto', 'Пропорции под фото'),
    hasPhoto && !c.allContain && item('contain', 'Целиком'),
    hasPhoto && !c.allCover && item('cover', 'С обрезкой'),
    ...shapeItems(c),
    c.framed && item('resetFrame', 'Сбросить кадр'),
    c.grid && item('layer', 'Слой', { sub: true }),
  ]
  const out = list.filter((it): it is MenuItem => !!it)
  if (out.length > 0) out[0] = { ...out[0], sep: true }
  return out
}

function build(c: MenuCtx): MenuItem[] {
  const one = c.places === 1 && c.photos === 1
  const n = c.places + c.texts + c.stickers
  return [
    ...fileItems(c, one),
    ...frameItems(c),
    ...(one ? [item('caption', 'Подпись', { sep: true })] : []),
    item('delete', n > 1 ? `Удалить (${n})` : 'Удалить', { hint: 'Del', danger: true, sep: !one }),
  ]
}
