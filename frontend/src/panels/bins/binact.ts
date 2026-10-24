// Действия с подборками (083): вызовы Go и тосты. Правка идёт через store.take (цепочка видов, undo — общий ряд);
// ошибки create/rename возвращаются текстом — окно показывает их под полем, а не тостом.
import { store } from '../../base/core/store.svelte'
import { capFirst, errText } from '../../base/words/errtext'
import type { View } from '../../base/core/types'
import * as Bins from './binapi'
import {
  addedText,
  alreadyText,
  binAction,
  binChange,
  binsOf,
  cleanName,
  DEL_HINT,
  deletedText,
  nameTaken,
  noBinsText,
  NO_SELECTION,
  NO_TARGET,
  removedText,
  TAKEN_TEXT,
} from './bins'
import { binsel } from './binsel.svelte'

export interface BinResult {
  ok: boolean
  err?: string // текст для поля; нет — ошибки нет или её уже показал store
}

async function call(p: Promise<View>): Promise<BinResult> {
  try {
    return { ok: await store.take(await p) }
  } catch (e) {
    return { ok: false, err: capFirst(errText(e)) }
  }
}

// Новая подборка, выделенные ids — в неё сразу. Становится целевой; лента не переключается.
export async function create(name: string, ids: string[]): Promise<BinResult> {
  const bins = binsOf(store.view)
  if (name.trim() !== '' && nameTaken(bins, name)) return { ok: false, err: TAKEN_TEXT }
  const before = new Set(bins.map((b) => b.id))
  const r = await call(Bins.NewBin(name, ids))
  if (r.ok) {
    const added = binsOf(store.view).find((b) => !before.has(b.id))
    if (added) binsel.target = added.id
  }
  return r
}

// Переименовать. Пустое имя и прежнее — холостая правка без вызова Go (поле просто закрывается).
export async function rename(id: string, name: string): Promise<BinResult> {
  const bins = binsOf(store.view)
  const was = bins.find((b) => b.id === id)
  const next = cleanName(name)
  if (!was || next === '' || next === was.name) return { ok: true }
  if (nameTaken(bins, next, id)) return { ok: false, err: TAKEN_TEXT }
  return call(Bins.RenameBin(id, next))
}

// Удалить без вопроса: фото остаются, тост «Вернуть». Показанная пропадёт из списка — лента «Все фото» (prune в wire).
export async function remove(id: string): Promise<boolean> {
  const was = binsOf(store.view).find((b) => b.id === id)
  if (!was) return false
  const r = await call(Bins.DeleteBin(id))
  if (r.err) store.notify(r.err, 'error')
  if (r.ok) store.undoToast(deletedText(was.name))
  return r.ok
}

// Правки подборок идут по очереди: «добавить/убрать» и сравнение rev честны, только когда прежний ответ уже принят
// (быстрый второй B иначе видел бы устаревшие rev и членство).
let tail: Promise<unknown> = Promise.resolve()

// Добавить (add) или убрать выделенные ids; тост называет итог. Всё уже так — тост и никакого шага.
export function setFor(id: string, ids: readonly string[], add: boolean): Promise<boolean> {
  const run = tail.then(() => setNow(id, ids, add))
  tail = run.catch(() => {})
  return run
}

async function setNow(id: string, ids: readonly string[], add: boolean): Promise<boolean> {
  const was = binsOf(store.view).find((b) => b.id === id)
  if (!was) return false
  if (ids.length === 0) {
    store.notify(NO_SELECTION)
    return false
  }
  const act = add ? 'add' : 'remove'
  const n = binChange(ids, id, binsel.index, act)
  if (n === 0) {
    store.notify(alreadyText(was.name, act))
    return false
  }
  const rev = store.view.rev
  const r = await call(Bins.SetBin(id, [...ids], add))
  if (r.err) store.notify(r.err, 'error')
  if (!r.ok) return false
  // Членство в index перечитывается с опозданием; Go ответил холостой правкой (rev тот же) — шага отмены нет.
  if (store.view.rev === rev) {
    store.notify(alreadyText(was.name, act))
    return false
  }
  if (add) binsel.target = id
  store.undoToast(add ? addedText(was.name, n) : removedText(was.name, n))
  return true
}

// B: выделенные — в целевую подборку, повтор у всех выделенных — убрать (образец — цветные метки).
// Нет выделения / подборок / целевой — тост, а где нужно выбирать — окно «Подборки».
export async function toggleSelected(): Promise<boolean> {
  const ids = store.selectedIds
  if (ids.length === 0) {
    store.notify(NO_SELECTION)
    return false
  }
  const bins = binsOf(store.view)
  if (bins.length === 0) {
    store.notify(noBinsText(binsel.host))
    // Как Shift+B: сразу поле имени «Новая из выделенных (N)» с фокусом — Enter создаёт, а не уходит в программу.
    binsel.pending = []
    binsel.wantNew = true
    binsel.open = true
    return false
  }
  const target = binsel.targetId()
  if (!bins.some((b) => b.id === target)) {
    store.notify(NO_TARGET)
    binsel.open = true
    return false
  }
  return setFor(target, ids, binAction(ids, target, binsel.index) === 'add')
}

// Shift+B: окно «Подборки» сразу со строкой новой подборки (с выделением — «из выделенных (N)»).
export function openNew() {
  binsel.pending = []
  binsel.wantNew = true
  binsel.open = true
}

// Del в ленте: выделенные — из показанной подборки (с диска и из альбома ничего не уходит).
export async function removeSelected(): Promise<boolean> {
  const id = binsel.shown
  if (!id) {
    store.notify(DEL_HINT)
    return false
  }
  return setFor(id, store.selectedIds, false)
}

// Бросок фото из ленты на кнопку «Подборки»: есть целевая — сразу в неё; нет — окно с «+ N» для брошенных.
export async function drop(ids: readonly string[]): Promise<boolean> {
  if (ids.length === 0) return false
  const target = binsel.targetId()
  if (binsOf(store.view).some((b) => b.id === target)) return setFor(target, ids, true)
  binsel.pending = [...ids]
  binsel.wantNew = false
  binsel.open = true
  return false
}
