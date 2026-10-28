// Слой вызовов Go для окна «Макеты альбома» (131 U1, U4): тонкая обёртка над биндингами, остальной код области зовёт
// только `lookApi`. Каталог, ✎ и × — общие методы библиотеки (вид 'album'); числа, план и постановку считает Go.
// planSave — сухой прогон SaveLook: ok=false и reason (текст ошибки Go) вместо отклонённого промиса; ничего не пишет.
// save — SaveLook: снимок альбома в свои макеты темы; альбом не меняется, отмены нет.
import { Api } from '../../base/core/store.svelte'
import type { GridPlan, View } from '../../base/core/types'
import type { TplResult } from '../../panels/decor/decor'
import type { LookCatalog, LookCount, LookInfo, LookItem, LookOpts, LookPlan, LookPlanSave, LookResult, SaveLookOpts } from './looktypes'

const as = <T>(p: Promise<unknown>) => p as Promise<T>

export const lookApi = {
  catalog: () => as<LookCatalog>(Api.DecorCatalog('album')),
  info: (id: string) => as<LookInfo>(Api.LookInfo(id)),
  // Серия из n разворотов: сколько их на деле, мест и сколько разворотов нужно под k фото.
  count: (id: string, n: number, k: number) => as<LookCount>(Api.LookCount(id, n, k)),
  // Сухой прогон на копии: альбом не меняется. spreads в ответе — «разворотов макета после постановки».
  plan: (id: string, o: LookOpts) => as<LookPlan>(Api.PlanLook(id, o)),
  // Одна правка, один шаг отмены; view идёт в store.run (цепочка вида).
  apply: (id: string, o: LookOpts) => as<LookResult>(Api.ApplyLook(id, o)),
  // «Поставить на разворот N» (design §8): сухой прогон 067 и сама постановка. step 0 — обложка, spread 'cover'.
  planStep: (spread: string, id: string, step: number) => as<GridPlan>(Api.PlanLookStep(spread, id, step)),
  applyStep: (spread: string, id: string, step: number) => as<TplResult>(Api.ApplyLookStep(spread, id, step)),
  // Правый щелчок по «+» ▸ «Разворот макета «…»»: следующий повторяемый шаг перед концом серии.
  newSpread: () => as<View>(Api.NewLookSpread()),
  rename: (id: string, name: string): Promise<void> => Api.RenameDecor('album', id, name),
  hide: (id: string): Promise<void> => Api.HideDecor('album', id),
  // Ошибки Go (больше 24 разных, пустое имя, нечего снять, лист-проект) идут как есть — окно покажет красный тост и останется.
  planSave: (o: SaveLookOpts) => as<LookPlanSave>(Api.PlanSaveLook(o)),
  save: (o: SaveLookOpts) => as<LookItem>(Api.SaveLook(o)),
}
