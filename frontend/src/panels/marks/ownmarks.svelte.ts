// Свои отметки фото (спека 004): копия Api.Marks() на фронте. Перечитывается, когда у View меняется
// marksRev, и после открытия/создания альбома (store: invalidate перед apply). Слияние с XMP — marks.ts.
import { Marks, ResetMarks, SetMarks } from '../../../wailsjs/go/app/App'
import type { MarkPatch, MarkView } from '../../base/core/types'

// Что нужно от store для правки отметок (store сам импортирует этот файл — без цикла).
export interface MarkHost {
  readonly selectedIds: string[]
  run(p: Promise<unknown>): Promise<boolean>
  notify(text: string): void
}

class OwnMarks {
  // Карта заменяется целиком — глубокая реактивность не нужна.
  map = $state.raw<Record<string, MarkView>>({})
  // marksRev, под который отметки уже запрошены; −1 — запросить при ближайшем sync.
  private seen = -1
  // Номер последнего запроса: ответ более раннего отбрасывается.
  private req = 0

  // Другой альбом: старые отметки не показываем, даже если Marks() упадёт; ответ в пути отбрасывается.
  invalidate() {
    this.seen = -1
    this.req++
    this.map = {}
  }

  sync(rev: number | undefined) {
    const r = rev ?? 0
    if (r !== this.seen) void this.load(r)
  }

  // Отметки выделенных видимых фото: один шаг undo; без выделения — тост. Ошибку Go («слишком много
  // своих отметок») показывает run. resetMarks — «Как в Lightroom»: снова действуют отметки из файлов.
  setMarks(host: MarkHost, patch: MarkPatch) {
    return this.op(host, (ids) => SetMarks(ids, patch))
  }

  resetMarks(host: MarkHost) {
    return this.op(host, (ids) => ResetMarks(ids))
  }

  private async op(host: MarkHost, call: (ids: string[]) => Promise<unknown>): Promise<boolean> {
    const ids = host.selectedIds
    if (ids.length > 0) return host.run(call(ids))
    host.notify('Выделите фото в ленте, чтобы поставить отметку')
    return false
  }

  private async load(rev: number) {
    this.seen = rev
    const n = ++this.req
    try {
      const m = (await Marks()) as Record<string, MarkView> | null
      if (n === this.req) this.map = m ?? {}
    } catch {
      // отметки вторичны: остаются прежние, ближайший sync перечитает
      if (n === this.req) this.seen = -1
    }
  }
}

export const ownMarks = new OwnMarks()
