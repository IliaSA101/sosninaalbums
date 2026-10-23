import { SvelteMap, SvelteSet } from 'svelte/reactivity'
import * as Api from '../../../wailsjs/go/app/App'
import { COVER_ID } from '../geom/sheet'
import type { StoreHooks } from './hooks'
import { backdropLost } from './undo'
import type { CoverView, Family, SpreadView, Variety, View } from './types'

// Что операциям над листами нужно от Store (структурный тип: цикла импортов со store нет).
export interface SpreadHost {
  readonly view: View
  readonly hooks: StoreHooks
  readonly onCover: boolean
  readonly spread: SpreadView | undefined
  readonly cover: CoverView | null
  busy: boolean
  run(p: Promise<unknown>): Promise<boolean>
  notify(text: string): void
  undoToast(text: string, back?: string): void
  goId(id: string): void
  goCover(): void
}

// Обложка, раскладки, удаление, разрез, склейка и имя разворота. Очередь удалений общая: «Вернуть» тоже ждёт её (toasts.svelte.ts).
export class SpreadOps {
  // Щелчки быстрее ответа Go: каждое удаление ждёт предыдущее, а «текущий» берётся уже после него —
  // иначе все щелчки уходят с одним id и удаляется один разворот. run ловит ошибки, цепочка не рвётся.
  delQueue: Promise<unknown> = Promise.resolve()

  constructor(private readonly h: SpreadHost) {}

  // id не задан — текущий разворот на момент выполнения; меню ленты передаёт разворот, по которому щёлкнули.
  deleteSpread(id?: string): Promise<void> {
    const next = this.delQueue.then(() => this.deleteNow(id))
    this.delQueue = next.catch(() => {})
    return next
  }

  // Несколько групп ленты (спека 019 F): один запрос, один шаг Ctrl+Z. Очередь общая с deleteSpread.
  deleteSpreads(ids: readonly string[]): Promise<void> {
    const next = this.delQueue.then(() => this.deleteManyNow(ids))
    this.delQueue = next.catch(() => {})
    return next
  }

  private async deleteManyNow(ids: readonly string[]) {
    const h = this.h
    const gone = h.view.spreads.map((s) => s.id).filter((id) => ids.includes(id))
    if (gone.length === 0) return
    const before = h.view.rev
    if (!(await h.run(Api.DeleteSpreads(gone)))) return
    h.hooks.spreadsCleared()
    const left = new SvelteSet(h.view.spreads.map((s) => s.id))
    const n = gone.filter((id) => !left.has(id)).length
    // Холостая правка (rev тот же): тоста нет, иначе «Вернуть» отменило бы чужую правку.
    if (n > 0 && h.view.rev !== before) h.undoToast(`Удалено разворотов: ${n}`, gone[0])
  }

  private async deleteNow(id?: string) {
    const h = this.h
    // На обложке «Удалить разворот» — это удаление обложки (Go отвечает отказом на разворот с её id).
    if (id === COVER_ID || (!id && h.onCover)) return this.deleteCover()
    const s = id ? h.view.spreads.find((x) => x.id === id) : h.spread
    if (!s) return
    const before = h.view.rev
    // Холостая правка (rev тот же): тоста нет, иначе «Вернуть» отменило бы чужую правку.
    if (!(await h.run(Api.DeleteSpread(s.id))) || h.view.rev === before) return
    h.undoToast('Разворот удалён', s.id)
  }

  // Делит разворот: фото [0:at] остаются, остальные — в новый разворот сразу после него.
  // keep задан — разрез через ручной разворот (103): Go сначала возвращает «Авто» (keep — свой кадр), один шаг отмены.
  async split(id: string, at: number, keep?: boolean) {
    const h = this.h
    const before = h.view.rev
    if (!(await h.run(keep === undefined ? Api.SplitSpread(id, at) : Api.SplitSpreadAuto(id, at, keep)))) return
    if (h.view.rev === before) return
    h.goId(id)
    h.undoToast('Разворот разрезан', id)
  }

  // Клеит разворот со следующим; текущим становится объединённый (второй исчез).
  // keep задан — склейка через ручной разворот (103): «Вернуть авто» ручным из пары и склейка одной правкой.
  async merge(id: string, keep?: boolean) {
    const h = this.h
    const before = h.view.rev
    const i = h.view.spreads.findIndex((s) => s.id === id)
    // Фон второго разворота при склейке снимается, если у первого свой: даём «Вернуть».
    const lost = backdropLost(h.view.spreads[i], h.view.spreads[i + 1])
    if (!(await h.run(keep === undefined ? Api.MergeSpreads(id) : Api.MergeSpreadsAuto(id, keep))) || h.view.rev === before) return
    h.goId(id)
    // Тост, как у разреза (094 §1.4): склейка больше не тихая; снятый фон — второй фразой того же тоста.
    h.undoToast(lost ? 'Развороты склеены. Фон второго разворота снят' : 'Развороты склеены', id)
  }

  // Обложка (032): бросок фото на плитку создаёт её с этим фото на лицевой (photoID '' — пустая) — один шаг отмены.
  // Пока запрос идёт, повторный щелчок по «+ Обложка» ничего не делает (иначе второй вызов — холостая правка с ошибкой).
  private coverMaking = false
  async createCover(photoID: string): Promise<boolean> {
    const h = this.h
    if (h.cover || h.view.settings?.sheet || h.busy || this.coverMaking) return false
    this.coverMaking = true
    try {
      if (!(await h.run(Api.CreateCover('', photoID)))) return false
    } finally {
      this.coverMaking = false
    }
    h.goCover()
    return true
  }

  // «Удалить обложку»: тост с «Вернуть», как у разворотов. Холостая правка (rev тот же) тоста не даёт.
  async deleteCover() {
    const h = this.h
    const before = h.view.rev
    if (!(await h.run(Api.DeleteCover())) || h.view.rev === before) return
    h.undoToast('Обложка удалена', COVER_ID)
  }

  // «Разнообразить раскладки» (041): Go меняет раскладки выбранных разворотов с учётом соседей, один шаг отмены.
  // families — тот же фильтр, что у автосборки (065). Вид без изменений (rev тот же) — тост вместо немого ничего; иначе — тост с «Вернуть».
  async varyLayouts(ids: readonly string[], variety: Variety, families: readonly Family[] = []): Promise<boolean> {
    const h = this.h
    if (ids.length === 0 || h.busy) return false
    const was = new SvelteMap(h.view.spreads.map((s) => [s.id, s.layout]))
    const before = h.view.rev
    if (!(await h.run(Api.VaryLayouts([...ids], variety, [...families])))) return false
    if (h.view.rev === before) {
      h.notify('Менять нечего: раскладки уже разные или других подходящих нет')
      return false
    }
    const n = h.view.spreads.filter((s) => was.has(s.id) && was.get(s.id) !== s.layout).length
    h.undoToast(`Раскладки изменены: ${n}`)
    return true
  }

  // Название разворота; пустое снимает. Правка — шаг undo, то же название — холостая в Go.
  rename(id: string, name: string) {
    return this.h.run(Api.SetSpreadName(id, name))
  }
}
