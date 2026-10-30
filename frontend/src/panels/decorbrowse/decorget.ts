// Плитки «Можно скачать» в галереях (144): образцы из наборов, которых нет в библиотеке. Чистые решения — плитка как
// предмет каталога, подпись, подсказка, вопрос «Скачать набор?» и тосты. Каталог и состав набора считает Go (Tiles).
// Название набора — недоверенный текст из подписанного каталога: везде только как текст.
import type { DecorCatalog, DecorItem, DecorKind } from '../decor/decor'
import { errText } from '../../base/words/errtext'
import { sizeLabel } from '../../shell/dialogs/backups'
import { contentsLabel } from '../../shell/packs/packs'

export const GET = 'get' // id темы-выборки «Можно скачать»
export const GET_SIDE_TIP = 'Образцы из наборов, которых ещё нет в библиотеке: щелчок скачает набор и поставит выбранное'

// Что ставит плитка: виды окон оформления и макет альбома (его окно — «Макеты альбома»).
export type GetKind = DecorKind | 'album'

// app.DecorGet: плитка образца; with — остальные наборы, что скачаются вместе (не null).
export interface DecorGet {
  id: string // get:<набор>:<src>
  pack: string // id набора для DownloadLibraryPack
  src: string // 16 hex: предмет набора для PackItem
  title: string
  url: string // /media/lp/<id>/<вид>/<i>
  w: number
  h: number
  bytes: number // всё, что скачает «Скачать»: с донором и дополнениями
  with: string[]
  bg: number
  sticker: number
  spread: number
  frame: number
}

// Плитка скачивания в форме предмета каталога: отбор «Набор» (browse) и подсчёт тем читают её как обычную.
export type GetItem = DecorItem & { getTile: true; get: DecorGet }

export const isGetItem = (it: DecorItem): it is GetItem => 'getTile' in it && (it as GetItem).getTile === true

const asItem = (g: DecorGet): GetItem => ({
  id: g.id,
  theme: GET,
  name: g.title,
  url: g.url,
  own: false,
  aspect: g.h > 0 ? g.w / g.h : 0,
  w: g.w,
  h: g.h,
  pack: g.title,
  packId: g.pack,
  getTile: true,
  get: { ...g, with: g.with ?? [] },
})

// Плитки строки «Можно скачать» в порядке каталога; у старого Go поля нет — пусто.
export const getItems = (cat: Pick<DecorCatalog, 'get'> | null | undefined): GetItem[] =>
  (cat?.get ?? []).filter((g) => typeof g?.id === 'string' && typeof g.pack === 'string').map(asItem)

// Адрес клетки образца: Go отдаёт только /media/lp/…, остальное картинкой не просим. Выход из префикса не годится:
// `..`, `%2e` (любой регистр), `\` и `//` после него — пусто (защита в глубину: адрес приходит из сети через Go).
const LP_PREFIX = '/media/lp/'
export const sampleUrl = (url: string): string => {
  if (!url.startsWith(LP_PREFIX)) return ''
  const rest = url.slice(LP_PREFIX.length)
  return rest.includes('..') || /%2e/i.test(rest) || rest.includes('\\') || url.includes('//') ? '' : url
}

// Слова по виду: что ставит плитка (подсказка и вопрос), родительный падеж для «этого … в нём нет», слово на кнопке вопроса.
const WHAT: Record<GetKind, { put: string; no: string; after: string; ok: string }> = {
  bg: { put: 'поставить этот фон', no: 'этого фона', after: 'Этот фон встанет сразу после скачивания.', ok: 'Скачать и поставить' },
  sticker: {
    put: 'поставить этот стикер',
    no: 'этого стикера',
    after: 'Этот стикер встанет сразу после скачивания.',
    ok: 'Скачать и поставить',
  },
  frame: {
    put: 'поставить эту рамочку',
    no: 'этой рамочки',
    after: 'Эта рамочка встанет сразу после скачивания.',
    ok: 'Скачать и поставить',
  },
  spread: {
    put: 'поставить этот разворот',
    no: 'этого разворота',
    after: 'Этот разворот встанет сразу после скачивания.',
    ok: 'Скачать и поставить',
  },
  album: { put: 'показать этот макет', no: 'этого макета', after: 'Макет откроется в этом окне.', ok: 'Скачать и показать' },
}

export const ASK_TITLE = 'Скачать набор?'
export const ASK_CANCEL = 'Отмена'
export const askOk = (kind: GetKind): string => WHAT[kind].ok

// Пропорция плитки: стикер и рамочка — квадрат; остальным окно даёт box — пропорцию стоящей плитки вида (фон и разворот —
// лист или страница по «Куда», макет — лист альбома). Нет box — клетка образца.
export function getRatio(g: Pick<DecorGet, 'w' | 'h'>, kind: GetKind, box?: number): string {
  if (kind === 'sticker' || kind === 'frame') return '1 / 1'
  if (box !== undefined && box > 0) return String(box)
  return g.w > 0 && g.h > 0 ? `${g.w} / ${g.h}` : '3 / 2'
}

// Подпись плитки: «⤓ Школа — осень» и размер «38 МБ» (TplItem склеивает их « · »).
export const getName = (g: Pick<DecorGet, 'title'>): string => `⤓ ${g.title}`
export const getNote = (g: Pick<DecorGet, 'bytes'>): string => sizeLabel(g.bytes)

// Подсказка плитки: что скачается и что встанет.
export const getTitle = (g: Pick<DecorGet, 'title' | 'bytes'>, kind: GetKind): string =>
  `Скачать набор «${g.title}» (${sizeLabel(g.bytes)}) и ${WHAT[kind].put}`

// Вопрос: состав нажатого набора, размер всего скачиваемого, «Вместе с ним» и что встанет.
export function getQuestion(g: DecorGet, kind: GetKind): string {
  const parts = contentsLabel({ bg: g.bg, sticker: g.sticker, spread: g.spread, frame: g.frame, print: 0 })
  const head = `В наборе «${g.title}» ${parts ? `${parts} — ` : '— '}${sizeLabel(g.bytes)}.`
  const withText = g.with?.length ? `Вместе с ним: ${g.with.map((n) => `«${n}»`).join(', ')}.` : ''
  return [head, withText, WHAT[kind].after].filter(Boolean).join(' ')
}

// Приглашение в конце «Все»: списка наборов ещё не было, щелчок открывает окно «Наборы библиотеки».
export const INVITE_LABEL = '⤓ Ещё из интернета'
export const INVITE_TIP = 'Наборы фонов, стикеров и макетов с github.com — окно «Наборы библиотеки»'

// Тосты, когда скачанное не ставится само.
export const otherWhereText = (title: string): string => `Набор «${title}» скачан — этот разворот для другого «Куда», он в «Новых»`
export const noItemText = (title: string, kind: GetKind): string =>
  `Набор «${title}» скачан, но ${WHAT[kind].no} в нём нет — выберите другой`

// Go: предмета набора нет в реестре (decor.ErrNoItem) — не сбой, а «выберите другой».
// caps-ok: сверка по тексту, не показ
export const isNoItem = (e: unknown): boolean => /предмета в наборе нет/i.test(errText(e))
