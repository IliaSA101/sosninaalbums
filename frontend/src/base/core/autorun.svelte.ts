import { SvelteSet } from 'svelte/reactivity'
import * as Api from '../../../wailsjs/go/app/App'
import { model } from '../../../wailsjs/go/models'
import { keymap } from '../keys/keymap.svelte'
import { unitOf } from '../geom/sheet'
import { withSimilar, withSkipped } from '../words/errtext'
import type { PickResult, StoreHooks } from './hooks'
import type { Toast } from './toasts.svelte'
import type { AutoRange, Family, PhotoOpts, Variety, View } from './types'

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

const NO_PHOTOS: PhotoOpts = { similar: 0, keep: [], gray: 0, variant: 1, again: false }
export const STALE_AGAIN = 'После сборки альбом менялся — пересобрать нельзя. Отменить сборку'
export const SAME_VARIANT = 'Этот вариант не отличается — попробуйте следующий'
export const againDone = (variant: number): string => `Пересобрано: вариант ${variant}`

// Что автосборке нужно от Store (структурный тип: цикла импортов со store нет).
export interface AutoHost {
  readonly view: View
  readonly hooks: StoreHooks
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

  constructor(private h: AutoHost) {}

  // Пересборка жива: Go считает по версиям проекта, UI ревизию не сравнивает
  get alive(): boolean {
    return !!this.last && this.h.view.again === true
  }

  // Другой альбом — прошлая сборка ему не принадлежит
  reset(): void {
    this.last = null
  }

  // «Автосборка»: выбрать фото и открыть окно; пусто — тост с причиной уже показан. Выбора нет, а пересборка жива
  // (все фото уже в альбоме) — открываем её вместо тоста «нет фото» (сц. 12).
  open(): void {
    const h = this.h
    if (h.busy) return
    const p = h.hooks.pick(true)
    if (p.ids.length > 0) {
      h.autoDraft = p
      return
    }
    if (!this.alive) return
    h.toast = null
    this.openAgain()
  }

  // «Пересобрать…»: те же фото последней сборки без отклонённых после неё; не жива — тост сц. 13
  openAgain(): void {
    const h = this.h
    const l = this.last
    if (h.busy) return
    if (!l || h.view.again !== true) return h.notify(keymap.tip(STALE_AGAIN, 'undo'))
    const ids = h.hooks.notRejected(l.pick.ids)
    if (ids.length === 0) return h.notify('Все фото сборки отклонены — пересобирать нечего')
    h.autoDraft = { ...l.pick, ids, again: true }
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
  ): Promise<boolean> {
    const h = this.h
    // Выделенные — как есть: Go переносит уже стоящие в альбоме фото в новые развороты.
    const d = h.autoDraft
    h.autoDraft = null
    if (!d || d.ids.length === 0) return false
    const again = ph?.again === true
    // Пересборка идёт от снимка до первой сборки — позиция та же, что тогда
    const at = again && this.last ? this.last.at : h.view.spreads.length
    const have = again && this.last ? this.last.have : h.view.spreads.length
    const before = new SvelteSet(h.view.spreads.map((s) => s.id))
    const rev = h.view.rev
    h.busy = true
    let ok: boolean
    try {
      const opts = model.AutoOpts.createFrom({ spreads, ...range, variety, families: [...families], ...ph })
      ok = await h.run(Api.AutoBuild([...d.ids], at, opts))
    } finally {
      h.busy = false
    }
    if (!ok) return false
    h.selected.clear()
    const used = ph ?? NO_PHOTOS
    this.last = {
      pick: { ...d, again: false },
      at,
      have,
      opts: { spreads, range, variety, families: [...families], ph: { ...used, keep: [...used.keep], again: false } },
      variant: used.variant,
      rev: h.view.rev,
    }
    const more = { label: 'Пересобрать…', run: () => this.openAgain() }
    if (again) {
      // Развороты прошлого варианта сохраняют ID: переход не прыгает
      h.notify(h.view.rev === rev ? SAME_VARIANT : againDone(used.variant), 'info', more)
      return true
    }
    h.goNew(before, at)
    const n = h.view.spreads.filter((s) => !before.has(s.id)).length
    h.notify(withSkipped(withSimilar(`Собрано ${unitOf(h.view.settings).plural[2]}: ${n}`, dropped), d.skipped), 'info', more)
    return true
  }
}
