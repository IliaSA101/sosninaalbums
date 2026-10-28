// Крупный просмотр плитки окон оформления (133 U2): адреса картинок, подпись, строка клавиш, соседи. Чистое, под тест.
import type { BackdropScope, SpreadView } from '../../base/core/types'
import { tilePx, tileSrc, type DecorItem, type DecorKind } from '../decor/decor'
import { tileLook, tplNote, tplSrc } from '../decor/spreadtpl'
import { tileUrl } from './tilesize'

// Вторая строка подсказки плитки (вариант Б вопроса 2: правый щелчок — сразу просмотр, без Ctrl).
export const PEEK_TIP = 'Правый щелчок — крупно'

// Крупная картинка ≤ 1024 у шаблонов разворотов: Go рисует их по запросу (/media/tpl), больше не нужно.
const TPL_MAX = 1024

// Миниатюра плитки — тот же адрес, что у самой плитки (кэш WebView): видна, пока грузится крупная, и если она не загрузилась.
export function thumbSrc(it: DecorItem, kind: DecorKind, sp: SpreadView, scope: BackdropScope, hold: number, dpr: number): string {
  if (kind === 'spread') {
    const look = tileLook(it, sp, scope, hold, dpr)
    return tplSrc(it.url, look.px, look.r)
  }
  return tileUrl(it.url, tilePx(hold, dpr))
}

// Крупная картинка. Свои и наборы — превью 2048 от Go (peek, без px); встроенные — px по большей стороне окна;
// разворот — как плитка, но до 1024. span — большая сторона окна просмотра, CSS px.
export function peekSrc(it: DecorItem, kind: DecorKind, sp: SpreadView, scope: BackdropScope, span: number, dpr: number): string {
  const px = tilePx(span, dpr)
  if (kind === 'spread') return tplSrc(it.url, Math.min(TPL_MAX, px), tileLook(it, sp, scope, span, dpr).r)
  return it.peek || tileSrc(it.url, px)
}

// Пропорции до загрузки картинки (для рамки лупы): фон — как лист, разворот — как его плитка, остальное — по данным плитки.
export function peekGuess(it: DecorItem, kind: DecorKind, sp: SpreadView, scope: BackdropScope): number {
  if (kind === 'bg' && !(it.w > 0 && it.h > 0)) return sp.h > 0 ? sp.w / sp.h : 1
  if (kind === 'spread') return tileLook(it, sp, scope, 100, 1).r
  if (it.w > 0 && it.h > 0) return it.w / it.h
  return it.aspect > 0 ? it.aspect : 1
}

// Подпись: «Имя · набор «…» · N из M»; своё — «свой», встроенное — без второй части, у страницы разворота — «страница».
export function peekCaption(it: DecorItem, n: number, total: number): string {
  const src = it.own ? 'свой' : it.pack ? `набор «${it.pack}»` : ''
  return [it.name, tplNote(it), src, `${n} из ${total}`].filter(Boolean).join(' · ')
}

// Строка клавиш (метки клавиш заполняет keymap.text). Без лупы (разворот, сбой картинки) зума нет.
export function peekHint(mod: string, zoom: boolean, replace: boolean): string {
  const z = zoom ? ` · щелчок{key:photoPreview| или %} — 1:1 · ${mod}+колесо или щипок — зум{key:fitZoom| · % — вписать}` : ''
  return `← → — соседние${z} · Enter — ${replace ? 'заменить' : 'поставить'} · Esc — назад`
}

// Сосед в списке: на краю и для чужого id — тот же id.
export function neighbor(ids: readonly string[], id: string, dir: 1 | -1): string {
  const i = ids.indexOf(id)
  const j = i + dir
  return i < 0 || j < 0 || j >= ids.length ? id : ids[j]
}
