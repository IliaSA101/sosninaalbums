import { Api, store } from '../../base/core/store.svelte'
import { edit } from '../../editor/text/edit.svelte'
import { errText, capFirst } from '../../base/core/errtext'
import type { NamesResult } from '../../base/core/types'
import { emptyPerson } from './names'
import { sheetIn } from '../../base/geom/sheet'

// Окно «Имена из списка» (030): вставленный список → имена по порядку фото листа. Пока окно открыто,
// App не исполняет клавиши. Ошибки Go — в самом окне (тост под затемнением не виден).
class NamesUi {
  open = $state(false)
  spreadId = $state('')
  text = $state('')
  firstFirst = $state(false) // «Имя первым»: помнится до конца сеанса
  busy = $state(false)
  error = $state('')
  // Есть ли у фото листа имя сейчас (своё или из файла): без него «фото без имени» в предпросмотре не посчитать.
  named = $state<Record<string, boolean>>({})

  async show(spreadId?: string) {
    const sp = sheetIn(store.view, spreadId ?? store.spread?.id ?? '')
    if (!sp) return
    await edit.idle() // набранный текст сначала записывается
    this.spreadId = sp.id
    this.text = ''
    this.error = ''
    this.named = {}
    this.open = true
    const ids = [...sp.photos]
    const got = await Promise.all(
      ids.map((id) =>
        Api.Person(id).then(
          (p) => !emptyPerson(p),
          () => false,
        ),
      ),
    )
    if (this.open && this.spreadId === sp.id) this.named = Object.fromEntries(ids.map((id, i) => [id, got[i]]))
  }

  close() {
    this.open = false
  }

  // Вставка одним шагом отмены; итог — тостом: сколько строк лишних и фото осталось без имени.
  async apply(): Promise<boolean> {
    if (this.busy) return false
    this.busy = true
    this.error = ''
    try {
      const r = (await Api.PasteNames(this.spreadId, this.text, this.firstFirst)) as unknown as NamesResult
      store.apply(r.view)
      this.open = false
      store.notify(`Имена вставлены. Строк лишних: ${r.extra}, фото без имени: ${r.missing}`)
      return true
    } catch (e) {
      this.error = capFirst(errText(e))
      return false
    } finally {
      this.busy = false
    }
  }
}

export const namesUi = new NamesUi()
