// Копировать и вставить на холсте (спека 099): состояние и вызовы Go. Буфер живёт в Go (App.clip);
// здесь только его сводка, точка курсора над листом и действия Ctrl+C, X, V, D.
import * as Api from '../../../wailsjs/go/app/App'
import { store } from '../../base/core/store.svelte'
import { askCopy } from '../../base/drag/copyask.svelte'
import { movedText, type Moved } from '../../base/core/strip'
import type { View } from '../../base/core/types'
import { keymap } from '../../base/keys/keymap.svelte'
import { errText } from '../../base/words/errtext'
import { gridAct } from '../../panels/layouts/gridact.svelte'
import { placesel } from '../places/placesel.svelte'
import { edit } from '../text/edit.svelte'
import {
  NO_CLIP,
  REPEAT_TEXT,
  canPaste,
  clipSel,
  clipToast,
  pastedRefs,
  stamp,
  type ClipCmd,
  type ClipInfo,
  type ClipSel,
  type Pt,
} from './clip'

// Ответ вставки и дубля (app.PasteResult).
interface Pasted {
  view: View
  refs: string[]
  texts: string[]
  stickers: string[]
  repeat: string[]
  clip: ClipInfo
}

// Методы биндингов принимают классы с методами; в Go уходят только поля.
const asArg = <T>(v: object) => v as unknown as T

class Clip {
  // Сводка буфера Go: тосты, серые пункты «Правки».
  info = $state<ClipInfo>(NO_CLIP)
  // Курсор над текущим листом, мм (ставит NameTrack.track, снимает leave); null — не над листом.
  hover: Pt | null = null
  // Вызовы идут по очереди: Ctrl+C и сразу Ctrl+V не обгоняют друг друга.
  private chain: Promise<unknown> = Promise.resolve()

  // Выделенное на текущем листе; идёт ввод надписи — копировать нечего (там родное копирование текста).
  private sel(): ClipSel | null {
    if (edit.editing) return null
    const s = store.spread
    const one = edit.sel?.kind === 'text' && edit.sel.spread === s?.id ? edit.sel.id : null
    return clipSel(s, placesel.refs, placesel.texts, placesel.stickers, one)
  }

  get canCopy(): boolean {
    return this.sel() !== null
  }

  get canPaste(): boolean {
    return canPaste(this.info) && !!store.spread && !edit.editing
  }

  // Сводка буфера из Go: при старте и после смены альбома (буфер прежнего альбома пуст, 099-3).
  // Идёт по той же очереди, что и команды: поздний ответ ClipState не откатит сводку после Ctrl+C сразу за «Открыть».
  sync(): Promise<void> {
    const job = async () => {
      try {
        this.info = (await Api.ClipState()) as ClipInfo
      } catch {
        this.info = NO_CLIP
      }
    }
    const run = this.chain.then(job)
    this.chain = run
    return run
  }

  private queue(cmd: ClipCmd, job: () => Promise<void>) {
    stamp(cmd, Date.now())
    this.chain = this.chain.then(job).catch((e) => store.notify(errText(e), 'error'))
  }

  private toast(verb: 'Скопировано' | 'Вырезано', info: ClipInfo) {
    store.notify(clipToast(verb, info, keymap.hint('pasteItems')))
  }

  // Ctrl+C: снимок выделенного в этот момент. Нет выделения — ничего, буфер прежний.
  copy() {
    const sel = this.sel()
    const sid = store.spread?.id
    if (!sel || !sid) return
    this.queue('copyItems', async () => {
      this.info = (await Api.CopyItems(sid, asArg(sel))) as ClipInfo
      this.toast('Скопировано', this.info)
    })
  }

  // Ctrl+X: снимок и удаление одним шагом отмены (место уходит вместе с фото).
  cut() {
    const sel = this.sel()
    const sid = store.spread?.id
    if (!sel || !sid) return
    this.queue('cutItems', async () => {
      const r = await Api.CutItems(sid, asArg(sel))
      this.info = r.clip as ClipInfo
      if (!(await store.take(r.view as unknown as View))) return
      this.toast('Вырезано', this.info)
    })
  }

  // Ctrl+V и «Вставить» меню (at — центр вставленного, мм листа). Куда встанет, решает Go.
  paste(at?: Pt) {
    const sid = store.spread?.id
    if (!sid || !this.canPaste) return
    const req = { sheet: sid, ...(this.hover ? { hover: { ...this.hover } } : {}), ...(at ? { at: { ...at } } : {}) }
    const call = (repeat: string) => Api.PasteItems(asArg({ ...req, repeat }))
    this.queue('pasteItems', () => this.apply(sid, call(''), call))
  }

  // Ctrl+D: дубль выделенного на том же листе со сдвигом; буфер не меняется. Фото — копией сразу (0.23-q2).
  duplicate() {
    const sel = this.sel()
    const sid = store.spread?.id
    if (!sel || !sid) return
    this.queue('duplicateItems', () => this.apply(sid, Api.DuplicateItems(sid, asArg(sel))))
  }

  // Общий хвост вставки и дубля: вид, выделение вставленного, тосты. again — второй вызов с ответом окна 098
  // (только вставка: дубль фото копия сразу, вопроса у него нет).
  private async apply(sid: string, first: Promise<unknown>, again?: (repeat: string) => Promise<unknown>) {
    let r = (await first) as Pasted
    let moved: Moved[] = [] // откуда перенесено при ответе «Перенести сюда» — считаем до правки, как у броска из ленты
    this.info = r.clip
    // Фото уже в альбоме: Go ничего не менял — спрашиваем окном 098 и зовём второй раз с ответом; «Отмена» — ничего.
    if (r.repeat.length > 0) {
      if (!again) return void store.notify(REPEAT_TEXT)
      const a = await askCopy(r.repeat)
      if (a === 'cancel' || store.currentId !== sid) return
      if (a === 'move') moved = store.movedFrom(r.repeat, sid)
      r = (await again(a)) as Pasted
      this.info = r.clip
      if (r.repeat.length > 0) return void store.notify(REPEAT_TEXT) // Go не принял ответ — вид прежний
    }
    // Через placeEdit: «Авто», ставшая «Ручной», и виньетка получают тост с «Вернуть», как от «+ Место».
    if (!(await gridAct.placeEdit(sid, Promise.resolve(r.view)))) return
    if (store.currentId !== sid) return
    this.movedToast(moved, sid)
    placesel.pickHit({ refs: pastedRefs(store.spread, r.refs), texts: r.texts, stickers: r.stickers }, false)
  }

  // «Фото перенесено из …» (098 п. 4) — тот же текст, что у броска из ленты; только по фото, реально вставшим на лист.
  // Тост «Вернуть» от placeEdit («Раскладка стала ручной») не перетираем, а дополняем.
  private movedToast(moved: Moved[], sid: string) {
    const landed = store.sheet(sid)?.photos
    const text = movedText(
      moved.filter((m) => landed?.includes(m.id)),
      store.view,
    )
    if (!text) return
    const t = store.toast
    if (t?.action && t.rev === store.view.rev) store.toast = { ...t, text: `${text}. ${t.text}` }
    else store.notify(text)
  }
}

export const clip = new Clip()
