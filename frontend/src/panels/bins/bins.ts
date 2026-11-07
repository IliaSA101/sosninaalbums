// Подборки фото (083): чистая логика под тест — индекс членства, числа, имена, тексты.
// Подборка = {id, name} в View.bins; членство — MarkView.bins у фото (Api.Marks), не геометрия и не проект.
import type { Bin, Photo } from '../../base/core/types'
import type { OwnMarks } from '../marks/marks'

export const MAX_BIN_NAME = 40 // знаков; зеркало model.MaxBinName в Go

export interface BinCount {
  all: number // фото подборки в ленте, без учёта фильтров
  inAlbum: number // из них уже в местах или на фоне = all − «Не в альбоме»
}

export type BinIndex = ReadonlyMap<string, ReadonlySet<string>>

// ID подборки → ID её фото. Пересчитывается при смене карты своих отметок.
export function binIndex(marks: OwnMarks): Map<string, Set<string>> {
  const idx = new Map<string, Set<string>>()
  for (const [photo, m] of Object.entries(marks)) {
    for (const b of m.bins ?? []) {
      let set = idx.get(b)
      if (!set) idx.set(b, (set = new Set()))
      set.add(photo)
    }
  }
  return idx
}

// Фото ленты, входящие в подборку (порядок ленты сохраняется).
export function filterBin(photos: Photo[], bin: string, idx: BinIndex): Photo[] {
  const set = idx.get(bin)
  return set ? photos.filter((p) => set.has(p.id)) : []
}

// Числа окна «в альбоме / всего» за один проход по всей ленте. «В альбоме» = всего − свободные (не в местах и
// не на фоне): так число окна не расходится с тем, что покажет «Не в альбоме» в этой подборке (spec 083 п. 10).
export function binCounts(
  bins: readonly Bin[],
  photos: readonly Photo[],
  idx: BinIndex,
  used: ReadonlySet<string>,
  backdrop: ReadonlySet<string>,
): Map<string, BinCount> {
  const out = new Map<string, BinCount>()
  const sets = bins.map((b) => {
    out.set(b.id, { all: 0, inAlbum: 0 })
    return { c: out.get(b.id)!, set: idx.get(b.id) }
  })
  for (const p of photos) {
    const placed = used.has(p.id) || backdrop.has(p.id)
    for (const { c, set } of sets) {
      if (!set?.has(p.id)) continue
      c.all++
      if (placed) c.inAlbum++
    }
  }
  return out
}

// Что сделает «± N» по выделенным: все уже в подборке — убрать, иначе добавить (как flagClick у меток).
export function binAction(selected: readonly string[], bin: string, idx: BinIndex): 'add' | 'remove' {
  const set = idx.get(bin)
  return set && selected.length > 0 && selected.every((id) => set.has(id)) ? 'remove' : 'add'
}

// Сколько выделенных изменит действие (в подборке уже есть / ещё нет).
export function binChange(selected: readonly string[], bin: string, idx: BinIndex, act: 'add' | 'remove'): number {
  const set = idx.get(bin)
  return selected.filter((id) => (set?.has(id) ?? false) === (act === 'remove')).length
}

// Подборки фото в порядке создания (до Go поля нет).
export const binsOf = (v: { bins?: Bin[] }): Bin[] => v.bins ?? []

// Имя как у Go (CleanBinName): пробелы по краям и повторные схлопнуты, управляющие — пробел, не больше MAX_BIN_NAME знаков.
export function cleanName(s: string): string {
  const t = s
    .replace(/\p{Cc}/gu, ' ') // управляющие знаки
    .replace(/\s+/g, ' ')
    .trim()
  return [...t].slice(0, MAX_BIN_NAME).join('').trim()
}

// Первое свободное «Подборка N» (зеркало FreeBinName).
export function freeName(bins: readonly Bin[]): string {
  const have = new Set(bins.map((b) => b.name.toLowerCase()))
  let n = 1
  while (have.has(`подборка ${n}`)) n++
  return `Подборка ${n}`
}

// Имя занято другой подборкой (без учёта регистра); except — подборка, которую переименовываем.
export function nameTaken(bins: readonly Bin[], name: string, except = ''): boolean {
  const k = cleanName(name).toLowerCase()
  return bins.some((b) => b.id !== except && b.name.toLowerCase() === k)
}

export const TAKEN_TEXT = 'Такая подборка уже есть'

// Длинное имя на кнопке обрезается; полное — в подсказке.
export function binLabel(name: string, max = 16): string {
  const r = [...name]
  return r.length <= max ? name : r.slice(0, max - 1).join('') + '…'
}

// Подпись кнопки: «Подборки», «Подборки 3» — без показанной; «Обязательно 24 / 30» — с ней.
export function binButton(count: number, shown: { name: string; n: BinCount } | null): string {
  if (shown) return `${binLabel(shown.name)} ${shown.n.inAlbum} / ${shown.n.all}`
  return count > 0 ? `Подборки ${count}` : 'Подборки'
}

// Подсказка кнопки: у показанной — полное имя и числа.
export function binButtonTip(shown: { name: string; n: BinCount } | null): string {
  if (!shown) return 'Подборки фото: свои списки, лента показывает одну из них'
  return `Подборка «${shown.name}»: в альбоме ${shown.n.inAlbum} из ${shown.n.all} фото`
}

export const countText = (n: BinCount): string => `${n.inAlbum} / ${n.all}`
export const countTip = (n: BinCount): string => `${n.inAlbum} из ${n.all} фото подборки уже в альбоме`

// Подсказка миниатюры: «Подборки: Обязательно, Для родителей»; пустой список — пусто.
export const binTip = (names: readonly string[]): string => (names.length > 0 ? `Подборки: ${names.join(', ')}` : '')

// Тосты правок (с «Вернуть»).
export const addedText = (name: string, n: number): string => `В «${name}»: +${n}`
export const removedText = (name: string, n: number): string => `Из «${name}»: −${n}`
export const alreadyText = (name: string, act: 'add' | 'remove'): string =>
  act === 'add' ? `Эти фото уже в «${name}»` : `Этих фото нет в «${name}»`
export const deletedText = (name: string): string => `Подборка «${name}» удалена`

// Пустая лента показанной подборки; key — клавиша «В подборку» (keymap.hint), пока её нет — подсказка словами.
export function emptyBinText(name: string, key: string): string {
  const how = key ? `нажмите ${key}` : 'добавьте их кнопкой «+» в окне «Подборки»'
  return `В подборке «${name}» пока нет фото. Выделите фото в «Все фото» и ${how} или перетащите их на кнопку «Подборки»`
}

// Подборка не пуста, но все её фото — в скрытых папках (фильтр папок режет ленту): отдельный текст вместо «пока нет фото».
export const hiddenBinText = (name: string): string => `Фото подборки «${name}» в скрытых папках`

// «Не в альбоме» при показанной подборке: все её фото уже разложены.
export const allBinInAlbumText = (name: string): string => `Все фото подборки «${name}» уже в альбоме`

// Тосты клавиш и броска без выделения, подборок или целевой (сц. 6).
export const NO_SELECTION = 'Выделите фото в ленте'
export const NO_BINS = 'Сначала создайте подборку: кнопка «Подборки»'
export const NO_BINS_MORE = 'Сначала создайте подборку: кнопка «⋯» → «Подборки»'
export const NO_TARGET = 'Выберите подборку в окне «Подборки»'
export const DEL_HINT = 'Del убирает фото из подборки — сначала покажите подборку'

// Где показать окно «Подборки», когда его открыли снаружи (B, Shift+B, бросок): кнопка в полосе — у неё своё окно;
// в «⋯» — открыть «⋯» (там кнопка и окно); иначе окну негде быть — флаг сбросить, чтобы не всплыл позже.
export type BinHost = 'bar' | 'more' | 'none'
export function binHost(present: boolean, hidden: boolean, collapsed: boolean): BinHost {
  if (!present || collapsed) return 'none'
  return hidden ? 'more' : 'bar'
}

// Тост «подборок нет» называет путь к кнопке: в полосе или через «⋯».
export const noBinsText = (host: BinHost): string => (host === 'more' ? NO_BINS_MORE : NO_BINS)

// Бросок фото, пока кнопка «Подборки» в «⋯» (083-29): целью становится сама «⋯» — иначе цели в полосе нет.
// drop — ставить ли `data-drop="bin"` на «⋯» (только пока тащат фото из ленты и кнопка там); text — подпись над
// «⋯» под курсором (та же, что у самой кнопки: `+ в «X»` / `+ в подборку…`).
export function moreBinDrop(host: BinHost, dragging: boolean, over: boolean, target: string): { drop: boolean; text: string } {
  const drop = host === 'more' && dragging
  return { drop, text: drop && over ? dropLabel(target) : '' }
}

// Подпись кнопки «Подборки» во время переноса фото: целевая подборка названа, иначе окно выбора.
export const dropLabel = (target: string): string => (target ? `+ в «${binLabel(target)}»` : '+ в подборку…')

// Строки окна и кнопок с выделением.
export const plusText = (n: number): string => `+ ${n}`
export const minusText = (n: number): string => `− ${n}`

// Строка «Новая» внизу окна: для брошенных на кнопку, для выделения или пустая (Shift+B без выделения).
export const newText = (n: number, dropped: boolean): string =>
  n === 0 ? 'Новая подборка' : dropped ? `Новая из перенесённых (${n})` : `Новая из выделенных (${n})`
