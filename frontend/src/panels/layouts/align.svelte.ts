// Выравнивание выделенного (спека 010 U7): что выделено на текущем развороте, опора «Относительно»
// (настройка окна — localStorage, в проект не попадает) и один вызов Api.AlignItems = один шаг undo.
// Решения «можно ли» — align.ts.
import { Api, store } from '../../base/core/store.svelte'
import { gridAct } from './gridact.svelte'
import { edit } from '../../editor/text/edit.svelte'
import { placesel } from '../../editor/places/placesel.svelte'
import { prefs } from '../../base/prefs/prefs.svelte'
import { alignBlock, alignSlot, alignToBlock, isDist, normAlignTo, normAlignToOne, type AlignOp, type AlignTo } from './align'

// Две опоры (спека 100 п. 4). Прежний ключ alignTo — опора для нескольких: выбор пользователя не теряется.
const KEY_MANY = 'alignTo'
const KEY_ONE = 'alignToOne'

function load(key: string, norm: (s: unknown) => AlignTo): AlignTo {
  try {
    return norm(localStorage.getItem(key))
  } catch {
    return norm(null)
  }
}

class Align {
  many = $state<AlignTo>(load(KEY_MANY, normAlignTo))
  one = $state<AlignTo>(load(KEY_ONE, normAlignToOne))

  // Опора по числу выделенных: у одного — своя («Страницы» по умолчанию), у нескольких — своя.
  get to(): AlignTo {
    return alignSlot(this.ids.length) === 'many' ? this.many : this.one
  }

  // То, что принимает Go: id мест (сетка) или фото («Авто»), тексты группы и одиночный текст.
  get ids(): string[] {
    return placesel.targetIds
  }

  // Пишет только опору своего числа выделенных; «Выделения» для одного — отказ (кнопка неактивна).
  setTo(to: AlignTo) {
    const n = this.ids.length
    if (alignToBlock(to, n)) return
    const many = alignSlot(n) === 'many'
    if (many) this.many = to
    else this.one = to
    try {
      localStorage.setItem(many ? KEY_MANY : KEY_ONE, to)
    } catch {
      // Без хранилища опора не запомнится.
    }
  }

  // У «Авто» с фото Go закрепляет разворот той же правкой — тост «Вернуть» даёт placeEdit.
  async run(op: AlignOp) {
    const s = store.spread
    const ids = [...this.ids]
    // Опора зависит от числа выделенных: читаем вместе с ids, до ожидания.
    const to = this.to
    if (!s || !prefs.can('align') || alignBlock(op, to, ids.length)) return
    // Ждём начатые отправки текста, иначе сдвиг уйдёт раньше них (как у undo).
    await edit.idle()
    const was = store.toast
    if (!(await gridAct.placeEdit(s.id, Api.AlignItems(s.id, ids, op, to))) || !isDist(op)) return
    // Наложение считает Go (после правки ответ тот же). Тост «Вернуть» от этой же
    // правки (закрепление «Авто») важнее подсказки — его не перекрываем.
    const own = store.toast !== was && store.toast?.action
    if (!own && (await Api.AlignOverlap(s.id, ids, op, to))) store.notify('Не помещаются без наложения')
  }
}

export const align = new Align()
