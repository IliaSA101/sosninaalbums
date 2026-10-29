import { store } from '../../base/core/store.svelte'
import { drag } from '../../base/drag/drag.svelte'
import { tip } from '../../base/ui/tip.svelte'
import { sheetIn, unitOf } from '../../base/geom/sheet'
import { modes } from '../../base/prefs/view.svelte'
import { gridAct } from '../layouts/gridact.svelte'
import { clearVignette, makeVignette } from '../vignette/vigact'
import { vigDialog } from '../vignette/vigsession.svelte'
import { cutState, glueText, joinState } from '../timeline/timeline'
import { naming } from '../timeline/naming.svelte'

export interface MenuState {
  x: number
  y: number
  ids: string[]
  photo: number // номер фото под курсором, -1 — мимо фото
}

// Склейка и разрез через ручной разворот (103): окно «Склеить развороты?» / «Разрезать разворот?» вместо отказа.
// Вызывают черта, зона разреза, Enter/Пробел и пункты меню; до ответа модель не меняется.
export const glueAsk = {
  join(id: string) {
    const spreads = store.view.spreads
    const i = spreads.findIndex((s) => s.id === id)
    if (i < 0 || i >= spreads.length - 1) return
    const manual = [i, i + 1].filter((k) => spreads[k].grid).map((k) => k + 1)
    const t = glueText('join', manual, i + 1, unitOf(store.view.settings))
    void gridAct.askGlue({ kind: 'join', ...t, ids: [id, spreads[i + 1].id], run: (keep) => store.merge(id, keep) })
  },
  cut(id: string, j: number) {
    const i = store.view.spreads.findIndex((s) => s.id === id)
    if (i < 0) return
    const t = glueText('cut', [i + 1], i + 1, unitOf(store.view.settings), j + 1)
    void gridAct.askGlue({ kind: 'cut', ...t, ids: [id], run: (keep) => store.split(id, j, keep) })
  },
}

// Меню правого щелчка по группе: общее для ленты и «Всех разворотов» (спека 103). Состояние и обработчики
// пунктов здесь, один экземпляр на компонент; разметку рисует SpreadMenuHost. where — где править название.
export class SpreadMenuCtl {
  menu = $state<MenuState | null>(null)

  constructor(private readonly where: 'strip' | 'grid') {}

  private readonly spreads = $derived(store.view.spreads)
  readonly max = $derived(store.view.maxSpreadPhotos || 64)
  // Одна группа под меню: её индекс для вставки, разреза и склейки (094 §1.8); несколько — -1.
  readonly at = $derived(this.menu?.ids.length === 1 ? this.spreads.findIndex((s) => s.id === this.menu?.ids[0]) : -1)
  private readonly unit = $derived(unitOf(store.view.settings))
  readonly cut = $derived(this.at >= 0 && this.menu ? cutState(this.spreads[this.at], this.menu.photo, this.unit) : undefined)
  readonly join = $derived(
    this.at >= 0 && this.at < this.spreads.length - 1 ? joinState(this.spreads, this.at, this.max, this.unit) : undefined,
  )

  open(e: MouseEvent, ids: string[], photo: number) {
    e.preventDefault()
    e.stopPropagation()
    if (drag.active) return
    tip.hide()
    this.menu = { x: e.clientX, y: e.clientY, ids, photo }
  }

  close = () => {
    this.menu = null
  }

  doCut() {
    const sp = this.spreads[this.at]
    const j = this.menu?.photo ?? -1
    this.menu = null
    const st = sp && cutState(sp, j, this.unit)
    if (!sp || !st?.ok) return
    if (st.ask) glueAsk.cut(sp.id, j)
    else void store.split(sp.id, j)
  }

  doJoin() {
    const at = this.at
    const sp = this.spreads[at]
    this.menu = null
    const st = sp && joinState(this.spreads, at, this.max, this.unit)
    if (!sp || !st?.ok) return
    if (st.ask) glueAsk.join(sp.id)
    else void store.merge(sp.id)
  }

  // Пустой разворот перед группой или сразу после неё (103 §2): переход на него и тост ошибки — у addEmptySpread.
  insert(where: 'before' | 'after') {
    const i = this.at
    this.menu = null
    if (i >= 0) void store.addEmptySpread(where === 'before' ? i : i + 1)
  }

  // «Сделать виньеткой» / «Обычный разворот» для группы под курсором (031).
  vignette(act: 'make' | 'clear' | 'setup') {
    const sp = this.menu && sheetIn(store.view, this.menu.ids[0])
    this.menu = null
    if (!sp) return
    if (act === 'setup') vigDialog.open(sp.id)
    else if (act === 'make') void makeVignette(sp)
    else void clearVignette(sp.id)
  }

  // «Разнообразить раскладки» (041) — для выделенных групп или той, по которой щёлкнули.
  vary() {
    const ids = this.menu?.ids ?? []
    this.menu = null
    void store.varyLayouts(ids, modes.varyLevel, modes.autoFamilies)
  }

  rename() {
    const id = this.menu?.ids[0]
    this.menu = null
    if (id) naming.start(id, this.where)
  }

  del() {
    const ids = this.menu?.ids ?? []
    this.menu = null
    if (ids.length > 1) store.deleteSpreads(ids)
    else if (ids.length === 1) store.deleteSpread(ids[0])
  }
}
