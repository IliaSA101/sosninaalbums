// Действия с виньеткой (031): вызовы Go и тосты. Ответ — в store.apply, ошибка Go (в том числе «не больше
// 64 мест») — тост. Используют панель, меню разворота, карточка, «Правка» и колонка «+ Новый …» ленты.
import { Api, store } from '../../base/core/store.svelte'
import { errText } from '../../base/words/errtext'
import { model } from '../../../wailsjs/go/models'
import { unitOf } from '../../base/geom/sheet'
import { gridAct } from '../layouts/gridact.svelte'
import type { GridResult, SpreadView, VigChoice, VigPreview, VigSetup } from '../../base/core/types'
import type { VigFamilyFilter } from '../../base/tpl/tplfilter'
import { stepTile, vigTiles, pickTile, type FormFilter } from './vigform'
import { formOf, isVignette, leftText, paramsOf, startForm, type VigForm } from './vignette'

// Карточки шаблонов для чисел и пропорции (превью считает Go). Ошибку Go отдаёт вызывающему.
export async function vigChoices(spreadId: string, f: Pick<VigForm, 'teachers' | 'pupils' | 'ratio'>): Promise<VigChoice[]> {
  const list = await Api.VignetteChoices(spreadId, f.teachers, f.pupils, f.ratio)
  return (list ?? []) as unknown as VigChoice[]
}

// Сухой прогон окна «Настроить виньетку»: лист после «Применить», в проект ничего не пишется, шага отмены нет.
// Отказ Go (больше 64 мест, нет запаса надписей) — исключение с текстом: окно показывает его у себя, не тостом.
// ID новых мест в ответе случайные (в итоге будут другие) — ничего не привязывать к ним.
export const previewVignette = (spreadId: string, s: VigSetup): Promise<VigPreview> =>
  Api.PreviewVignette(spreadId, model.VigSetup.createFrom(s)) as unknown as Promise<VigPreview>

// «Применить» окна: параметры, места, фото и поля шапки — один шаг отмены. Ошибка Go — тост, окно остаётся.
export async function applySetup(spreadId: string, s: VigSetup): Promise<boolean> {
  try {
    const r = (await Api.SetupVignette(spreadId, model.VigSetup.createFrom(s))) as unknown as GridResult
    store.apply(r.view)
    if (r.left > 0) store.undoToast(leftText(r.left))
    return true
  } catch (e) {
    store.notify(errText(e), 'error')
    return false
  }
}

// «Применить» и «Сделать виньеткой»: фото, не вставшие в места своей роли, уходят в ленту — тост с «Вернуть».
// done — текст тоста, который нужен всегда (превращение разворота): «Вернуть» отменяет этот один шаг SetVignette.
export async function applyVignette(spreadId: string, f: VigForm, done = ''): Promise<boolean> {
  try {
    const r = (await Api.SetVignette(spreadId, model.Vignette.createFrom(paramsOf(f)))) as unknown as GridResult
    store.apply(r.view)
    const left = r.left > 0 ? leftText(r.left) : ''
    if (done) store.undoToast(left ? `${done}. ${left}` : done, spreadId)
    else if (left) store.undoToast(left)
    return true
  } catch (e) {
    store.notify(errText(e), 'error')
    return false
  }
}

// Обычный разворот → виньетка: фото по порядку садятся в места учеников. Тост с «Вернуть» — всегда (отзыв 14.10):
// кнопка в карточке и пункт меню меняют вид листа целиком, путь назад должен быть на виду.
export const makeVignette = (sp: SpreadView): Promise<boolean> =>
  applyVignette(sp.id, startForm(sp.photos.length), `${unitOf(store.view.settings).cap} стал виньеткой`)

// Новая виньетка по умолчанию на место at (колонка «+ Новый …»); лента переходит к ней.
let adding = false
export async function addVignette(at: number): Promise<void> {
  if (adding) return
  adding = true
  try {
    const before = new Set(store.view.spreads.map((s) => s.id))
    if (!(await store.run(Api.AddVignette(at)))) return
    const i = store.view.spreads.findIndex((s) => !before.has(s.id))
    store.go(i >= 0 ? i : at)
  } finally {
    adding = false
  }
}

// «Обычный разворот»: места, фото, подписи и шапка остаются, уходят только роли и параметры.
export const clearVignette = (spreadId: string): Promise<boolean> => store.run(Api.ClearVignette(spreadId))

// Смена шаблона и формы на листе (‹ ›, Пробел, плитка в ▦): patch — что ставит плитка (pickTile: шаблон, форма, у своего
// ещё портрет), остальное — как на листе. Ручная виньетка сначала спрашивает «Ваши правки мест сбросятся…»
// (gridAct.vigGate), как страница; false — отказано Go или ждёт ответа.
export async function changeTpl(spreadId: string, f: VigForm, patch: Partial<VigForm>): Promise<boolean> {
  let ok = false
  await gridAct.vigGate(spreadId, 'tpl', async () => {
    ok = await applyVignette(spreadId, { ...f, ...patch })
  })
  return ok
}

// Пробел и стрелки вариантов на виньетке: следующая / предыдущая видимая плитка (шаблон или форма) с числами, что стоят
// на листе (не из формы панели: несохранённая форма на лист не влияет). Пока идёт шаг, новые нажатия пропускаются.
// family и form — фильтры открытого ▦: листаем только видимое, как стрелки в окне «Настроить виньетку».
let stepping = false
export async function stepVignette(
  sp: SpreadView,
  delta: number,
  family: VigFamilyFilter = 'all',
  form: FormFilter = 'all',
): Promise<boolean> {
  if (stepping || !isVignette(sp)) return false
  stepping = true
  try {
    const f = formOf(sp.vig)
    const list = await vigChoices(sp.id, f)
    const t = stepTile(vigTiles(list, family, form, f), f, delta)
    if (!t) {
      store.notify('Других шаблонов для этих чисел нет')
      return false
    }
    return await changeTpl(sp.id, f, pickTile(t, f.ratio))
  } catch (e) {
    store.notify(errText(e), 'error')
    return false
  } finally {
    stepping = false
  }
}

// «Сортировать по фамилии»: ученики по (Фамилия, Имя). Ничего не изменилось — говорим, чтобы кнопка не казалась сломанной.
export async function sortVignette(spreadId: string): Promise<boolean> {
  const before = store.view.rev
  const ok = await store.run(Api.SortVignette(spreadId))
  if (ok && store.view.rev === before) store.notify('Ученики уже по фамилии: менять нечего')
  return ok
}
