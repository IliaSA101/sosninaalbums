// Бросок фото на ручной разворот (спека 042): места, плитка новых мест, вопрос об остальных.
// Решает Go (Api.DropPhotos); здесь вызов, вопрос «что с остальными» и тосты после правки.
import * as Api from '../../../wailsjs/go/app/App'
import { errText } from '../words/errtext'
import { store } from '../core/store.svelte'
import { movedText, type Moved } from '../core/strip'
import type { DropResult, View } from '../core/types'
import { unitOf, type Unit } from '../geom/sheet'

// Что брошено и куда: нужно и для первого вызова, и для второго (с ответом на вопрос).
interface Query {
  spread: string
  place: string // '' — на лист, иначе на место
  ids: string[]
  at: number // доля ширины листа под курсором, < 0 — точки нет
  fromSpread: boolean // фото уже в альбоме: тост «Перенесено из …» не нужен
  go: boolean // после правки перейти на лист (бросок на миниатюру)
  moved: Moved[] // откуда пришли фото; считается до первого вызова
}

// Открытый вопрос: Go ответил Ask, проект не менялся (rev тот же).
export interface DropAsk extends Query {
  fit: number
  total: number
  grow: boolean // нетронутая виньетка: лишние фото станут учениками, а не плиткой
  rev: number
}

class DropAskState {
  cur = $state<DropAsk | null>(null)

  // Правка из другого места (Ctrl+Z, лист удалён) делает вопрос неактуальным.
  prune(v: View) {
    const a = this.cur
    if (a && (v.rev !== a.rev || !v.spreads.some((s) => s.id === a.spread))) this.cur = null
  }
}

export const dropAsk = new DropAskState()

// Тексты окна вопроса. Слово «разворот» у листа виньетки — «лист» (unitOf).
export function dropAskText(a: Pick<DropAsk, 'fit' | 'total' | 'grow'>, unit: Unit) {
  return {
    title: 'Не хватает свободных мест',
    ask: `На свободные места встанут ${a.fit} из ${a.total} фото. Что сделать с остальными?`,
    skip: `Не добавлять на ${unit.one}`,
    add: a.grow ? 'Добавить учеников в виньетку' : `Вставить на ${unit.one} в свободной форме`,
  }
}

// Пока запрос идёт, второй бросок игнорируется: иначе второй вызов Go встал бы в очередь за первым.
let inflight = false

// Бросок фото ids на лист spread (place '' — на лист) или на место. at — доля ширины листа, < 0 — нет.
// Пока открыт вопрос, новый бросок (Enter под окном, второй жест) игнорируется: ответ уже ждёт первый.
export async function dropGrid(spread: string, place: string, ids: string[], at: number, fromSpread: boolean, go = false): Promise<void> {
  if (ids.length === 0 || inflight || dropAsk.cur) return
  const moved = fromSpread ? [] : store.movedFrom(ids, spread)
  await call({ spread, place, ids, at, fromSpread, go, moved }, '')
}

// Ответ на вопрос: 'add' — остальные плиткой (у виньетки — учениками), 'skip' — не добавлять.
export async function answerDrop(rest: 'add' | 'skip'): Promise<void> {
  const q = dropAsk.cur
  dropAsk.cur = null
  if (q) await call(q, rest)
}

// × и Esc: бросок отменён целиком, Go не зовём.
export function cancelDrop() {
  dropAsk.cur = null
}

async function call(q: Query, rest: '' | 'add' | 'skip') {
  inflight = true
  try {
    const r = (await Api.DropPhotos(q.spread, q.place, q.ids, q.at, rest)) as unknown as DropResult
    if (r.ask) {
      const vig = store.sheet(q.spread)?.vig
      dropAsk.cur = { ...q, fit: r.fit, total: r.total, grow: !!vig && !vig.manual, rev: r.view.rev }
    } else done(q, r)
  } catch (e) {
    // Предел мест и слишком мелкая плитка приходят ошибкой: проект прежний.
    store.notify(errText(e), 'error')
  } finally {
    inflight = false
  }
}

function done(q: Query, r: DropResult) {
  store.apply(r.view)
  if (q.go) store.goId(q.spread)
  const sheet = store.sheet(q.spread)
  // «Перенесено из …» не показываем сразу: общий тост ниже должен нести и её (иначе «Вернуть» перетрёт).
  let lead = ''
  if (!q.fromSpread) {
    lead = movedText(
      q.moved.filter((m) => sheet?.photos.includes(m.id)),
      store.view,
    )
    store.selected.clear()
  } else if (q.ids.length > 1) store.selected.clear()
  // Новые места выделены, как после «Место»: их сразу можно тащить группой. Выделение — у текущего листа.
  const fresh = r.fresh ?? []
  if (fresh.length > 0 && store.currentId === q.spread) store.hooks.select(fresh)
  const notes: string[] = []
  if (r.out) notes.push('Прежнее фото вернулось в ленту')
  if (r.grown > 0) notes.push(`Учеников в виньетке: ${sheet?.vig?.pupils ?? ''}`)
  if (r.small) notes.push(`Места вышли мелкими — много фото удобнее бросить на «+ ${unitOf(store.view.settings).new}»`)
  if (notes.length > 0) store.undoToast([lead, ...notes].filter(Boolean).join('. '))
  else if (lead) store.notify(lead)
}
