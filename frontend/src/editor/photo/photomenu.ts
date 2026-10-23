// Меню правого щелчка по фото (спека 008, research-edit.md «Меню SmartAlbums и наше»): какие пункты
// показать в данном контексте. Чистая логика; действия — в PhotoMenu.svelte.
import { canFitPhoto, type SelBox } from '../../base/geom/gridedit'
import { actualAspect, isDefaultFrame } from '../../base/geom/frame'
import { shapeKey } from '../../base/geom/shape'
import { allGray } from '../../base/geom/tone'
import type { FeatureId } from '../../base/prefs/levels'
import type { FrameView, SpreadView } from '../../base/core/types'
import { borderNeedsRelink } from '../border/borderstate'
import { revealLabel } from '../../base/core/platform'

export type MenuId =
  | 'preview'
  | 'copy'
  | 'clipCopy'
  | 'clipPaste'
  | 'explorer'
  | 'backdrop'
  | 'relink'
  | 'fitPhoto'
  | 'contain'
  | 'cover'
  | 'rect'
  | 'rounded'
  | 'oval'
  | 'border'
  | 'resetFrame'
  | 'flipH'
  | 'flipV'
  | 'gray'
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
  disabled?: boolean // серый пункт: нечего копировать или буфер пуст (как в «Правке»)
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
  borderLost: number // у скольких пропала картинка рамочки без записи в библиотеке (079): ищется папкой
  differ: boolean // у кого-то пропорции места не совпадают с фото
  allContain: boolean
  allCover: boolean
  allRect: boolean // форма (044, 076): у всех прямоугольник (в «Авто» всегда)
  allRounded: boolean // у всех скруглённое
  allOval: boolean // у всех круг
  allGray: boolean // 068: все выделенные фото уже ч/б — пункт «Сделать цветным»
  framed: boolean // одно фото и его кадр изменён или у него свой кадр (098)
  copyable: boolean // 099: есть что копировать (как в «Правке»)
  pasteable: boolean // 099: буфер не пуст
}

// Состояние буфера для серых пунктов (clip.canCopy, clip.canPaste).
export interface ClipState {
  copy: boolean
  paste: boolean
}

// Клавиша команды для подписи справа (keymap.hint); в чистой логике — подставляется.
export type HintFn = (cmd: string) => string

// Кадр изменён: масштаб или сдвиг, а на «Авто» ещё и свой кадр (098) — «Сбросить кадр» снимает и пропорции.
// На сетке свой кадр не действует (место задаёт пропорции), поэтому пункта ради него нет.
const isFramed = (fv: FrameView, grid: boolean) => !isDefaultFrame(fv) || (!grid && (fv.own ?? 0) > 0)

export function menuContext(
  s: SpreadView | undefined,
  sel: SelBox[],
  missing: readonly string[],
  texts = 0,
  stickers = 0,
  clip: ClipState = { copy: true, paste: false },
): MenuCtx {
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
    borderLost: sel.filter((b) => borderNeedsRelink(s?.places?.find((p) => p.id === b.id)?.border, missing)).length,
    differ,
    allContain: sel.length > 0 && sel.every((b) => b.fit === 'contain'),
    allCover: sel.length > 0 && sel.every((b) => b.fit !== 'contain'),
    allRect: sel.length > 0 && sel.every((b) => shapeKey(b) === 'rect'),
    allRounded: sel.length > 0 && sel.every((b) => shapeKey(b) === 'round'),
    allOval: sel.length > 0 && sel.every((b) => shapeKey(b) === 'oval'),
    allGray: allGray(
      s,
      withPhoto.map((b) => b.photo),
    ),
    framed: !!fv && isFramed(fv, !!s?.grid),
    copyable: clip.copy,
    pasteable: clip.paste,
  }
}

// Функция уровня для пункта (levels.ts); пункта нет в таблице — он есть всегда, пока есть само меню.
const FEATURE: Partial<Record<MenuId, FeatureId>> = {
  backdrop: 'bgPhoto',
  fitPhoto: 'frame',
  contain: 'frame',
  cover: 'frame',
  rect: 'frame',
  rounded: 'frame',
  oval: 'frame',
  border: 'border',
  resetFrame: 'frame',
  flipH: 'flip',
  flipV: 'flip',
  gray: 'adjust',
  turnCw: 'turn',
  turnCcw: 'turn',
  straighten: 'tilt',
  layer: 'layers',
  caption: 'captions',
}

// can — уровень интерфейса (не задан — всё доступно). В «Простом» меню правой кнопки нет (photoMenu).
// hint — клавиши команд буфера из реестра (keymap.hint); не задан — без подписи справа.
export function photoMenu(c: MenuCtx, can: (f: FeatureId) => boolean = () => true, hint: HintFn = () => ''): MenuItem[] {
  const own = ownMenu(c, can, hint)
  if (own) return own
  if (c.places === 0 || !can('photoMenu')) return []
  // Линия стоит перед первым пунктом группы: скрыт он — линию получает следующий видимый.
  const out: MenuItem[] = []
  let line = false
  for (const it of build(c, can('clipboard'), hint)) {
    line ||= !!it.sep
    const f = FEATURE[it.id]
    if (f && !can(f)) continue
    out.push({ ...it, sep: out.length > 0 && line ? true : undefined })
    line = false
  }
  return out
}

// Свои меню без мест в выделении: только стикеры (048, функция decor, а не photoMenu — стикер не фото)
// и только надписи (099, по образцу меню стикера). null — выделено место, меню фото.
function ownMenu(c: MenuCtx, can: (f: FeatureId) => boolean, hint: HintFn): MenuItem[] | null {
  if (c.places > 0) return null
  if (c.stickers > 0) return can('decor') ? stickerMenu(c, can, hint) : []
  return c.texts > 0 ? textMenu(c, can, hint) : null
}

// Меню пустого листа (099): одно «Вставить» в точку щелчка; буфер пуст или функции нет — меню нет.
export function sheetMenu(pasteable: boolean, can: (f: FeatureId) => boolean = () => true, hint: HintFn = () => ''): MenuItem[] {
  if (!pasteable || !can('clipboard')) return []
  return [clipItem('clipPaste', 'Вставить', 'pasteItems', false, hint)]
}

const item = (id: MenuId, label: string, extra: Partial<MenuItem> = {}): MenuItem => ({ id, label, ...extra })

// Пункт буфера: слова, порядок и серость — как у «Правки» (menu.ts clipSpecs); клавиша — из реестра.
function clipItem(id: MenuId, label: string, cmd: string, off: boolean, hint: HintFn): MenuItem {
  const key = hint(cmd)
  return item(id, label, { ...(key ? { hint: key } : {}), ...(off ? { disabled: true } : {}) })
}

// Группа буфера (099): Копировать · Вставить; «Вырезать» и «Дублировать» — в «Правке» и на клавишах (меню фото не растим, BACKLOG п. 14).
// Линия перед группой — у вызывающего.
function clipItems(c: MenuCtx, hint: HintFn): MenuItem[] {
  return [
    clipItem('clipCopy', 'Копировать', 'copyItems', !c.copyable, hint),
    clipItem('clipPaste', 'Вставить', 'pasteItems', !c.pasteable, hint),
  ]
}

// Меню стикеров (048): «Копировать», «Вставить» (099) первыми; «Заменить…» и «Зеркало» — у одного стикера; «Слой ▸» — функция слоёв (на «Авто» тоже: мест в цели нет); «Удалить» — всегда.
function stickerMenu(c: MenuCtx, can: (f: FeatureId) => boolean, hint: HintFn): MenuItem[] {
  const solo = c.stickers === 1 && c.texts === 0
  const n = c.stickers + c.texts
  const clip = can('clipboard')
  const list: (MenuItem | false)[] = [
    ...(clip ? clipItems(c, hint) : []),
    solo && item('replace', 'Заменить…', { sep: clip }),
    solo && item('flipH', 'Зеркало ↔', { sep: true }),
    solo && item('flipV', 'Зеркало ↕'),
    can('layers') && item('layer', 'Слой', { sub: true, sep: true }),
    item('delete', n > 1 ? `Удалить (${n})` : 'Удалить', { hint: 'Del', danger: true, sep: true }),
  ]
  return firstNoLine(list)
}

// Меню надписи (099): «Копировать», «Вставить», «Слой ▸» (только «Ручная»: у «Авто» места не перекрываются), «Удалить».
// Без функции буфера надпись меню не получает (раньше его не было).
function textMenu(c: MenuCtx, can: (f: FeatureId) => boolean, hint: HintFn): MenuItem[] {
  if (!can('clipboard')) return []
  const n = c.texts
  return firstNoLine([
    ...clipItems(c, hint),
    c.grid && can('layers') && item('layer', 'Слой', { sub: true, sep: true }),
    item('delete', n > 1 ? `Удалить (${n})` : 'Удалить', { hint: 'Del', danger: true, sep: true }),
  ])
}

// Убрать ложное и поставить: линия — только между пунктами.
function firstNoLine(list: (MenuItem | false)[]): MenuItem[] {
  const out = list.filter((it): it is MenuItem => !!it)
  out[0] = { ...out[0], sep: undefined }
  return out
}

// Просмотр, «Показать в…» (подпись по ОС), имена и перепривязка — про файл.
function fileItems(c: MenuCtx, one: boolean): MenuItem[] {
  // Просмотр и Проводник нужны живому файлу. Имена копируются только у найденных: у пропавшего
  // фото имени во фронте нет (store.photos без них), если найденных нет — пункта нет.
  const found = c.photos - c.missing
  const alive = one && c.missing === 0
  const out: MenuItem[] = []
  if (alive) out.push(item('preview', 'Крупный просмотр'))
  if (found > 0) out.push(item('copy', found > 1 ? 'Копировать имена файлов' : 'Копировать имя файла'))
  if (alive) out.push(item('explorer', revealLabel()))
  // Фото может стоять и в месте, и на фоне: из места оно не уходит (спека 009).
  if (alive) out.push(item('backdrop', 'На фон', { sub: true }))
  if (c.missing > 0 || c.borderLost > 0) out.push(item('relink', 'Перепривязать…'))
  return out
}

// Выпрямлять можно одно фото: режим привязан к выделенному фото, группа с текстом ему мешает.
const canStraighten = (c: MenuCtx) => c.places === 1 && c.photos === 1 && c.texts === 0 && c.stickers === 0

// Форма (044, 076) — у любого места, в том числе пустого: показан пункт, которого нет у всех выделенных.
// Порядок везде один: прямоугольное · скруглённое · круглое.
const shapeItems = (c: MenuCtx): MenuItem[] => [
  ...(c.allRect ? [] : [item('rect', 'Прямоугольное')]),
  ...(c.allRounded ? [] : [item('rounded', 'Скруглённое')]),
  ...(c.allOval ? [] : [item('oval', 'Круглое')]),
]

// Зеркало и ч/б (068) — только у фото. Ч/б: всем или никому решает Go (SetGray), подпись — по состоянию выделенных.
const colorItems = (c: MenuCtx): MenuItem[] =>
  c.photos === 0
    ? []
    : [item('flipH', 'Зеркало ↔'), item('flipV', 'Зеркало ↕'), item('gray', c.allGray ? 'Сделать цветным' : 'Сделать ч/б')]

// Место и кадр; линия — перед первым пунктом группы.
function frameItems(c: MenuCtx): MenuItem[] {
  // Зеркало — фото; «Слой ▸» — любое выделенное место, только «Ручная» (у «Авто» места не перекрываются).
  const hasPhoto = c.photos > 0
  const list: (MenuItem | false)[] = [
    hasPhoto && item('turnCw', 'Повернуть ↻ 90°'),
    hasPhoto && item('turnCcw', 'Повернуть ↺ 90°'),
    // Выпрямить по линии (045): одно фото, без текстов в группе — режим привязан к выделенному фото.
    canStraighten(c) && item('straighten', 'Выпрямить по линии'),
    ...colorItems(c),
    hasPhoto && c.differ && item('fitPhoto', 'Пропорции под фото'),
    hasPhoto && !c.allContain && item('contain', 'Целиком'),
    hasPhoto && !c.allCover && item('cover', 'С обрезкой'),
    ...shapeItems(c),
    item('border', 'Рамочка…'), // 079: окно «Рамочки», у фото и пустого места
    c.framed && item('resetFrame', 'Сбросить кадр'),
    c.grid && item('layer', 'Слой', { sub: true }),
  ]
  const out = list.filter((it): it is MenuItem => !!it)
  if (out.length > 0) out[0] = { ...out[0], sep: true }
  return out
}

function build(c: MenuCtx, clip: boolean, hint: HintFn): MenuItem[] {
  const one = c.places === 1 && c.photos === 1
  const n = c.places + c.texts + c.stickers
  const files = fileItems(c, one)
  // Буфер (099) — сразу после «Крупного просмотра» (если он есть), отдельной группой.
  const head = files.filter((it) => it.id === 'preview')
  const rest = files.filter((it) => it.id !== 'preview')
  return [
    ...head,
    ...(clip ? clipItems(c, hint).map((it, i) => (i === 0 && head.length > 0 ? { ...it, sep: true } : it)) : []),
    ...rest.map((it, i) => (i === 0 && clip ? { ...it, sep: true } : it)),
    ...frameItems(c),
    ...(one ? [item('caption', 'Подпись', { sep: true })] : []),
    item('delete', n > 1 ? `Удалить (${n})` : 'Удалить', { hint: 'Del', danger: true, sep: !one }),
  ]
}
