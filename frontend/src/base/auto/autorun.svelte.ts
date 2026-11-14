import { SvelteSet } from 'svelte/reactivity'
import * as Api from '../../../wailsjs/go/app/App'
import { model } from '../../../wailsjs/go/models'
import { keymap } from '../keys/keymap.svelte'
import { unitOf } from '../geom/sheet'
import { plural } from '../words/plural'
import { withSimilar, withSkipped } from '../words/errtext'
import type { PickResult, StoreHooks } from '../core/hooks'
import type { Toast } from '../core/toasts.svelte'
import type { AutoRange, Family, Photo, PhotoOpts, Variety, View } from '../core/types'
import { albumIds } from './albumpick'

// Последняя автосборка (078, design §9.3): с чем пересобирать. Живость («Пересобрать…» ещё можно) считает Go (View.again),
// rev — после ответа, для справки: звёзды и метки пересборку не закрывают, поэтому по rev её не гасим.
export interface LastAuto {
  pick: PickResult
  at: number // позиция вставки первой сборки: пересборка считается от снимка до неё
  have: number // сколько разворотов было в альбоме до первой сборки (строка «В альбоме станет…» при пересборке)
  opts: { spreads: number; range: AutoRange; variety: Variety; families: readonly Family[]; ph: PhotoOpts }
  variant: number
  rev: number
}

export interface SameRun {
  first: number
  last: number
  next: number
}

const NO_PHOTOS: PhotoOpts = { similar: 0, keep: [], gray: 0, variant: 1, again: false }
export const STALE_AGAIN = 'После сборки альбом менялся — пересобрать нельзя. Отменить сборку'
export const SAME_VARIANT = 'Этот вариант не отличается — попробуйте следующий'
export const againDone = (variant: number): string => `Пересобрано: вариант ${variant}`

// Снимок сборки для пересборки: без alt окна, признак пересборки — не часть настроек
function snapshot(d: PickResult, at: number, have: number, o: LastAuto['opts'], rev: number): LastAuto {
  return {
    pick: { ...d, again: false, alt: undefined, base: undefined },
    at,
    have,
    opts: { ...o, ph: { ...o.ph, keep: [...o.ph.keep], again: false } },
    variant: o.ph.variant,
    rev,
  }
}

// Запрос в Go; режим альбома (107) помечает его album: true (прежние сборки поле не получают)
function optsOf(
  d: PickResult,
  spreads: number,
  range: AutoRange,
  variety: Variety,
  families: readonly Family[],
  ph?: PhotoOpts,
): model.AutoOpts {
  const album = d.from === 'album' ? { album: true } : {}
  return model.AutoOpts.createFrom({ spreads, ...range, variety, families: [...families], ...ph, ...album })
}

// Сколько разворотов назвать в тосте пересборки альбома (N окна из плана); обычная сборка считает новые по ID — null
const albumCount = (d: PickResult, spreads: number): number | null => (d.from === 'album' ? spreads : null)

// Что автосборке нужно от Store (структурный тип: цикла импортов со store нет).
export interface AutoHost {
  readonly view: View
  readonly hooks: StoreHooks
  readonly ordered: readonly Photo[] // вся лента в порядке сортировки (107: фото альбома по ленте)
  readonly used: { has(id: string): boolean }
  readonly selected: { clear(): void }
  busy: boolean
  autoDraft: PickResult | null
  toast: Toast | null
  notify(text: string, kind?: Toast['kind'], action?: Toast['action']): void
  run(p: Promise<unknown>): Promise<boolean>
  goNew(before: Set<string>, fallback: number): void
}

// Окно «Автосборка» и «Пересобрать…»: выбор фото, сама сборка, тост. Состояние окна (autoDraft) живёт в Store.
export class AutoRun {
  last = $state.raw<LastAuto | null>(null)
  // 080-1: окно пересборки ждёт ответа Go (кнопка «Пересобираю…»)
  waiting = $state(false)
  // 080-1: варианты first…last не отличаются от текущего, окно показало next
  same = $state.raw<SameRun | null>(null)

  constructor(private h: AutoHost) {}

  // Пересборка жива: Go считает по версиям проекта, UI ревизию не сравнивает
  get alive(): boolean {
    return !!this.last && this.h.view.again === true
  }

  // Другой альбом — прошлая сборка ему не принадлежит
  reset(): void {
    this.last = null
  }

  // «Автосборка»: выбрать фото и открыть окно; пусто — тост с причиной уже показан (080-3).
  // Выделенные — новая сборка (ссылка «Пересобрать прошлую»); свободные после сборки свободных — пересборка
  // (ссылка «Собрать свободные»); пересборка из выделенных — новая сборка свободных; выбора нет, а пересборка жива —
  // она вместо тоста «нет фото» (сц. 12 078); пересборки нет, а развороты есть — «Весь альбом заново» (107, сц. 9).
  open(): void {
    const h = this.h
    if (h.busy) return
    const p = h.hooks.pick(true)
    // Пересборка альбома ссылкой не предлагается: к ней ведёт строка «Собрать» (107)
    const again = this.againDraft()
    const alt = again?.from === 'album' ? undefined : again
    this.same = null
    if (p.from === 'selected' && p.ids.length > 0) h.autoDraft = { ...p, alt }
    else if (again && this.last?.pick.from === 'free') {
      if (p.ids.length === 0) h.toast = null // тост «нет фото» от pick уже не нужен
      h.autoDraft = { ...again, alt: p.ids.length > 0 ? p : undefined }
    } else if (p.ids.length > 0) h.autoDraft = { ...p, alt }
    else if (this.alive) {
      h.toast = null
      this.openAgain()
    } else if (h.view.spreads.length > 0) {
      if (p.skipped === 0) h.toast = null // тост «нет фото» от pick уже не нужен; «Отклонённые не взяты» остаётся
      h.autoDraft = this.albumDraft()
    }
  }

  // Окно в режиме «Весь альбом заново» (107). Пересборка альбома жива — сразу она (вариант +1, без вопроса);
  // иначе новая: фото альбома по ленте, свободные — галочкой (выключена). base — выбор, к которому вернёт строка «Собрать».
  private albumDraft(base?: PickResult): PickResult {
    const again = this.last?.pick.from === 'album' ? this.againDraft() : undefined
    if (again) return { ...again, base }
    const freeIds = this.h.hooks.pickFree()
    const ids = albumIds(this.h.ordered, this.h.used, freeIds, false)
    return { ids, skipped: 0, from: 'album', inAlbum: 0, filtered: false, freeIds, addFree: false, base }
  }

  // Строка «Собрать: …»: сменить источник — новое состояние окна, как ссылка 080-3. Повторный щелчок по выбранному — ничего.
  setSource(to: 'base' | 'album'): void {
    const d = this.h.autoDraft
    if (!d || this.waiting || this.h.busy) return
    if (to === 'album' && d.from !== 'album' && this.h.view.spreads.length > 0) this.h.autoDraft = this.albumDraft(d)
    else if (to === 'base' && d.from === 'album' && d.base) this.h.autoDraft = d.base
    else return
    this.same = null
  }

  // Галочка «Добавить свободные фото ленты»: другой состав фото — новое состояние окна
  setAddFree(on: boolean): void {
    const d = this.h.autoDraft
    if (!d || d.from !== 'album' || d.again || this.waiting || this.h.busy || !!d.addFree === on) return
    this.h.autoDraft = { ...d, addFree: on, ids: albumIds(this.h.ordered, this.h.used, d.freeIds ?? [], on) }
  }

  // Пересборка без отклонённых после сборки; нет (не жива, все отклонены) — undefined
  private againDraft(): PickResult | undefined {
    const l = this.last
    if (!l || this.h.view.again !== true) return undefined
    const ids = this.h.hooks.notRejected(l.pick.ids)
    // Альбом ищет фото сам: пустой список для него не помеха
    return ids.length > 0 || l.pick.from === 'album' ? { ...l.pick, ids, again: true } : undefined
  }

  // «Пересобрать…»: те же фото последней сборки без отклонённых после неё; не жива — тост сц. 13
  openAgain(): void {
    const h = this.h
    if (h.busy) return
    if (!this.last || h.view.again !== true) return h.notify(keymap.tip(STALE_AGAIN, 'undo'))
    const d = this.againDraft()
    if (!d) return h.notify('Все фото сборки отклонены — пересобирать нечего')
    this.same = null
    h.autoDraft = d
  }

  // Ссылка в строке источника: окно открывается заново для другого набора фото; пока идёт пересборка — нельзя
  swap(): void {
    const d = this.h.autoDraft
    if (!d?.alt || this.waiting || this.h.busy) return
    this.same = null
    this.h.autoDraft = { ...d.alt, alt: { ...d, alt: undefined } }
  }

  // «Собрать» / «Пересобрать»: ровно `spreads` разворотов, фото на каждом — по `range` («Авто» или «от … до …», серии; 051),
  // разнообразие раскладок — variety (041), разрешённые семейства — families (065; [] — все); ph — похожие, ч/б, вариант и
  // признак пересборки (078), dropped — сколько похожих кадров окно не берёт (только для тоста: убирает их Go).
  async build(
    spreads: number,
    range: AutoRange,
    variety: Variety = 1,
    families: readonly Family[] = [],
    ph?: PhotoOpts,
    dropped = 0,
    show?: (variant: number) => void, // окно пересборки открыто, вариант совпал: какой показать следующим
  ): Promise<boolean> {
    const h = this.h
    // Выделенные — как есть: Go переносит уже стоящие в альбоме фото в новые развороты.
    const again = ph?.again === true
    const d = this.take(again)
    if (!d) return false
    const { at, have } = this.origin(again)
    const before = new SvelteSet(h.view.spreads.map((s) => s.id))
    const rev = h.view.rev
    const opts = optsOf(d, spreads, range, variety, families, ph)
    const ok = await this.send(d.ids, at, opts, again)
    // Окно ещё открыто (✕ во время ожидания закрыло бы его: тогда ответ — тостом, как в 0.20)
    const open = again && h.autoDraft === d
    if (open && (!ok || h.view.rev !== rev)) h.autoDraft = null // изменилось или ошибка: окно закрыто
    if (!ok) return false
    h.selected.clear()
    const used = ph ?? NO_PHOTOS
    if (open && h.autoDraft === d) {
      // Вариант совпал с текущим: вариант не поставлен, last прежний; окно покажет следующий (сц. 2)
      show?.(this.noteSame(used.variant))
      return true
    }
    this.last = snapshot(d, at, have, { spreads, range, variety, families: [...families], ph: used }, h.view.rev)
    const fin = (t: string) => withSkipped(withSimilar(t, dropped), d.skipped)
    this.toastDone(again, used.variant, h.view.rev === rev, before, at, fin, albumCount(d, spreads))
    return true
  }

  // Тост после сборки: «Пересобрано…» (или «не отличается», если окно закрыли) либо «Собрано разворотов: N» с «Пересобрать…»
  private toastDone(
    again: boolean,
    variant: number,
    same: boolean,
    before: Set<string>,
    at: number,
    fin: (text: string) => string,
    album: number | null, // 107: сколько разворотов пересобрано (N из плана окна)
  ): void {
    const h = this.h
    const more = { label: 'Пересобрать…', run: () => this.openAgain() }
    if (again) {
      // Развороты прошлого варианта сохраняют ID: переход не прыгает
      h.notify(same ? SAME_VARIANT : againDone(variant), 'info', more)
      return
    }
    if (album !== null) {
      // Развороты сохраняют ID: текущий остаётся на месте, переходить некуда
      h.notify(fin(`Альбом пересобран: ${album} ${plural(album, unitOf(h.view.settings).plural)}`), 'info', more)
      return
    }
    h.goNew(before, at)
    const n = h.view.spreads.filter((s) => !before.has(s.id)).length
    h.notify(fin(`Собрано ${unitOf(h.view.settings).plural[2]}: ${n}`), 'info', more)
  }

  // Фото окна. Пересборка ждёт ответа при открытом окне (080-1): холостой вариант покажет следующий, а не закроет окно
  private take(again: boolean): PickResult | null {
    // Запрос уже идёт (двойной Enter в окне пересборки: busy ставится в send до первого await): второй не уходит
    if (this.h.busy) return null
    const d = this.h.autoDraft
    const ok = !!d && (d.ids.length > 0 || d.from === 'album')
    if (!again || !ok) this.h.autoDraft = null
    return ok ? d : null
  }

  // Пересборка идёт от снимка до первой сборки — позиция та же, что тогда
  private origin(again: boolean): { at: number; have: number } {
    const l = again ? this.last : null
    const n = this.h.view.spreads.length
    return { at: l ? l.at : n, have: l ? l.have : n }
  }

  // Запрос в Go; у пересборки окно открыто и ждёт (waiting)
  private async send(ids: readonly string[], at: number, opts: model.AutoOpts, again: boolean): Promise<boolean> {
    this.h.busy = true
    this.waiting = again
    try {
      return await this.h.run(Api.AutoBuild([...ids], at, opts))
    } finally {
      this.h.busy = false
      this.waiting = false
    }
  }

  // Вариант совпал: серия подряд идущих (следующий после совпавшего) растёт; возвращает вариант для показа
  private noteSame(variant: number): number {
    const s = this.same
    const first = s && s.next === variant ? s.first : variant
    this.same = { first, last: variant, next: variant + 1 }
    return variant + 1
  }
}
