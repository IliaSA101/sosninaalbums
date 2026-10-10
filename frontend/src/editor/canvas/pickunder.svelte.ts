// Подключение выбора нижнего объекта (спека 015, design §4): action на `.content` разворота ловит нажатие и
// щелчок в фазе захвата, решает по чистому pressPlan (pickunder.ts) и либо уступает обычным обработчикам,
// либо берёт перенос/выделение на себя. Состояние — только «выбрано Alt+щелчком» и счётчик подсказки.
import { store } from '../../base/core/store.svelte'
import { prefs } from '../../base/prefs/prefs.svelte'
import { edit } from '../text/edit.svelte'
import { placesel } from '../places/placesel.svelte'
import { moveSelected } from '../places/groupgesture'
import { additive } from '../places/gesture'
import { gestureClickAllowed, markDone } from '../places/placegesture'
import { bgmode } from '../backdrop/bgmode.svelte'
import { drag } from '../../base/drag/drag.svelte'
import { gallery } from '../../panels/layouts/gallery.svelte'
import { canvas } from './canvas.svelte'
import { pressPlan, stackAt, type PickItem, type PickKind, type PressPlan } from './pickunder'
import type { SpreadView } from '../../base/core/types'
import type { TextGestures } from '../text/textgesture.svelte'

export interface PickParams {
  spread: () => SpreadView
  root: () => HTMLElement | undefined
  tg: TextGestures
  on: boolean // false — мини-разворот и сосед ряда: жестов нет
}

// Сдвиг от нажатия (px), после которого это уже не щелчок.
const CLICK_SLOP = 3

const params = new WeakMap<Element, PickParams>()

// Выбранное Alt+щелчком: действует, пока подпись выделения не изменилась (design §4 «picked»).
let picked: { item: PickItem; sig: string } | null = null

// Подпись выделения: меняется при любом другом выделении, в том числе на другом развороте.
function sigOf(sid: string): string {
  const e = edit.sel
  return `${sid}|${placesel.refs.join(',')}|${placesel.texts.join(',')}|${e ? e.kind + ':' + e.id : ''}`
}

// Одиночное выделение текущего разворота; группа — null.
function selection(s: SpreadView): { kind: PickKind; ref: string } | null {
  const e = edit.sel
  if (e) return e.spread === s.id && placesel.refs.length + placesel.texts.length === 0 ? { kind: e.kind, ref: e.id } : null
  if (placesel.texts.length > 0 || placesel.refs.length !== 1) return null
  const ref = placesel.refs[0]
  return { kind: s.photos.includes(ref) ? 'photo' : 'place', ref }
}

interface Ctx {
  plan: PressPlan
  top: PickItem | null
  params: PickParams
}

// План считается один раз на событие: первым его спросит TextLayer (window, захват) или сам action.
const cache = new WeakMap<Event, Ctx | null>()

// Жест сейчас занят чем-то другим (режим фона, редактор текста, перенос, зум): Alt+щелчок молчит.
const gestureBusy = () => bgmode.cur || edit.editing || drag.active || canvas.busy || canvas.space || canvas.panning

// Параметры action под событием, если он сейчас вправе работать: текущий разворот, жест свободен.
function activeParams(e: PointerEvent): { p: PickParams; s: SpreadView; root: HTMLElement } | null {
  const node = (e.target as Element | null)?.closest?.('.content')
  const p = node && params.get(node)
  if (!p || !p.on) return null
  const s = p.spread()
  const root = p.root()
  if (!root || store.spread?.id !== s.id || gallery.preview?.spread === s.id) return null
  return gestureBusy() ? null : { p, s, root }
}

function compute(e: PointerEvent): Ctx | null {
  const a = activeParams(e)
  if (!a) return null
  const { p, s, root } = a
  const rect = root.getBoundingClientRect()
  const st = store.view.settings
  const sw = st.pageW * 2
  if (rect.width <= 0) return null
  // Обе оси через ширину (как toSpreadMM в marquee.ts): разворот не растягивается неравномерно.
  const k = sw / rect.width
  const pt = { x: (e.clientX - rect.left) * k, y: (e.clientY - rect.top) * k }
  const stack = stackAt(s, pt, sw, st.pageH, (kind) => kind === 'photo' || !prefs.locked(kind))
  const sel = selection(s)
  const cur = (sel && stack.find((it) => it.kind === sel.kind && it.ref === sel.ref)) || null
  const pk = picked && picked.sig === sigOf(s.id) ? picked.item : null
  const plan = pressPlan({ button: e.button, alt: e.altKey, additive: additive(e), stack, cur, picked: pk })
  return { plan, top: stack[0] ?? null, params: p }
}

function ctxOf(e: PointerEvent): Ctx | null {
  if (!cache.has(e)) cache.set(e, compute(e))
  return cache.get(e) ?? null
}

export const planOf = (e: PointerEvent): PressPlan | null => ctxOf(e)?.plan ?? null

// Нажатие берёт на себя pickunder: глобальный слушатель текста не должен снимать выделение переносимого.
export const pickOwns = (e: PointerEvent): boolean => planOf(e)?.press === 'move'

function selectItem(item: PickItem, sid: string) {
  if (item.kind === 'text' || item.kind === 'caption') edit.select({ kind: item.kind, spread: sid, id: item.ref })
  else {
    if (edit.sel) edit.clear()
    placesel.pick(item.ref, false)
  }
}

// Перенос выбранного прежними жестами; выделение уже стоит.
function startMove(e: PointerEvent, item: PickItem, p: PickParams) {
  const s = p.spread()
  const root = p.root()
  if (!root) return
  if (item.kind === 'text') {
    const t = s.texts.find((x) => x.id === item.ref)
    if (t) p.tg.start(e, t, 'move')
  } else if (item.kind === 'photo' || item.kind === 'place') {
    if (s.grid && prefs.can('manual')) moveSelected(e, root)
    else if (item.kind === 'photo') drag.start(e, () => ({ kind: 'photos', ids: [item.ref], fromSpread: s.id }))
  }
  // Подпись не переносится.
}

// ---------- подсказка про Alt (не больше 3 раз, после первого Alt+щелчка — никогда) ----------

const HINT_KEY = 'pickHint'
const HINT_MAX = 3
const HINT_TEXT = 'Под ним скрыт объект — Alt+щелчок выберет его'

function hintState(): { shown: number; used: boolean } {
  try {
    const v = JSON.parse(localStorage.getItem(HINT_KEY) ?? 'null')
    if (v && typeof v === 'object') return { shown: Number(v.shown) || 0, used: v.used === true }
  } catch {
    // Без хранилища — подсказка по умолчанию.
  }
  return { shown: 0, used: false }
}

function saveHint(h: { shown: number; used: boolean }) {
  try {
    localStorage.setItem(HINT_KEY, JSON.stringify(h))
  } catch {
    // Без хранилища подсказка может повториться.
  }
}

function showHint() {
  const h = hintState()
  if (h.used || h.shown >= HINT_MAX) return
  saveHint({ ...h, shown: h.shown + 1 })
  store.notify(HINT_TEXT, 'info')
}

function markUsed() {
  const h = hintState()
  if (!h.used) saveHint({ ...h, used: true })
}

// ---------- action ----------

export function pickUnder(node: HTMLElement, init: PickParams) {
  params.set(node, init)
  // Нажатие, на которое ждём щелчок: координаты и план.
  let press: { x: number; y: number; ctx: Ctx; sid: string } | null = null

  function down(e: PointerEvent) {
    press = null
    const ctx = ctxOf(e)
    if (!ctx) return
    const { plan } = ctx
    if (plan.press === 'pass' && plan.click === 'pass') picked = null
    press = { x: e.clientX, y: e.clientY, ctx, sid: ctx.params.spread().id }
    // press сбрасывается после щелчка (он идёт сразу за pointerup); без щелчка — следующим нажатием.
    window.addEventListener('pointerup', () => setTimeout(() => (press = null), 0), { once: true })
    if (plan.press !== 'move' || !plan.move) return
    e.stopPropagation()
    // Щелчок после переноса не должен снять выделение с поля.
    window.addEventListener('pointerup', () => markDone(), { once: true })
    startMove(e, plan.move, ctx.params)
  }

  function click(e: MouseEvent) {
    const pr = press
    if (!pr) return
    const { plan, top } = pr.ctx
    const still = Math.hypot(e.clientX - pr.x, e.clientY - pr.y) <= CLICK_SLOP
    if (plan.press === 'move') e.stopPropagation() // нажатие было наше
    if (plan.click === 'select' && plan.select && still && drag.clickAllowed() && gestureClickAllowed()) {
      e.stopPropagation()
      const before = sigOf(pr.sid)
      selectItem(plan.select, pr.sid)
      picked = { item: plan.select, sig: sigOf(pr.sid) }
      if (picked.sig !== before) markUsed()
      markDone()
    } else if (plan.hint && still) {
      // Подсказка — после обычного щелчка, когда выделение уже стоит: выделен верхний.
      setTimeout(() => {
        const sel = selection(pr.ctx.params.spread())
        if (top && sel && sel.kind === top.kind && sel.ref === top.ref) showHint()
      }, 0)
    }
  }

  // Два Alt+щелчка — два шага по кругу: редактор текста не открывается.
  function dbl(e: MouseEvent) {
    const p = params.get(node)
    if (!e.altKey || !p?.on || bgmode.cur || edit.editing) return
    if (store.spread?.id !== p.spread().id) return
    e.stopPropagation()
  }

  node.addEventListener('pointerdown', down, true)
  node.addEventListener('click', click, true)
  node.addEventListener('dblclick', dbl, true)
  return {
    update(next: PickParams) {
      params.set(node, next)
    },
    destroy() {
      node.removeEventListener('pointerdown', down, true)
      node.removeEventListener('click', click, true)
      node.removeEventListener('dblclick', dbl, true)
      params.delete(node)
    },
  }
}
