// Настройки программы (спеки 010, 011): уровень интерфейса, набор «Своего» и стартовое окно. Живут в
// config.json у Go (State.prefs, Api.SetLevel / SetCustom / SetShowStart), в проект и undo не попадают.
// Решения «что доступно» — levels.ts и levelcustom.ts.
import { Api, store } from '../core/store.svelte'
import {
  blockedRowText,
  blockedText,
  normBase,
  normLevel,
  type Base,
  type Custom,
  type FeatureId,
  type Level,
  type ObjectKind,
  type RowId,
} from './levels'
import { blockerOf, features, fromBase, setBase, shortOf, toggle, type Toggled } from './levelcustom'
import { modes } from './view.svelte'
import { errText } from '../core/errtext'
import type { Prefs as PrefsData } from '../core/types'

// Состояние уровня целиком: по нему делается «Вернуть» и откат при ошибке Go.
export interface LevelState {
  level: Level
  custom: Custom | null
}

// Результат правки для строки статуса матрицы: что изменилось и как вернуть (apply(prev)).
export interface Edit extends Pick<Toggled, 'added' | 'removed'> {
  prev: LevelState
}

// Набор из ответа Go: списки могут быть null, основа — любая строка.
function customOf(c: PrefsData['custom']): Custom | null {
  return c ? { base: normBase(c.base), on: [...(c.on ?? [])], off: [...(c.off ?? [])] } : null
}

const copyOf = (c: Custom | null): Custom | null => (c ? { base: c.base, on: [...c.on], off: [...c.off] } : null)

class Prefs {
  level = $state<Level>('simple')
  // Набор «Своего»: null — не настраивали. При переходе на другой уровень остаётся.
  custom = $state<Custom | null>(null)
  showStart = $state(true)
  // Первый запуск 0.5.0: стартовое окно показывается всегда (U2).
  first = $state(false)
  // Окно уровней открыто (LevelsDialog читает и закрывает).
  compare = $state(false)
  // Стартовое окно открыто: при запуске — пока стоит флажок или первый запуск; потом — «Справка ▸ Стартовое окно».
  startOpen = $state(false)
  // «Что нового» раскрыт в стартовом окне («Справка ▸ Что нового»).
  news = $state(false)

  // Доступные функции: пересчёт только при смене уровня или набора.
  enabled = $derived(features(this.level, this.custom))

  // Стрелкой: можно передавать как `can` без привязки this.
  can = (id: FeatureId): boolean => this.enabled.has(id)

  private seq = 0 // номер последнего apply: откат устаревшего вызова не трогает позднее состояние

  // Основа для нового набора: текущий готовый уровень (у «Своего» — его основа).
  private get baseNow(): Base {
    return this.level === 'custom' ? (this.custom?.base ?? 'simple') : this.level
  }

  load(p: PrefsData | undefined) {
    if (!p) return
    this.level = normLevel(p.level)
    this.custom = customOf(p.custom)
    this.showStart = p.showStart !== false
    this.first = !!p.first
    this.startOpen = this.showStart || this.first
  }

  // Закрытие стартового окна. Первый запуск: уровень записывается, чтобы окно «первого раза» не
  // повторялось (если выбран «Простой», set() ничего не сохранял).
  closeStart() {
    this.startOpen = false
    this.news = false
    if (!this.first) return
    this.first = false
    void Api.SetLevel(this.level).catch((e) => store.notify(errText(e), 'error'))
  }

  // Смена уровня. «Свой» в первый раз копирует текущий уровень как основу.
  set(level: Level): Promise<void> {
    if (level === this.level) return Promise.resolve()
    return this.apply({ level, custom: level === 'custom' ? (this.custom ?? fromBase(this.baseNow)) : this.custom })
  }

  // Галочка в колонке «Свой»: включает «Свой» с этой правкой (набор — прежний или от текущего уровня).
  toggle(row: RowId, on: boolean): Edit {
    const prev = this.snapshot()
    const t = toggle(this.custom ?? fromBase(this.baseNow), row, on)
    if (t.added.length || t.removed.length) void this.apply({ level: 'custom', custom: t.custom })
    return { added: t.added, removed: t.removed, prev }
  }

  // «Включить» из тоста: как toggle, но ждёт ответа Go; удалось (объект больше не заблокирован) — after.
  private async enable(row: RowId, kind: ObjectKind, after?: () => void): Promise<void> {
    const t = toggle(this.custom ?? fromBase(this.baseNow), row, true)
    if (t.added.length || t.removed.length) await this.apply({ level: 'custom', custom: t.custom })
    if (after && blockerOf(kind, this.level, this.custom) === null) after()
  }

  // Другая основа «Своего» начинает набор заново.
  setBase(b: Base): Edit {
    const prev = this.snapshot()
    void this.apply({ level: 'custom', custom: setBase(b) })
    return { added: [], removed: [], prev }
  }

  snapshot(): LevelState {
    return { level: this.level, custom: copyOf(this.custom) }
  }

  // Применить состояние: оптимистично, затем Go (каждый вызов пишет своё поле). Ответы могут прийти
  // не по порядку — UI на них не опирается; при ошибке откат к прежнему и тост. Если набор функций
  // сузился — сброс выделения, режимов и окон, как при переходе вниз.
  async apply(next: LevelState): Promise<void> {
    const prev = this.snapshot()
    const my = ++this.seq
    const before = this.enabled
    const oldCustom = this.custom
    // Вернуть «не настраивали» нельзя (в Go нет сброса): пишем пустой набор от того же уровня.
    const custom = next.custom ?? (oldCustom ? fromBase(next.level === 'custom' ? 'simple' : next.level) : null)
    this.level = next.level
    this.custom = custom
    if ([...before].some((id) => !this.enabled.has(id))) this.reset()
    try {
      if (custom && custom !== oldCustom) await Api.SetCustom(custom)
      if (prev.level !== next.level) this.first = !!(await Api.SetLevel(next.level)).first
    } catch (e) {
      // Откат только если после нас apply не начинался: иначе затрём чужое успешное состояние
      if (my === this.seq) {
        this.level = prev.level
        this.custom = prev.custom
      }
      store.notify(errText(e), 'error')
    }
  }

  async setShowStart(v: boolean) {
    const prev = this.showStart
    this.showStart = v
    try {
      await Api.SetShowStart(v)
    } catch (e) {
      this.showStart = prev
      store.notify(errText(e), 'error')
    }
  }

  // Сброс состояний, которых при новом наборе быть не должно (design §2.3 п. 4). Выделения, режимы и окна
  // верхних групп сбрасывает narrowed (подключает shell/boot/narrow.ts), здесь — только то, что в основе.
  private reset() {
    if (!this.can('timeline') && modes.big) modes.toggleBig()
    this.narrowed()
  }

  // Набор функций сузился: верх сбрасывает свои выделения, режимы и окна (читает prefs.can).
  narrowed: () => void = () => {}

  // Объект уровень не даёт править (без тоста): Alt+щелчок такого в стек не берёт (спека 015).
  locked = (kind: ObjectKind): boolean => blockerOf(kind, this.level, this.custom) !== null

  // Щелчок по объекту, который уровень править не даёт (вопрос 5): объект виден, но не выделяется;
  // тост подсказывает, что включить. Возвращает true, если щелчок надо погасить.
  // after — что сделать после успешного «Включить» (выделить тот объект, по которому щёлкнули);
  // при ошибке Go набор откатывается и after не зовётся.
  blocked(kind: ObjectKind, after?: () => void): boolean {
    const b = blockerOf(kind, this.level, this.custom)
    if (!b) return false
    if ('level' in b) {
      const need = b.level
      store.notify(blockedText(need), 'info', { label: 'Переключить', run: () => void this.set(need) })
    } else {
      const row = b.row
      store.notify(blockedRowText(shortOf(row)), 'info', { label: 'Включить', run: () => void this.enable(row, kind, after) })
    }
    return true
  }
}

export const prefs = new Prefs()
