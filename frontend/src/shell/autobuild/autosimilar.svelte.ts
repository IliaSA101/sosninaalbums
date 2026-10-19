import { SvelteSet } from 'svelte/reactivity'
import type { AutoRange, PhotoOpts, SigState, SimilarDrop } from '../../base/core/types'
import { norm03 } from '../../base/prefs/view.svelte'
import { keepKey, mergeDrops } from './autosimilar'

export type SigsFn = (ids: readonly string[]) => Promise<SigState>
export type DropsFn = (ids: readonly string[], range: AutoRange, o: PhotoOpts) => Promise<SimilarDrop[]>

// Связь блока «Фото» с окном: что спросить заново, когда меняются похожие, ч/б, вариант или готовы подписи.
export interface PhotosHost {
  range(): AutoRange // на «похожих» влияют серии съёмки
  replan(): void // перезапросить план и ряд «Раскладки»
}

export interface PhotosSaved {
  similar?: number
  gray?: number
  again?: boolean
  variant?: number
  keep?: readonly string[] // «Вернуть» прошлой сборки: пересборка берёт тот же набор фото
}

export const MAX_VARIANT = 9999 // предел Go (Clamp)

// Блок «Фото» окна автосборки (078, design §9.2): похожие кадры, ч/б, вариант и ход подготовки подписей.
// Запросов в Go отсюда два: PrepareSigs (подписи кадров, «Собрать» ждёт) и AutoSimilar (ряд отброшенных, один в полёте).
export class PhotosState {
  similar = $state(0)
  gray = $state(0)
  variant = $state(1)
  again = false
  // «Вернуть»: кадры, которые пользователь велел не убирать (в окне, между окнами не помнится)
  readonly keep = new SvelteSet<string>()
  sigs = $state<SigState | null>(null)
  // Ряд отброшенных: раскрыт ли и что в нём (ответ Go + вернувшиеся)
  dropsOpen = $state(false)
  drops = $state<SimilarDrop[] | null>(null)
  dropsBusy = $state(false)
  dropsError = $state(false)

  private requested = false // PrepareSigs уже ушёл
  private restored: boolean // «Вернуть» пришли из прошлой сборки: ряду ещё не спрашивали их плитки
  private dropsRunning = false
  private order: Record<string, number>
  private kk = '0:0'

  constructor(
    private ids: readonly string[],
    private host: PhotosHost,
    saved: PhotosSaved,
    // Ч/б без функции «Ч/б и тон фото» (уровень «Простой») в Go не уходит, запомненное значение остаётся
    private grayOn: boolean,
    private sigsFn: SigsFn | null,
    private dropsFn: DropsFn | null,
  ) {
    this.similar = norm03(saved.similar)
    this.gray = norm03(saved.gray)
    this.again = saved.again === true
    this.variant = clampVariant(saved.variant ?? 1)
    for (const id of saved.keep ?? []) this.keep.add(id)
    this.kk = keepKey(this.keep)
    this.restored = this.keep.size > 0
    this.order = Object.fromEntries(ids.map((id, i) => [id, i]))
  }

  get grayNow(): number {
    return this.grayOn ? this.gray : 0
  }

  // Подписи нужны «похожим» и ч/б (цветность)
  get needSigs(): boolean {
    return this.similar > 0 || this.grayNow > 0
  }

  get ready(): boolean {
    return !this.needSigs || (this.requested && !!this.sigs && this.sigs.done >= this.sigs.total)
  }

  // «Собрать» ждёт, пока подписи не готовы (как «Считаю…»)
  get waiting(): boolean {
    return !this.ready
  }

  // Что уходит в Go вместе с диапазоном
  get opts(): PhotoOpts {
    return { similar: this.similar, keep: [...this.keep], gray: this.grayNow, variant: this.variant, again: this.again }
  }

  // Часть ключа плана и предпросмотра: значения, от которых зависит ответ Go; ready — чтобы готовые подписи перезапросили план.
  get key(): string {
    return `${this.similar}|${this.kk}|${this.grayNow}|${this.variant}|${this.again}|${this.ready}`
  }

  // Открытие окна: подготовка подписей, если нужны; иначе сразу план. Готовность сама перезапросит план.
  start(): void {
    if (this.needSigs) void this.prepare()
    else this.host.replan()
  }

  // Окно закрыто: отмена фоновой работы Go (PrepareSigs([]))
  dispose(): void {
    if (!this.requested || !this.sigsFn) return
    this.requested = false
    void this.sigsFn([]).catch(() => {})
  }

  setSimilar(v: number): void {
    const n = norm03(v)
    if (n === this.similar) return
    this.similar = n
    if (n === 0) {
      this.drops = null
      this.dropsError = false
    }
    this.changed()
  }

  setGray(v: number): void {
    const n = norm03(v)
    if (n === this.gray) return
    this.gray = n
    this.changed()
  }

  // «Вариант ‹ k ›»: k ≥ 1
  setVariant(v: number): void {
    const n = clampVariant(v)
    if (n === this.variant) return
    this.variant = n
    this.changed()
  }

  setDropsOpen(open: boolean): void {
    this.dropsOpen = open
    if (open) void this.dropsPump()
  }

  // Щелчок или Enter по плитке: вернуть / убрать снова
  toggleKeep(id: string): void {
    if (!this.keep.delete(id)) this.keep.add(id)
    this.keepChanged()
  }

  // «Вернуть все»: всё показанное в ряду
  keepAll(): void {
    for (const d of this.drops ?? []) this.keep.add(d.id)
    this.keepChanged()
  }

  // Событие sigs:progress. Чужие и опоздавшие (после закрытия) — мимо.
  onProgress(s: SigState): void {
    this.take(s)
  }

  private keepChanged(): void {
    this.kk = keepKey(this.keep)
    this.changed()
  }

  // Любая правка значений: пока подписи не готовы, спрашивать нечего — готовность сама перезапросит всё.
  private changed(): void {
    if (this.needSigs && !this.requested) {
      void this.prepare()
      return
    }
    if (this.waiting) return
    this.host.replan()
    void this.dropsPump()
  }

  // Сначала подписи, потом план: ответ Go приходит сразу, ход — событиями. Ошибка не держит «Собрать» вечно.
  private async prepare(): Promise<void> {
    this.requested = true
    this.sigs = null
    let st: SigState = { done: 0, total: 0 }
    try {
      if (this.sigsFn) st = await this.sigsFn(this.ids)
    } catch {
      // подписей нет — сборка идёт без отбора похожих, не ждёт вечно
    }
    this.take(st)
  }

  // Событие может обогнать ответ: не откатываем готовое назад
  private take(s: SigState): void {
    if (!this.requested) return // окно закрыто, пока ждали ответ
    const was = this.ready
    if (!this.sigs || this.sigs.total !== s.total || s.done >= this.sigs.done) this.sigs = { done: s.done, total: s.total }
    if (!was && this.ready) this.onReady()
  }

  private onReady(): void {
    this.host.replan()
    void this.dropsPump()
  }

  // Ряд отброшенных (AutoSimilar): пока раскрыт и подписи готовы; один запрос, устаревший ответ отбрасывается — как previewPump.
  private async dropsPump(): Promise<void> {
    if (!this.dropsFn || this.dropsRunning || !this.dropsOpen || this.similar <= 0) return
    this.dropsRunning = true
    this.dropsBusy = true
    try {
      for (;;) {
        if (!this.dropsOpen || this.similar <= 0 || this.waiting) return
        const k = this.dropsKey()
        let res: SimilarDrop[]
        try {
          // Первый вопрос с уже возвращёнными (пересборка): без них, чтобы ряд показал и их плитки
          const o = this.restored ? { ...this.opts, keep: [] } : this.opts
          res = await this.dropsFn(this.ids, this.host.range(), o)
        } catch {
          if (k !== this.dropsKey()) continue
          this.dropsError = true
          return
        }
        if (k !== this.dropsKey()) continue
        this.restored = false
        this.drops = mergeDrops(this.drops, res ?? [], this.keep, this.order)
        this.dropsError = false
        return
      }
    } finally {
      this.dropsRunning = false
      this.dropsBusy = false
    }
  }

  private dropsKey = () => `${this.key}|${this.host.range().series}|${this.dropsOpen}`
}

export const clampVariant = (v: number): number => (Number.isFinite(v) ? Math.max(1, Math.min(MAX_VARIANT, Math.round(v))) : 1)
