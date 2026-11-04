// Подгон по лицам (130): кнопка карточки виньетки. Состояние поиска — здесь по ID листа, вне {#key} карточки:
// карточка пересоздаётся при смене листа, а поиск идёт секунды и должен пережить её.
// Go ищет лица по одному за раз, поэтому busy общий; ход «Лица 12 / 25» показывает только лист, где идёт поиск.
import { store } from '../../base/core/store.svelte'
import type { View } from '../../base/core/types'
import { errText } from '../../base/words/errtext'
import { framelive } from '../../editor/photo/framelive.svelte'
import { placesel } from '../../editor/places/placesel.svelte'
import { gridAct } from '../layouts/gridact.svelte'
import { fitFaces, onFacesProgress, type FaceReport, type FacesMode } from './facesapi'
import { FACES_ASK, FACES_TOAST_MS, faceShowIds, faceText } from './vigfaces'

class VigFaces {
  // Идущий поиск: лист и ход. null — свободно.
  private run = $state<{ id: string; done: number; total: number } | null>(null)

  get busy(): boolean {
    return this.run !== null
  }

  // Ход для листа id: null, если поиск идёт на другом листе или не идёт.
  progress(id: string): { done: number; total: number } | null {
    return this.run && this.run.id === id ? { done: this.run.done, total: this.run.total } : null
  }

  // Нажатие кнопки. Повторное нажатие во время поиска — молча пропускается (кнопка aria-disabled, Go ответил бы ошибкой).
  async fit(id: string, mode: FacesMode = '', ref = ''): Promise<boolean> {
    if (this.run) return false
    this.run = { id, done: 0, total: 0 }
    // Ревизия до поиска: событие "view" от Go приходит раньше ответа и уже поднимает store.view.rev.
    const revBefore = store.view.rev
    const off = onFacesProgress((done, total) => {
      if (this.run) this.run = { id, done, total }
    })
    try {
      const r = await fitFaces(id, ref, mode)
      framelive.bumpNorm() // норма листа могла смениться («как это лицо»): силуэт перезапросит форму
      if (r.report.asked) {
        this.ask(id, ref, r.report)
        return true
      }
      // Вид из ответа принимает store.take, как store.run у других правок (приведение: типы биндингов строже ручных).
      // Go ничего не изменил, если ревизия ответа равна той, что была до вызова (правку во время поиска ловит Go: «изменили»).
      const idle = r.view.rev === revBefore
      if (!(await store.take(r.view as unknown as View))) return false
      this.toast(id, r.report, idle)
      return true
    } catch (e) {
      store.notify(errText(e), 'error')
      return false
    } finally {
      off()
      this.run = null
    }
  }

  // Правленые кадры есть: окно 098. «Подогнать» с галочкой — mode «keep», без неё — «all»; «Отмена» ничего не меняет. Повтор идёт с тем же листом и образцом (ref).
  private ask(id: string, ref: string, r: FaceReport) {
    gridAct.openAsk({ kind: 'faces', ...FACES_ASK, ids: [id], run: (keep) => this.fit(id, keep ? 'keep' : 'all', ref) }, r.manual)
  }

  // Итог: одна строка на 20 с; «Показать» — если есть что проверить или что не по норме.
  private toast(id: string, r: FaceReport, idle: boolean) {
    const ids = faceShowIds(r)
    const action = ids.length > 0 ? { label: 'Показать', run: () => this.show(id, ids) } : undefined
    store.notify(faceText(r, idle), 'info', action, FACES_TOAST_MS)
  }

  // «Показать»: с другого листа — сначала переход, затем выделение мест первых двух групп.
  show(id: string, ids: string[]) {
    if (store.currentId !== id) store.goId(id)
    placesel.select(ids)
  }
}

export const vigFaces = new VigFaces()
