// Щелчок по плитке «Можно скачать» (144, design §7): скачать набор и поставить выбранное. Ход, тост «скачан» и «Отменить» —
// те же, что у «Скачать» в окне «Наборы» (packsUi.net.fetchOne). Ставится только то, что окно ещё ждёт: окно закрыли или
// сменили разворот, «Куда» — остаётся тост. Постановка — тот же decor.pick, что щелчок по стоящей плитке (вопросы, один шаг отмены).
import { store } from '../../base/core/store.svelte'
import { capFirst, errText } from '../../base/words/errtext'
import { sheetIn } from '../../base/geom/sheet'
import { packsUi } from '../../shell/packs/packs.svelte'
import { decor } from '../decor/decor.svelte'
import type { DecorItem } from '../decor/decor'
import { isNoItem, noItemText, otherWhereText, type GetItem, type GetKind } from './decorget'
import { getApi } from './getapi'
import { tplVisible } from '../decor/spreadtpl'

// Набор скачан и предмет найден — его плитка; иначе null (тосты 137 или «предмета нет» уже показаны). still() — окно ещё ждёт:
// спрашивается до и после поиска предмета, пока шёл вызов, окно могли закрыть.
export async function fetchPackItem(tile: GetItem, kind: GetKind, still: () => boolean): Promise<DecorItem | null> {
  const g = tile.get
  const r = await packsUi.net.fetchOne({ id: g.pack, title: g.title })
  if (!r?.picked || !still()) return null
  try {
    const item = await getApi.item(kind, g.pack, g.src)
    return still() ? item : null
  } catch (e) {
    if (!still()) return null
    if (isNoItem(e)) store.notify(noItemText(g.title, kind))
    else store.notify(capFirst(errText(e)), 'error')
    return null
  }
}

// «Скачать и поставить» в вопросе окна «Фоны», «Стикеры», «Рамочки», «Развороты».
export async function confirmDecorGet(): Promise<void> {
  const a = decor.ask
  const c = decor.cur
  if (a?.kind !== 'get' || !c) return
  const where = decor.target()
  decor.dismiss()
  const item = await fetchPackItem(a.tile, c.kind, () => decor.target() === where)
  if (!item) return
  // Разворот встаёт не на любое «Куда»: страница — на любую область, разворот — на «Весь разворот» листа со швом
  const sp = sheetIn(store.view, c.spread)
  if (c.kind === 'spread' && sp && tplVisible([item], c.scope, sp).length === 0) {
    store.notify(otherWhereText(a.tile.get.title))
    return
  }
  await decor.pick(item)
}
