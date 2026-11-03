// Действия блока «Разметка типографии» (032, 066): вызовы Go. Выбор и «Убрать» — по одному шагу отмены; файл
// выбирает Go своим диалогом, путь от интерфейса не принимается.
import { Api, store } from '../../base/core/store.svelte'
import type { GuideView } from '../../base/core/types'
import type { MarkupKind } from '../../editor/markup/markup'
import { markupUi } from '../../editor/markup/markupui.svelte'

const guideOf = (kind: MarkupKind): GuideView | null =>
  kind === 'cover' ? (store.cover?.markup ?? null) : (store.view.spreadMarkup ?? null)

class MarkupActions {
  async pick(kind: MarkupKind) {
    const before = guideOf(kind)?.key ?? ''
    if (!(await store.run(kind === 'cover' ? Api.PickCoverGuide() : Api.PickSpreadGuide()))) return
    // Другой файл (key) картинку перечитает сам (markupUi.track). Тот же файл выбрали заново — Go сбросил кэш,
    // key тот же, адрес тот же, поэтому номер сбиваем здесь.
    const after = guideOf(kind)?.key ?? ''
    if (after && after === before) markupUi.bump(kind, store.view.rev)
  }

  clear(kind: MarkupKind) {
    return store.run(kind === 'cover' ? Api.ClearCoverGuide() : Api.ClearSpreadGuide())
  }
}

export const markupAct = new MarkupActions()
