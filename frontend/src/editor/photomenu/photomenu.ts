// Меню правого щелчка по фото (спека 008, 108): дерево пунктов для данного контекста — группы с подменю
// сбоку (М1). Чистая логика; действия — в PhotoMenu.svelte, слова и функции уровня — в каталоге photoacts.ts.
import { canFitPhoto, type SelBox } from '../../base/geom/gridedit'
import { actualAspect, isDefaultFrame } from '../../base/geom/frame'
import { shapeKey } from '../../base/geom/shape'
import { allGray } from '../../base/geom/tone'
import type { FeatureId } from '../../base/prefs/levels'
import type { BackdropScope, FrameView, SpreadView } from '../../base/core/types'
import type { MenuNode } from '../../base/ui/menulist'
import { isVignette } from '../../panels/vignette/vignette'
import { borderNeedsRelink } from '../border/borderstate'
import { revealLabel } from '../../base/core/platform'
import { LAYER_OPS } from '../places/minipanel'
import { ACTS, actOk, GROUP_ICON, GROUP_LABEL, type ActId, type GroupId } from './photoacts'

// Пункт меню фото — общий узел списка (base/ui/menulist.ts).
export type MenuItem = MenuNode
// Идентификатор действия пункта: ActId; у пунктов подменю («Слой», «На фон») действие несёт параметр arg.
export type MenuId = ActId

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
  faceRef?: boolean // 130 U2: одно фото на листе-виньетке — «Остальные — как это лицо» в «Кадр ▸»
  framed: boolean // одно фото и его кадр изменён или у него свой кадр (098)
  copyable: boolean // 099: есть что копировать (как в «Правке»)
  pasteable: boolean // 099: буфер не пуст
  allFlipH: boolean // 108: зеркало ↔ включено у всех выделенных фото (✓ в «Зеркало ▸»)
  allFlipV: boolean
  tiltOk: boolean // 108: «Наклон в рамке» — одно фото на месте, не пропавшее (как значок на панели)
  scopes: { scope: BackdropScope; label: string }[] // области «На фон ▸» (лист или обложка): считает PhotoMenu
  uses: boolean // «Где ещё стоит ▸» в «Файл ▸» (одно фото; 108 U3: в menuContext включено)
  usesSub?: MenuNode[] // строки подменю вхождений (editor/photoprops/usemenu); нет — пункт без подменю
  draft?: boolean // 131: выделена одна надпись и она заготовка макета — «Сделать обычной надписью»; нужен на любом уровне (вопрос 20)
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
  // «У всех выделенных»: пустое выделение — нет.
  const every = <T>(list: T[], f: (x: T) => boolean) => list.length > 0 && list.every(f)
  return {
    grid: !!s?.grid,
    places: sel.length,
    texts,
    stickers,
    photos: withPhoto.length,
    missing: withPhoto.filter((b) => missing.includes(b.photo)).length,
    borderLost: sel.filter((b) => borderNeedsRelink(s?.places?.find((p) => p.id === b.id)?.border, missing)).length,
    differ,
    allContain: every(sel, (b) => b.fit === 'contain'),
    allCover: every(sel, (b) => b.fit !== 'contain'),
    allRect: every(sel, (b) => shapeKey(b) === 'rect'),
    allRounded: every(sel, (b) => shapeKey(b) === 'round'),
    allOval: every(sel, (b) => shapeKey(b) === 'oval'),
    allGray: allGray(
      s,
      withPhoto.map((b) => b.photo),
    ),
    faceRef: sel.length === 1 && withPhoto.length === 1 && isVignette(s),
    framed: !!fv && isFramed(fv, !!s?.grid),
    copyable: clip.copy,
    pasteable: clip.paste,
    allFlipH: every(withPhoto, (b) => !!frameOf(b.photo)?.flipH),
    allFlipV: every(withPhoto, (b) => !!frameOf(b.photo)?.flipV),
    tiltOk: sel.length === 1 && withPhoto.length === 1 && texts + stickers === 0 && !missing.includes(withPhoto[0].photo),
    scopes: [],
    uses: true,
  }
}

// ── Сборка дерева ──

// Линия между группами. Ставится явно, а остаётся только между видимыми пунктами (tidy).
const SEP = '|' as const
type Row = MenuNode | false | typeof SEP

// Убирает скрытые пункты; линия — только перед видимым пунктом и не первой: скрыт первый пункт группы — линию
// получает следующий видимый.
function tidy(rows: Row[]): MenuNode[] {
  const out: MenuNode[] = []
  let line = false
  for (const r of rows) {
    if (r === SEP) line = true
    else if (r) {
      out.push(line && out.length > 0 ? { ...r, sep: true } : r)
      line = false
    }
  }
  return out
}

// Подменю: без видимых пунктов его нет (уровень, пустое место); с одним остаётся подменю.
function group(id: GroupId, rows: Row[]): MenuNode | false {
  const sub = tidy(rows)
  return sub.length > 0 && { id, label: GROUP_LABEL[id], icon: GROUP_ICON[id], sub }
}

// Пункт каталога: слово — из ACTS, клавиша — из реестра команд (если есть и не снята в настройках).
function node(id: ActId, hint: HintFn, extra: Partial<MenuNode> = {}): MenuNode {
  const cmd = ACTS[id].cmd
  const key = cmd ? hint(cmd) : ''
  const icon = ACTS[id].icon
  return { id, label: ACTS[id].label, ...(icon ? { icon } : {}), ...(key ? { hint: key } : {}), ...extra }
}

// «Крупный просмотр» для меню ленты (128): тот же пункт и клавиша справа, что у меню фото на холсте.
export const previewItem = (hint: HintFn): MenuNode => node('preview', hint)

// Пункт отмечен как состояние (✓ или •).
const check = (on: boolean): Partial<MenuNode> => ({ mark: 'check', checked: on })
const radio = (on: boolean): Partial<MenuNode> => ({ mark: 'radio', checked: on })

// Пункты подменю «Слой ▸»: id один, операцию различает arg; значки — те же, что у «Расположения» в правой панели.
const layerSub = (): MenuNode[] => LAYER_OPS.map((o) => ({ id: 'layer', arg: o.op, icon: o.icon, label: o.label }))

const deleteLabel = (n: number) => (n > 1 ? `Удалить (${n})` : 'Удалить')

// Группа буфера (099): слова, порядок и серость — как у «Правки»; серый — нечего копировать или буфер пуст.
const clipRows = (c: MenuCtx, hint: HintFn): Row[] => [
  node('clipCopy', hint, c.copyable ? {} : { disabled: true }),
  node('clipPaste', hint, c.pasteable ? {} : { disabled: true }),
]

// Что нужно «Файл ▸»: у меню фото — из MenuCtx, у меню ленты (спека 106) — из своего выделения.
export interface FileCtx {
  photos: number // выделено фото
  missing: number // из них без файла на диске
  one: boolean // выделено ровно одно фото
  uses: boolean // пункт «Где ещё стоит ▸» есть (одно фото)
  usesLabel?: string // подпись пункта; нет — «Где ещё стоит» (у ленты — «Где стоит»)
  usesSub?: MenuNode[]
  // Строки вхождений прямо в «Файл ▸» под серой подписью, а не подменю: MenuList рисует два уровня, а «Файл ▸» уже второй
  usesInline?: boolean
}

// Пункты «Файл ▸»: «Показать в …» (слово по ОС) — одному живому файлу; имена копируются только у найденных
// (у пропавшего имени во фронте нет); «Где ещё стоит…» — одному фото, в том числе пропавшему.
export function fileGroup(c: FileCtx, can: (f: FeatureId) => boolean = () => true): MenuNode[] {
  const found = c.photos - c.missing
  const none: HintFn = () => ''
  const on = (id: ActId) => actOk(id, can)
  return tidy([
    c.one && c.missing === 0 && on('explorer') && node('explorer', none, { label: revealLabel() }),
    found > 0 && on('copy') && node('copy', none, { label: found > 1 ? 'Копировать имена файлов' : 'Копировать имя файла' }),
    ...usesRows(c, on('uses'), none),
  ])
}

// «Где ещё стоит»: подменю ▸ (меню ленты) или подпись со строками внутри «Файл ▸» (меню холста).
function usesRows(c: FileCtx, ok: boolean, none: HintFn): Row[] {
  if (!(c.one && c.uses && ok)) return []
  const label = c.usesLabel ?? ACTS.uses.label
  if (c.usesInline && c.usesSub) return [{ id: 'usesHead', label, kind: 'note', sep: true }, ...c.usesSub]
  return [node('uses', none, { label, ...(c.usesSub ? { sub: c.usesSub } : {}) })]
}

// Выпрямлять можно одно фото: режим привязан к выделенному фото, группа с текстом ему мешает.
const canStraighten = (c: MenuCtx) => c.places === 1 && c.photos === 1 && c.texts === 0 && c.stickers === 0

// Подсказка пункта «Остальные — как это лицо» (макет 130-m1-menu).
export const FACE_REF_TIP = 'Как у этого фото — размер лица и линия глаз у всех на листе'

type MkAct = (id: ActId, extra?: Partial<MenuNode>) => MenuNode | false

// Правка фото и места: поворот, зеркало, кадр, форма, ч/б.
function editRows(c: MenuCtx, A: MkAct): Row[] {
  const hasPhoto = c.photos > 0
  return [
    hasPhoto &&
      group('turn', [
        A('turnCw'),
        A('turnCcw'),
        // Выпрямить по линии (045): одно фото, без текстов в группе.
        canStraighten(c) && A('straighten'),
        c.tiltOk && A('tilt'),
      ]),
    hasPhoto && group('mirror', [A('flipH', check(c.allFlipH)), A('flipV', check(c.allFlipV))]),
    hasPhoto &&
      group('frame', [
        A('contain', radio(c.allContain)),
        A('cover', radio(c.allCover)),
        SEP,
        c.differ && A('fitPhoto'),
        c.framed && A('resetFrame'),
        // Команда над листом, а не свойство фото: отдельной группой под чертой (130 U2).
        SEP,
        !!c.faceRef && A('faceRef', { title: FACE_REF_TIP }),
      ]),
    // Форма (044, 076) и рамочка (079) — у любого места, в том числе пустого.
    c.places > 0 &&
      group('shape', [A('rect', radio(c.allRect)), A('rounded', radio(c.allRounded)), A('oval', radio(c.allOval)), SEP, A('border')]),
    // Ч/б (068): всем или никому решает Go, слово — по состоянию выделенных.
    hasPhoto && A('gray', { label: c.allGray ? 'Сделать цветным' : 'Сделать ч/б' }),
  ]
}

// Где фото стоит: фон, слой, подпись.
function placeRows(c: MenuCtx, A: MkAct): Row[] {
  const one = c.places === 1 && c.photos === 1
  const alive = one && c.missing === 0
  return [
    // Фото может стоять и в месте, и на фоне: из места оно не уходит (спека 009).
    alive &&
      c.scopes.length > 0 &&
      A('backdrop', { sub: c.scopes.map((s) => ({ id: 'backdrop', arg: s.scope, icon: ACTS.backdrop.icon, label: s.label })) }),
    c.grid && A('layer', { sub: layerSub() }), // «Слой ▸» — только «Ручная»: у «Авто» места не перекрываются
    one && A('caption'),
  ]
}

function build(c: MenuCtx, can: (f: FeatureId) => boolean, hint: HintFn): Row[] {
  const one = c.places === 1 && c.photos === 1
  const n = c.places + c.texts + c.stickers
  // Пункт каталога с проверкой уровня.
  const A: MkAct = (id, extra = {}) => actOk(id, can) && node(id, hint, extra)
  const files = fileGroup({ photos: c.photos, missing: c.missing, one, uses: c.uses, usesSub: c.usesSub, usesInline: !!c.usesSub }, can)

  return [
    one && c.missing === 0 && A('preview'),
    SEP,
    ...(can('clipboard') ? clipRows(c, hint) : []),
    SEP,
    ...editRows(c, A),
    SEP,
    ...placeRows(c, A),
    SEP,
    group('file', files),
    (c.missing > 0 || c.borderLost > 0) && A('relink'),
    SEP,
    node('delete', hint, { label: deleteLabel(n), hint: 'Del', danger: true }),
  ]
}

// can — уровень интерфейса (не задан — всё доступно). В «Простом» меню правой кнопки нет.
// hint — клавиши команд из реестра (keymap.hint); не задан — без подписи справа.
export function photoMenu(c: MenuCtx, can: (f: FeatureId) => boolean = () => true, hint: HintFn = () => ''): MenuItem[] {
  const own = ownMenu(c, can, hint)
  if (own) return own
  if (c.places === 0 || !can('photoMenu')) return []
  return tidy(build(c, can, hint))
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
  return [node('clipPaste', hint)]
}

// Меню стикеров (048): «Копировать», «Вставить» (099) первыми; «Заменить…» и «Зеркало» — у одного стикера (без отметок:
// состояния стикера в MenuCtx нет); «Слой ▸» — функция слоёв (на «Авто» тоже: мест в цели нет); «Удалить» — всегда.
function stickerMenu(c: MenuCtx, can: (f: FeatureId) => boolean, hint: HintFn): MenuItem[] {
  const solo = c.stickers === 1 && c.texts === 0
  const n = c.stickers + c.texts
  return tidy([
    ...(can('clipboard') ? clipRows(c, hint) : []),
    SEP,
    solo && node('replace', hint),
    SEP,
    solo && node('flipH', hint),
    solo && node('flipV', hint),
    SEP,
    actOk('layer', can) && node('layer', hint, { sub: layerSub() }),
    SEP,
    node('delete', hint, { label: deleteLabel(n), hint: 'Del', danger: true }),
  ])
}

// Меню надписи (099): «Копировать», «Вставить», «Слой ▸» (только «Ручная»: у «Авто» места не перекрываются), «Удалить».
// Без функции буфера надпись меню не получает (раньше его не было); заготовка (131) — исключение: её правят на любом уровне,
// поэтому «Сделать обычной надписью» стоит первым и остаётся единственным пунктом там, где буфера нет.
function textMenu(c: MenuCtx, can: (f: FeatureId) => boolean, hint: HintFn): MenuItem[] {
  const undraft = !!c.draft && node('undraft', hint)
  if (!can('clipboard')) return undraft ? [undraft] : []
  return tidy([
    undraft,
    SEP,
    ...clipRows(c, hint),
    SEP,
    c.grid && actOk('layer', can) && node('layer', hint, { sub: layerSub() }),
    SEP,
    node('delete', hint, { label: deleteLabel(c.texts), hint: 'Del', danger: true }),
  ])
}
