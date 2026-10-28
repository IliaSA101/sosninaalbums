// Действия карточки обложки (спека 032): вызовы Go. Каждая правка — один шаг отмены; свой вид Go присылает сам.
import { Api, store } from '../../base/core/store.svelte'
import type { CoverLayout, CoverPreset, CoverSpec } from '../../base/core/types'
import { edit } from '../../editor/text/edit.svelte'
import { COVER_ID } from '../../base/geom/sheet'
import { CUSTOM_ID, customSpec, spineAction } from './coverpanel'

class CoverActions {
  // Пресеты типографий Go отдаёт списком: читаем один раз и заново после print:changed (123 Б3).
  presets = $state<CoverPreset[]>([])
  private loading = false
  private again = false

  async loadPresets(force = false) {
    if (this.loading) {
      this.again ||= force // реестр сменился, пока шёл прежний запрос: после него — ещё раз
      return
    }
    if (this.presets.length > 0 && !force) return
    this.loading = true
    try {
      this.presets = ((await Api.CoverPresets()) ?? []) as unknown as CoverPreset[]
    } catch {
      // список — удобство: без него остаётся текущая типография, ошибка не должна мешать работе
    } finally {
      this.loading = false
    }
    if (this.again) {
      this.again = false
      await this.loadPresets(true)
    }
  }

  // Список типографий сменился (print:changed, 123 Б3): читаем заново.
  reloadPresets() {
    return this.loadPresets(true)
  }

  applyPreset(id: string) {
    const cv = store.cover
    // «Свои размеры» берут текущие числа, а не сбрасывают их к умолчанию Go; уже они — правки нет.
    if (id === CUSTOM_ID && cv) return cv.preset === CUSTOM_ID ? Promise.resolve(true) : this.setSpec(customSpec(cv))
    return store.run(Api.ApplyCoverPreset(id))
  }

  setSpec(spec: CoverSpec) {
    return store.run(Api.SetCoverSpec(spec as never))
  }

  setLayout(kind: CoverLayout) {
    return store.run(Api.SetCoverLayout(kind))
  }

  // Строка «Корешок», кнопка (058): надписи нет — добавить (один шаг отмены), выделить и сказать тостом;
  // есть — только выделить, без правки.
  async spineText() {
    if (spineAction(store.cover).kind === 'add') {
      if (!(await store.run(Api.CoverSpineText()))) return
      if (!store.cover?.spineText) return
      store.notify('Надпись на корешке добавлена')
    }
    const id = store.cover?.spineText
    if (id) edit.select({ kind: 'text', spread: COVER_ID, id })
  }

  // Цвет плашки корешка; "" — убрать плашку.
  setSpineBg(color: string) {
    return store.run(Api.SetCoverSpineBg(color))
  }
}

export const coverAct = new CoverActions()
