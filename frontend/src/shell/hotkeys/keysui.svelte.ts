// Окно «Горячие клавиши» (074, design §8): состояние и действия. Логику клавиш считает Go (SetHotkey и другие вернут
// новый вид), здесь — выбор, запись сочетания, вопрос «Заменить?» и строка «… · Вернуть».
import { Api, store } from '../../base/core/store.svelte'
import { isMac } from '../../base/core/platform'
import { keymap } from '../../base/keys/keymap.svelte'
import type { FeatureId } from '../../base/prefs/levels'
import { prefs } from '../../base/prefs/prefs.svelte'
import type { KeymapEntry, KeymapSetResult, KeymapView, KeyRef } from '../../base/types/keys'
import { capFirst, errText } from '../../base/words/errtext'
import { askText, assignedText, removedText, resetAllText, resetOneText, shadowNote } from './keywords'
import { partialText, pressStep, recStep, type RecEvent } from './keyrec'
import { cmdsAt, curId, modsOf, NO_MODS, rankOf, reasonOf, shadowsOf, type Mods, type SelCtx, type Sel } from './keysel'

type Custom = Record<string, string[]>

export interface Rec {
  id: string
  old: string // заменяемое сочетание; '' — новое («+»)
  partial: string // недописанное: «Ctrl+…»
}
// Вопрос «Заменить?»: set — назначение, reset — «Стандартное», упёршееся в чужую клавишу (074-22).
export interface Ask {
  kind: 'set' | 'reset'
  id: string
  old: string
  chord: string
  holder: KeyRef
  fromKey: boolean // назначали с выбранной клавиши: после «Заменить» она остаётся выбранной
}
export interface Status {
  text: string
  prev: Custom // своё до правки — для «Вернуть»
}

// Что делать с нажатием: окно его забрало или оно идёт дальше (Esc закрывает окно, Tab — по полям).
export type KeyVerdict = 'eat' | 'pass'

// Сами модификаторы: keyup меняет слой карты или недописанное сочетание.
const MODIFIER_KEY = /^(Control|Shift|Alt|AltGraph|Meta)$/

const clone = (c: Custom | null | undefined): Custom => JSON.parse(JSON.stringify(c ?? {})) as Custom
const asView = (v: unknown): KeymapView => v as KeymapView

class KeysUi {
  open = $state(false)
  sel = $state.raw<Sel>(null)
  mods = $state<Mods>({ ...NO_MODS })
  query = $state('')
  onlyOwn = $state(false)
  rec = $state<Rec | null>(null)
  recErr = $state<{ id: string; text: string } | null>(null)
  ask = $state.raw<Ask | null>(null)
  status = $state.raw<Status | null>(null)
  notes = $state.raw<string[]>([])
  pend = $state<string | null>(null) // свободная клавиша ждёт выбора команды в списке
  scroll = $state.raw<{ id: string; n: number } | null>(null) // «прокрутить список к команде» (n — чтобы повтор тоже сработал)
  private busy = false // ответ Go ещё не пришёл: новые нажатия записи не принимаются

  // Команда есть на уровне интерфейса (скрытая держит своё сочетание, но в списке и на карте её нет).
  readonly visible = (e: KeymapEntry): boolean => !e.feature || prefs.can(e.feature as FeatureId)
  ctx = $derived<SelCtx>({ index: keymap.index, locked: keymap.view.locked, visible: this.visible })
  // Команда, которую показывает карточка: выбранная строка или команда выбранной клавиши.
  cur = $derived(curId(this.ctx, this.sel))

  show() {
    this.sel = null
    this.mods = { ...NO_MODS }
    this.query = ''
    this.onlyOwn = false
    this.rec = null
    this.recErr = null
    this.ask = null
    this.status = null
    this.notes = []
    this.pend = null
    this.scroll = null
    this.open = true
    // Вид мог поменять другой экземпляр интерфейса: берём свежий.
    void Api.Keymap()
      .then((v) => keymap.apply(asView(v)))
      .catch(() => {})
  }

  close() {
    this.open = false
    this.rec = null
    this.ask = null
    this.pend = null
  }

  // Esc окна: сначала отмена записи, потом отмена «выберите команду»; иначе окно закроется.
  escape(): boolean {
    if (this.rec) {
      this.cancelRec()
      return true
    }
    if (this.pend) {
      this.pend = null
      return true
    }
    return false
  }

  private scrollTo(id: string) {
    this.scroll = { id, n: (this.scroll?.n ?? 0) + 1 }
  }

  // Выбор клавиши (нажатием или щелчком по карте): слой карты — по её модификаторам, список — к команде клавиши.
  selectKey(chord: string) {
    this.sel = { kind: 'key', chord, pick: null }
    this.pend = null
    this.recErr = null
    this.mods = modsOf(chord)
    if (this.cur) this.scrollTo(this.cur)
  }

  // Щелчок по строке списка. Свободная клавиша ждёт команду — тогда это назначение.
  selectRow(id: string) {
    if (this.pend) {
      void this.assignTo(id)
      return
    }
    const s = this.sel
    if (s?.kind === 'key' && !reasonOf(s.chord, this.ctx.locked) && cmdsAt(this.ctx, s.chord).some((e) => e.id === id)) {
      this.sel = { ...s, pick: id } // команда этой же клавиши: клавиша остаётся выбранной
      return
    }
    this.sel = { kind: 'cmd', id }
    const first = keymap.entry(id)?.keys[0]
    if (first) this.mods = modsOf(first) // карта показывает слой первой клавиши команды
  }

  // Переключатель команд клавиши в карточке.
  pickCmd(id: string) {
    if (this.sel?.kind !== 'key') return
    this.sel = { ...this.sel, pick: id }
    this.scrollTo(id)
  }

  toggleMod(m: keyof Mods) {
    this.mods = { ...this.mods, [m]: !this.mods[m] }
  }

  // ——— Запись сочетания ———
  startRec(id: string, old: string) {
    this.rec = { id, old, partial: '' }
    this.recErr = null
    this.pend = null
  }

  cancelRec() {
    this.rec = null
    this.recErr = null
  }

  // Нажатие в окне (capture на window). eat — окно забрало его; pass — пусть идёт дальше.
  keydown(e: RecEvent, inText: boolean, own = false): KeyVerdict {
    if (!this.open || this.ask) return 'pass'
    const mac = isMac()
    if (this.rec) {
      this.onRecKey(e, mac)
      return 'eat'
    }
    if (inText) return 'pass' // поле поиска: буквы и отмена — полю
    const s = pressStep(e, mac, own)
    if (s.kind === 'pass') return 'pass'
    if (s.kind === 'select') this.selectKey(s.chord)
    else this.mods = this.flagMods(e, mac)
    return 'eat'
  }

  keyup(e: RecEvent, inText: boolean) {
    if (!this.open || this.ask) return
    const mac = isMac()
    if (this.rec) {
      if (MODIFIER_KEY.test(e.key)) this.rec = { ...this.rec, partial: partialText(e, mac) }
    } else if (!inText && MODIFIER_KEY.test(e.key)) {
      this.mods = this.flagMods(e, mac)
    }
  }

  // Окно потеряло фокус (Alt+Tab): запись отменяется, иначе следующая клавиша ушла бы в чужое окно.
  blur() {
    if (this.rec) this.cancelRec()
  }

  private flagMods(e: RecEvent, mac: boolean): Mods {
    return { mod: mac ? e.metaKey : e.ctrlKey, alt: e.altKey, shift: e.shiftKey }
  }

  private onRecKey(e: RecEvent, mac: boolean) {
    const r = this.rec
    if (!r || this.busy) return
    const s = recStep(e, mac)
    if (s.kind === 'cancel') this.cancelRec()
    else if (s.kind === 'partial') this.rec = { ...r, partial: s.text }
    else if (s.kind === 'bad') {
      this.recErr = { id: r.id, text: 'Эту клавишу назначить нельзя.' }
      this.rec = { ...r, partial: '' }
    } else if (s.kind === 'chord') void this.setKey(r.id, r.old, s.chord, false, false)
  }

  // ——— Правки (всё решает Go; здесь — вид ответа) ———
  private async setKey(id: string, old: string, chord: string, replace: boolean, fromKey: boolean) {
    const prev = clone(keymap.view.custom)
    this.busy = true
    try {
      const res = (await Api.SetHotkey(id, old, chord, replace)) as unknown as KeymapSetResult
      this.pend = null
      if (res.conflict) {
        this.ask = { kind: 'set', id, old, chord, holder: res.conflict, fromKey }
        this.rec = null
        return
      }
      keymap.apply(res.view)
      this.rec = null
      this.recErr = null
      const e = keymap.entry(id)
      const replaced = res.replaced ? { label: res.replaced.label, visible: this.isVisible(res.replaced.id) } : null
      this.status = { text: assignedText(chord, e?.label ?? id, replaced, isMac()), prev }
      this.notes = this.shadowNotes(e, chord, res.shadows)
      if (fromKey) {
        this.sel = { kind: 'key', chord, pick: id }
        this.scrollTo(id)
      }
    } catch (err) {
      const text = capFirst(errText(err))
      if (this.rec) {
        this.recErr = { id: this.rec.id, text }
        this.rec = { ...this.rec, partial: '' }
      } else {
        this.status = null
        this.notes = [text]
      }
    } finally {
      this.busy = false
    }
  }

  private isVisible(id: string): boolean {
    const e = keymap.entry(id)
    return !e || this.visible(e)
  }

  // Строки про перекрытия: ответ Go (Shadows) — названия команд, область и ранг берём из каталога.
  private shadowNotes(e: KeymapEntry | undefined, chord: string, shadows: KeyRef[]): string[] {
    if (!e) return []
    const view = keymap.view
    const out: string[] = []
    for (const s of shadows) {
      const o = keymap.entry(s.id)
      if (o) out.push(shadowNote(e, o, chord, rankOf(view, o), rankOf(view, e), this.visible(o), isMac()))
    }
    return out
  }

  // Перекрытия сочетания у команды — для карточки (то же, что вернёт Go после правки).
  notesFor(e: KeymapEntry): string[] {
    const view = keymap.view
    const out: string[] = []
    for (const ch of e.keys) {
      for (const o of shadowsOf(view, keymap.index, e, ch)) {
        const note = shadowNote(e, o, ch, rankOf(view, o), rankOf(view, e), this.visible(o), isMac())
        if (!out.includes(note)) out.push(note)
      }
    }
    return out
  }

  askText(a: Ask): string {
    return askText(a.chord, a.holder.label, this.isVisible(a.holder.id), isMac())
  }

  confirmAsk() {
    const a = this.ask
    this.ask = null
    if (!a) return
    if (a.kind === 'set') void this.setKey(a.id, a.old, a.chord, true, a.fromKey)
    else void this.resetOne(a.id, true)
  }

  cancelAsk() {
    this.ask = null
  }

  // «Назначить команду…» у свободной клавиши: следующий щелчок по строке списка назначает её.
  assignKey(chord: string) {
    this.pend = chord
  }

  cancelPend() {
    this.pend = null
  }

  private async assignTo(id: string) {
    const chord = this.pend
    if (!chord || this.busy) return
    if (keymap.entry(id)?.fixed) {
      this.status = null
      this.notes = ['Служебной команде клавишу не назначить — выберите другую.']
      return
    }
    await this.setKey(id, '', chord, false, true)
  }

  async remove(id: string, chord: string) {
    const prev = clone(keymap.view.custom)
    try {
      keymap.apply(asView(await Api.RemoveHotkey(id, chord)))
      this.status = { text: removedText(chord, keymap.entry(id)?.label ?? id, isMac()), prev }
      this.notes = []
    } catch (err) {
      store.notify(capFirst(errText(err)), 'error')
    }
  }

  // «Стандартное» у одной команды; упёрлось в своё сочетание другой — тот же вопрос «Заменить?».
  async resetOne(id: string, replace = false) {
    const prev = clone(keymap.view.custom)
    try {
      const res = (await Api.ResetHotkey(id, replace)) as unknown as KeymapSetResult
      if (res.conflict) {
        this.ask = { kind: 'reset', id, old: '', chord: res.conflict.chord, holder: res.conflict, fromKey: false }
        return
      }
      keymap.apply(res.view)
      this.status = { text: resetOneText(keymap.entry(id)?.label ?? id), prev }
      this.notes = []
    } catch (err) {
      store.notify(capFirst(errText(err)), 'error')
    }
  }

  async resetAll() {
    const prev = clone(keymap.view.custom)
    const n = keymap.view.changed
    try {
      keymap.apply(asView(await Api.ResetHotkeys()))
      this.status = { text: resetAllText(n), prev }
      this.notes = []
    } catch (err) {
      store.notify(capFirst(errText(err)), 'error')
    }
  }

  // «Вернуть»: свои сочетания как до правки.
  async restore() {
    const s = this.status
    if (!s) return
    try {
      keymap.apply(asView(await Api.RestoreHotkeys(s.prev)))
      this.status = null
      this.notes = []
    } catch (err) {
      store.notify(capFirst(errText(err)), 'error')
    }
  }
}

export const keysUi = new KeysUi()
