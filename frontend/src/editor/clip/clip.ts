// Копировать и вставить (спека 099): чистая часть. Буфер хранит Go; здесь — что отдать Go из выделения,
// слова тоста, страж повтора и признак «выделен текст вне холста». Состояние и вызовы — clip.svelte.ts.
import type { SpreadView } from '../../base/core/types'
import { refsBoxes } from '../../base/geom/gridedit'
import { plural } from '../../base/words/plural'

// Сводка буфера от Go (model.ClipInfo): числа и лист-источник, без путей.
export interface ClipInfo {
  photos: number
  places: number
  texts: number
  stickers: number
  sheet: string
}

// Выделенное на листе для Go (model.ClipSel).
export interface ClipSel {
  refs: string[]
  texts: string[]
  stickers: string[]
}

// Точка листа, мм.
export interface Pt {
  x: number
  y: number
}

export const NO_CLIP: ClipInfo = { photos: 0, places: 0, texts: 0, stickers: 0, sheet: '' }

export type ClipCmd = 'cutItems' | 'copyItems' | 'pasteItems' | 'duplicateItems'

// Буфер пуст — вставлять нечего.
export const clipCount = (i: ClipInfo): number => i.photos + i.places + i.texts + i.stickers
export const canPaste = (i: ClipInfo): boolean => clipCount(i) > 0

// Что лежит на листе из выделенного: места «Ручной» и фото «Авто» — по ссылке (ID места или фото, как у Go),
// надписи и стикеры — по ID. Чужое и исчезнувшее отбрасывается; одиночная надпись (edit.sel) входит в texts.
// null — копировать нечего (Go не зовём, буфер прежний).
export function clipSel(
  s: SpreadView | undefined,
  refs: readonly string[],
  texts: readonly string[],
  stickers: readonly string[],
  single: string | null,
): ClipSel | null {
  if (!s) return null
  const haveT = new Set(s.texts.map((t) => t.id))
  const haveS = new Set((s.stickers ?? []).map((t) => t.id))
  const ids = [...new Set(single ? [single, ...texts] : texts)].filter((id) => haveT.has(id))
  const sel: ClipSel = {
    refs: refsBoxes(s, [...refs]).map((b) => b.id),
    texts: ids,
    stickers: [...new Set(stickers)].filter((id) => haveS.has(id)),
  }
  return sel.refs.length + sel.texts.length + sel.stickers.length > 0 ? sel : null
}

// Что выделить после вставки: Go отдаёт ID новых мест (у «Авто» — фото), а холст выделяет занятое место по фото.
export function pastedRefs(s: SpreadView | undefined, refs: readonly string[]): string[] {
  return refsBoxes(s, [...refs]).map((b) => b.photo || b.id)
}

// «2 фото, надпись»: единицу без числа (так читается тост из спеки).
export function clipWords(i: ClipInfo): string {
  const part = (n: number, forms: [string, string, string]) => (n <= 0 ? '' : n === 1 ? forms[0] : `${n} ${plural(n, forms)}`)
  return [
    part(i.photos, ['фото', 'фото', 'фото']),
    part(i.places, ['пустое место', 'пустых места', 'пустых мест']),
    part(i.texts, ['надпись', 'надписи', 'надписей']),
    part(i.stickers, ['стикер', 'стикера', 'стикеров']),
  ]
    .filter(Boolean)
    .join(', ')
}

// Тост после Ctrl+C и Ctrl+X: «Скопировано: 2 фото, надпись. Вставить — Ctrl+V на любом листе».
// paste — клавиша вставки из настроек; снята — хвоста нет.
export function clipToast(verb: 'Скопировано' | 'Вырезано', i: ClipInfo, paste: string): string {
  const tail = verb === 'Вырезано' ? '' : ' на любом листе'
  return `${verb}: ${clipWords(i)}.${paste ? ` Вставить — ${paste}${tail}` : ''}`
}

// Причина без окна: дубль не спрашивает, а Go повторил repeat и на ответ окна 098 — вид прежний.
export const REPEAT_TEXT = 'Не вставлено: это фото уже стоит в альбоме'

// Страж повтора (099-13): второй путь — события copy/cut/paste браузера (пункты «Правки» на Mac) — не должен
// сработать сразу вслед за клавишей. stamp ставит отметку при выполнении команды, fresh — «после неё прошло > 300 мс».
export const REPEAT_MS = 300
const stamps = new Map<ClipCmd, number>()
export const stamp = (cmd: ClipCmd, now: number) => void stamps.set(cmd, now)
export const fresh = (cmd: ClipCmd, now: number): boolean => now - (stamps.get(cmd) ?? -Infinity) > REPEAT_MS
export const resetStamps = () => stamps.clear()

// Выделен текст страницы вне холста (справочник, «О снимке»): Ctrl+C копирует его, а не выделенное на листе.
export interface SelLike {
  isCollapsed: boolean
  anchorNode: Node | null
  toString(): string
}
export function nativeSel(sel: SelLike | null | undefined): boolean {
  if (!sel || sel.isCollapsed || sel.toString() === '') return false
  // Якорь бывает текстовым узлом (у него нет closest) — тогда берём его элемент.
  const n = sel.anchorNode as (Node & { closest?: (s: string) => unknown }) | null
  const el = n && typeof n.closest === 'function' ? (n as Element) : n?.parentElement
  return !el?.closest('.stage')
}
