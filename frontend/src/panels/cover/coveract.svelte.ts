// Действия карточки обложки (спека 032): вызовы Go. Каждая правка — один шаг отмены; свой вид Go присылает сам.
import { Api, store } from '../../base/core/store.svelte'
import type { CoverLayout, CoverPreset, CoverSpec } from '../../base/core/types'
import { coverUi } from '../../editor/cover/coverui.svelte'
import { CUSTOM_ID, customSpec } from './coverpanel'

class CoverActions {
  // Пресеты типографий статичны: Go отдаёт их списком, читаем один раз.
  presets = $state<CoverPreset[]>([])
  private loading = false

  async loadPresets() {
    if (this.presets.length > 0 || this.loading) return
    this.loading = true
    try {
      this.presets = ((await Api.CoverPresets()) ?? []) as unknown as CoverPreset[]
    } catch {
      // список — удобство: без него остаётся текущая типография, ошибка не должна мешать работе
    } finally {
      this.loading = false
    }
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

  // «Надпись на корешке»: текст по центру корешка снизу вверх и плашка; повторный вызов Go считает холостым.
  spineText() {
    return store.run(Api.CoverSpineText())
  }

  // Цвет плашки корешка; "" — убрать плашку.
  setSpineBg(color: string) {
    return store.run(Api.SetCoverSpineBg(color))
  }

  // Новый файл разметки: адрес картинки тот же, поэтому кэш сбивается номером правки.
  async pickGuide() {
    const before = store.view.rev
    if ((await store.run(Api.PickCoverGuide())) && store.view.rev !== before) coverUi.guideRev = store.view.rev
  }

  clearGuide() {
    return store.run(Api.ClearCoverGuide())
  }
}

export const coverAct = new CoverActions()
