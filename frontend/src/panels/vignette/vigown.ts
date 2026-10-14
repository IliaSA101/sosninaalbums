// Свои шаблоны виньетки (038): чистая логика и вызовы Go для окна «Настроить виньетку», галереи и карточки.
// Шаблоны хранит Go (templates.json, общий для компьютера); здесь тексты, условия кнопок и тосты.
import { Api, store } from '../../base/core/store.svelte'
import { errText } from '../../base/core/errtext'
import type { GridResult, VigChoice } from '../../base/core/types'
import { tplCall } from '../layouts/gridactions'
import { tplError } from '../layouts/gridgallery'
import { RATIOS, choiceOf, leftText, portraitLines, type Ratio } from './vignette'

export const OWN_TITLE = 'Мои шаблоны'
export const NAME_MAX = 60
export const SAVE_HINT = 'Сохранятся места с ролями, шапка и стиль подписей. Шаблон хранится на этом компьютере, общий для всех альбомов'

// Индекс первого своего шаблона (перед ним — заголовок «Мои шаблоны»); своих нет — −1.
export const firstOwn = (items: readonly Pick<VigChoice, 'own'>[]): number => items.findIndex((c) => c.own)

// Название плитки: удалённый из файла шаблон помечен.
export const ownLabel = (c: Pick<VigChoice, 'name' | 'gone'>): string => (c.gone ? `${c.name} (шаблон удалён)` : c.name)

// Имя шаблона листа для карточки справа и строки раскладки: по списку Go (переименование видно сразу),
// tplName на листе — снимок на момент применения, запасной, пока списка нет.
export const tplLabel = (items: readonly VigChoice[], tpl: string, fallback?: string): string => {
  const c = choiceOf(items, tpl)
  return c ? ownLabel(c) : (fallback ?? '')
}

// Свой шаблон: его имя можно изменить, поэтому строке раскладки нужен список Go.
export const isOwnTpl = (tpl: string): boolean => tpl.startsWith('my:')

// ✎ и × — только у своего, живого шаблона и при уровне «Шаблоны»: удалённого из файла переименовывать нечего.
export const canEditOwn = (c: Pick<VigChoice, 'own' | 'gone'>, allowed: boolean): boolean => allowed && !!c.own && !c.gone

// Строки под плиткой галереи: ширина портрета и пометка «удалён».
export const tileLines = (c: VigChoice): string[] => (c.gone ? [...portraitLines(c), 'удалён'] : portraitLines(c))

// Выбор своего шаблона ставит в окне его портрет (точный режим требует того же); числа не трогает.
// null — менять нечего (каталожный шаблон, портрет тот же, не пришёл или неизвестный).
export function pickedRatio(items: readonly VigChoice[], id: string, cur: string): Ratio | null {
  const c = items.find((x) => x.id === id)
  const r = c?.ratio as Ratio | undefined
  return c?.own && r && r !== cur && RATIOS.includes(r) ? r : null
}

// Текст вопроса об удалении; cap — «Лист» или «Разворот» (unitOf.cap).
export const deleteText = (name: string, cap: string): string =>
  `Шаблон «${name}» будет удалён из списка. ${cap === 'Лист' ? 'Листы' : 'Развороты'}, где он уже применён, не изменятся.`

// Имя из поля: пробелы по краям убираются, длиннее NAME_MAX обрезается (Go зажимает так же).
export const nameOf = (raw: string): string => Array.from(raw.trim()).slice(0, NAME_MAX).join('')

// «Сохранить как шаблон…»: пустое имя — Go назовёт «Шаблон N». Успех — тост; ошибка Go — тост текста.
export async function saveVigTemplate(spreadId: string, name: string): Promise<boolean> {
  if (!(await tplCall(Api.SaveVignetteTemplate(spreadId, nameOf(name))))) return false
  store.notify('Шаблон сохранён')
  return true
}

// Id шаблона — VigChoice.id как есть («my:<id>»), префикс снимает Go.
export const renameVigTemplate = (id: string, name: string): Promise<boolean> => tplCall(Api.RenameGridTemplate(id, nameOf(name)))
export const deleteVigTemplate = (id: string): Promise<boolean> => tplCall(Api.DeleteGridTemplate(id))

// «Вернуть авто» в строке раскладки виньетки: места пересобираются по шаблону, один шаг отмены.
// Фото, что не встали, уходят в ленту — тост с «Вернуть», как у «Применить».
export async function resetVignette(spreadId: string): Promise<boolean> {
  try {
    const r = (await Api.ResetVignette(spreadId)) as unknown as GridResult
    store.apply(r.view)
    if (r.left > 0) store.undoToast(leftText(r.left))
    return true
  } catch (e) {
    store.notify(tplError(errText(e)), 'error')
    return false
  }
}
