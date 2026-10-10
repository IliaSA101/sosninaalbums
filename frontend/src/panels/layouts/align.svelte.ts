// Выравнивание выделенного (спека 010 U7): что выделено на текущем развороте, опора «Относительно»
// (настройка окна — localStorage, в проект не попадает) и один вызов Api.AlignItems = один шаг undo.
// Решения «можно ли» — align.ts.
import { Api, store } from '../../base/core/store.svelte'
import { gridAct } from './gridact.svelte'
import { edit } from '../../editor/text/edit.svelte'
import { placesel } from '../../editor/places/placesel.svelte'
import { prefs } from '../../base/prefs/prefs.svelte'
import { alignBlock, isDist, normAlignTo, type AlignOp, type AlignTo } from './align'

const KEY = 'alignTo'

function load(): AlignTo {
  try {
    return normAlignTo(localStorage.getItem(KEY))
  } catch {
    return 'sel'
  }
}

class Align {
  to = $state<AlignTo>(load())

  // То, что принимает Go: id мест (сетка) или фото («Авто»), тексты группы и одиночный текст.
  get ids(): string[] {
    return placesel.targetIds
  }

  setTo(to: AlignTo) {
    this.to = to
    try {
      localStorage.setItem(KEY, to)
    } catch {
      // Без хранилища опора не запомнится.
    }
  }

  // У «Авто» с фото Go закрепляет разворот той же правкой — тост «Вернуть» даёт placeEdit.
  async run(op: AlignOp) {
    const s = store.spread
    const ids = [...this.ids]
    if (!s || !prefs.can('align') || alignBlock(op, this.to, ids.length)) return
    // Ждём начатые отправки текста, иначе сдвиг уйдёт раньше них (как у undo).
    await edit.idle()
    const to = this.to
    const was = store.toast
    if (!(await gridAct.placeEdit(s.id, Api.AlignItems(s.id, ids, op, to))) || !isDist(op)) return
    // Наложение считает Go (после правки ответ тот же). Тост «Вернуть» от этой же
    // правки (закрепление «Авто») важнее подсказки — его не перекрываем.
    const own = store.toast !== was && store.toast?.action
    if (!own && (await Api.AlignOverlap(s.id, ids, op, to))) store.notify('Не помещаются без наложения')
  }
}

export const align = new Align()
