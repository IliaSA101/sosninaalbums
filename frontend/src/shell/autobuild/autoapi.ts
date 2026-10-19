// Вызовы Go окна «Автосборка»: сборка AutoOpts из полей окна. Состояние — autobuild.svelte.ts, блок «Фото» — autosimilar.svelte.ts.
import { model } from '../../../wailsjs/go/models'
import * as Api from '../../../wailsjs/go/app/App'
import { store } from '../../base/core/store.svelte'
import type { PickResult } from '../../base/core/hooks'
import { modes } from '../../base/prefs/view.svelte'
import type { AutoRange, Family, PhotoOpts, Variety } from '../../base/core/types'
import type { DropsFn, SigsFn } from './autosimilar.svelte'
import type { AutoDialog, PlanFn, PreviewFn } from './autobuild.svelte'
import { memOf } from './autobuild'

export const optsOf = (spreads: number, range: AutoRange, variety: Variety, families: readonly Family[], ph: PhotoOpts): model.AutoOpts =>
  model.AutoOpts.createFrom({ spreads, ...range, variety, families: [...families], ...ph, keep: [...ph.keep] })

// Разнообразие и семейства на план не влияют (группы те же), но поле контракта заполняем запомненным
export const planFn =
  (variety: () => Variety): PlanFn =>
  (ids, spreads, range, ph) =>
    Api.AutoPlan([...ids], optsOf(spreads, range, variety(), [], ph))

// 065: ряд «Раскладки» — то же, что соберёт «Собрать»: вставка в конец альбома, как у store.autoBuild
// Пересборка (078): позиция первой сборки — считается от снимка до неё
export const previewFn: PreviewFn = (ids, o) =>
  Api.AutoPreview(
    [...ids],
    (o.ph.again ? store.lastAuto?.at : undefined) ?? store.view.spreads.length,
    optsOf(o.spreads, o.range, o.variety, o.families ?? [], o.ph),
  )

// 078: кого убирают как похожие; число разворотов на ответ не влияет
export const dropsFn: DropsFn = (ids, range, ph) => Api.AutoSimilar([...ids], optsOf(1, range, 1, [], ph))

export const sigsFn: SigsFn = (ids) => Api.PrepareSigs([...ids])

// Начальные значения окна: память окна; в пересборке — настройки последней сборки, вариант на 1 больше (078, design §9.3)
export function savedOf(d: PickResult, famOn: boolean, grayOn: boolean): ConstructorParameters<typeof AutoDialog>[2] {
  const mem = {
    spreads: modes.autoSpreads,
    range: modes.autoRange,
    series: modes.autoSeries,
    per: modes.autoPer,
    by: modes.autoBy,
    variety: modes.autoVariety,
    families: famOn ? modes.autoFamilies : [],
    similar: modes.autoSimilar,
    gray: modes.autoGray,
  }
  const la = d.again ? store.lastAuto : null
  if (!la) return mem
  const o = la.opts
  return {
    ...mem,
    spreads: o.spreads,
    range: memOf(o.range),
    series: o.range.series,
    variety: o.variety,
    families: famOn ? [...o.families] : [],
    similar: o.ph.similar,
    gray: grayOn ? o.ph.gray : mem.gray,
    keep: o.ph.keep,
    variant: la.variant + 1,
    again: true,
  }
}
