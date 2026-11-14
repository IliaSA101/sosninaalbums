// Состояние окна сравнения 2–4 кадров (138): открытие, активная ячейка, «Лучший», закрытие.
// Список замораживается при открытии (как у окна 128). Жесты и общий зум — SyncLoupe, раскладка — tiles.ts.
import { Api, store } from '../../base/core/store.svelte'
import { prefs } from '../../base/prefs/prefs.svelte'
import { keymap } from '../../base/keys/keymap.svelte'
import { errText } from '../../base/words/errtext'
import { photoui } from '../../editor/photo/photoui.svelte'
import { nearestVisible } from '../../editor/photo/previewlist'
import { PreviewOriginal } from '../../editor/photo/original.svelte'
import type { Size } from '../../editor/photo/loupe'
import {
  activeId,
  closeSelection,
  dropGone,
  isLastLeft,
  openBySelection,
  openIds,
  openPair,
  removeCell,
  stepCell,
  type CompareState,
  type OpenWhy,
} from './compareset'
import { SyncLoupe } from './syncloupe.svelte'
import { tiles, type Cell } from './tiles'
import { budgetText, origBudget, tileMax } from './synczoom'

// Зазор между ячейками и высота подписи под кадром, px.
export const GAP = 10
export const CAP = 34

// Тексты тостов (design §7): Т2, Т3, Т4 — окно не открылось; Т5, Т6, Т7 — «Похожие к этому».
export const WHY_TEXT: Record<OpenWhy, (n: number) => string> = {
  none: () => 'Выделите в ленте 2–4 фото, чтобы сравнить',
  many: (n) => `Сравнить можно до 4 фото — выделено ${n}`,
  alone: () => 'Сравнивать не с чем: в ленте одно фото',
}
export const SIMILAR_TEXT = {
  more: (n: number) => `Похожих рядом ${n} — показаны 3 ближайших`,
  none: 'Похожих кадров рядом в ленте нет',
  nosig: 'Похожих пока не найти: миниатюра этого фото ещё готовится',
}

// Кого «Лучший» отклоняет: остальные кадры окна (0.29-138-8). Владелец может решить «только отбор» —
// тогда здесь `() => []`, и в Go ничего менять не нужно.
export const restForBest = (others: string[]): string[] => others

// Выбор «Лучший» доступен с функцией «Отметки» уровня.
const canBest = (): boolean => prefs.can('marks')

class Compare {
  st = $state<CompareState | null>(null)
  box = $state<Size>({ w: 0, h: 0 }) // область ячеек в окне, px (bind:clientWidth/Height)
  private sizes = $state.raw<Record<string, Size>>({}) // размер загруженного превью, px
  private lost = $state.raw<Record<string, true>>({}) // превью не загрузилось
  private carryFrom = new Map<string, Size>() // листание: размер кадра, которого ячейка лишилась
  private origs = new Map<string, PreviewOriginal>()
  private seq = 0 // номер вызова «Похожих»: устаревший ответ отбрасывается
  private busy = false // «Лучший» в пути
  readonly loupe = new SyncLoupe(() => ({
    ids: this.ids,
    active: this.active,
    boxes: this.cells.map((c) => ({ w: c.w, h: c.h })),
    nat: (id) => this.nat(id),
    ready: (id) => !this.gone(id) && this.sizes[id] !== undefined,
    max: (id) => this.max(id),
    pick: (i) => this.pick(i),
  }))

  get open(): boolean {
    return !!this.st
  }
  get ids(): string[] {
    return this.st?.cells ?? []
  }
  get active(): number {
    return this.st?.active ?? 0
  }
  get activeId(): string | null {
    return this.st ? activeId(this.st) : null
  }
  // «Лучший» есть: функция «Отметки» включена и кадров не меньше двух.
  get canBest(): boolean {
    return canBest() && this.ids.length > 1
  }

  // Размер кадра для расчёта: загруженный, до загрузки — пропорции из данных фото (только для рамки).
  nat(id: string): Size {
    const s = this.sizes[id]
    if (s) return s
    const p = store.byId.get(id)
    return p && p.w > 0 && p.h > 0 ? { w: (p.w / p.h) * 1000, h: 1000 } : { w: 1500, h: 1000 }
  }
  // Файла нет (088) или он не читается: Go уже сообщил (store.missing, store.unread) — заглушка сразу, как в ленте;
  // либо превью не загрузилось.
  gone(id: string): boolean {
    return !!this.lost[id] || store.missing.includes(id) || !!store.unread[id]
  }
  orig(id: string): PreviewOriginal {
    let o = this.origs.get(id)
    if (!o) this.origs.set(id, (o = new PreviewOriginal()))
    return o
  }
  // Оригиналы выше 1:1: всем или никому (бюджет 100 Мп и sRGB); пусто — можно.
  why = $derived(origBudget(this.ids.filter((id) => !this.gone(id)).map((id) => store.byId.get(id))))
  get note(): string {
    const a = this.activeId
    return a && this.loupe.zoomed(this.active) && this.loupe.scale(this.active) >= 1 - 1e-6 && this.loupe.label ? budgetText(this.why) : ''
  }
  max(id: string): number {
    return this.gone(id) || this.orig(id).failed(id) ? 1 : tileMax(this.why, store.byId.get(id), this.nat(id))
  }
  // Пропорция кадра для сетки.
  private aspect(id: string): number {
    const n = this.nat(id)
    return n.w / n.h
  }
  cells = $derived<Cell[]>(
    tiles(
      this.ids.map((id) => this.aspect(id)),
      this.box,
      GAP,
      CAP,
    ),
  )

  // ── открытие ──
  // before — перед показом окна (окно 128 передаёт кадры и закрывается: только если сравнение действительно открылось).
  private start(r: CompareState | OpenWhy, selected = 0, before?: () => void): boolean {
    if (typeof r === 'string') {
      store.notify(WHY_TEXT[r](selected))
      return false
    }
    before?.()
    this.reset()
    this.st = r
    return true
  }
  private reset() {
    this.loupe.reset()
    this.sizes = {}
    this.lost = {}
    this.carryFrom.clear()
    this.origs.clear()
    this.busy = false
  }
  private visibleIds = (): string[] => store.visible.map((p) => p.id)

  // C в ленте: по выделению (2–4 — они; один — он и соседний; иначе тост).
  openByKey(): boolean {
    return this.start(openBySelection(this.visibleIds(), store.selectedIds), store.selectedIds.length)
  }
  // C из окна 128: показанный кадр и соседний по замороженному списку окна.
  openFrom(list: readonly string[], shown: string, before?: () => void): boolean {
    return this.start(openPair(list, shown), 0, before)
  }
  // Shift+C в ленте: похожие на якорь (если он выделен) или первый выделенный.
  similarByKey(): Promise<boolean> {
    const ids = store.selectedIds
    const id = store.anchor && ids.includes(store.anchor) ? store.anchor : ids[0]
    if (!id) {
      store.notify(WHY_TEXT.none(0))
      return Promise.resolve(false)
    }
    return this.similar(id, this.visibleIds())
  }
  // «Похожие к этому»: Go ищет соседей по ленте list (SimilarTo); кадр и до 3 ближайших.
  async similar(origin: string, list: readonly string[], before?: () => void): Promise<boolean> {
    const n = ++this.seq
    let r: Awaited<ReturnType<typeof Api.SimilarTo>>
    try {
      r = await Api.SimilarTo(origin, [...list])
    } catch (e) {
      store.notify(errText(e), 'error')
      return false
    }
    if (n !== this.seq || this.st) return false // открыли другое, пока ждали
    if (r.why === 'nosig') {
      store.notify(SIMILAR_TEXT.nosig)
      return false
    }
    const s = r.why === 'none' ? 'alone' : openIds(list, r.ids ?? [], origin)
    if (typeof s === 'string') {
      store.notify(SIMILAR_TEXT.none)
      return false
    }
    if ((r.more ?? 0) > 0) store.notify(SIMILAR_TEXT.more(s.cells.length - 1 + r.more))
    return this.start(s, 0, before)
  }

  // ── окно ──
  pick(i: number) {
    if (this.st && i >= 0 && i < this.st.cells.length && i !== this.st.active) this.st = { ...this.st, active: i }
  }
  // Tab / Shift+Tab: активной становится следующая ячейка по кругу.
  cycle(d: 1 | -1) {
    const n = this.ids.length
    if (n > 1) this.pick((this.active + d + n) % n)
  }
  // ← →: кадр в активной ячейке меняется на соседний по списку; зум остаётся по правилу 128 (carry) после загрузки.
  step(d: 1 | -1) {
    const s = this.st
    if (!s) return
    const next = stepCell(s, d, (id) => store.byId.has(id))
    if (next === s) return
    const old = activeId(s)
    const now = activeId(next)
    const prev = this.sizes[old]
    if (prev) this.carryFrom.set(now, prev)
    this.st = next
  }
  // Превью загрузилось: запоминаем размер, при листании переносим зум.
  loaded(id: string, size: Size) {
    this.sizes = { ...this.sizes, [id]: size }
    const prev = this.carryFrom.get(id)
    if (prev) {
      this.carryFrom.delete(id)
      this.loupe.carry(id, prev, size)
    }
  }
  fail(id: string) {
    this.lost = { ...this.lost, [id]: true }
  }
  // ✕ или Delete: ячейка уходит, отметки не меняются; остался один кадр — окно закрывается.
  remove(at = this.active) {
    const s = this.st
    if (!s) return
    this.loupe.forget(s.cells[at])
    const next = removeCell(s, at)
    if (next === s) return
    if (isLastLeft(next)) this.close('last', next)
    else this.st = next
  }
  // Кадры, ушедшие из альбома, пропадают вместе с ячейкой.
  prune() {
    const s = this.st
    if (!s) return
    const next = dropGone(s, (id) => store.byId.has(id))
    if (next === s) return
    if (isLastLeft(next)) this.close('last', next)
    else this.st = next
  }

  // «Лучший»: активному «отобрано», остальным — «отклонено» (restForBest), один шаг отмены.
  async best(): Promise<boolean> {
    const s = this.st
    if (!s || !this.canBest || this.busy) return false
    const id = activeId(s)
    const rest = restForBest(s.cells.filter((c) => c !== id))
    this.busy = true
    const ok = await store.run(Api.PickBest(id, rest))
    this.busy = false
    if (!ok) return false
    this.close('best')
    store.notify(`Лучший — ${store.byId.get(id)?.name ?? ''}; отклонено: ${rest.length}` + keymap.text('{key:undo| (% — вернуть)}'))
    return true
  }

  // Закрыть. Выделение ленты — оставшиеся видимые кадры (после «Лучший» — только он), якорь — активный;
  // лента прокручивается к активному или ближайшему видимому (механизм 128, photoui.reveal).
  close(why: 'best' | 'other' | 'last' = 'other', state: CompareState | null = this.st) {
    const s = state
    if (!s) return
    const vis = new Set(store.visible.map((p) => p.id))
    const keep = why === 'best' ? { ...s, cells: [activeId(s)], active: 0 } : s
    const pick = closeSelection(keep, vis)
    if (pick.ids.length > 0) {
      store.selected.clear()
      for (const x of pick.ids) store.selected.add(x)
      store.anchor = pick.anchor
    }
    const to = nearestVisible({ ids: s.list, at: Math.max(0, s.list.indexOf(activeId(s))), from: 'library' }, vis)
    if (to) photoui.revealTo(to)
    this.seq++
    this.st = null
    this.reset()
  }
}

export const compare = new Compare()
