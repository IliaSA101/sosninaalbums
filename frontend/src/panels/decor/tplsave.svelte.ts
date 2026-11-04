// «Сохранить разворот…» (067 Ф1): окно с именем, областью, темой и флажками. Снимок делает Go (SaveSpreadTpl),
// альбом не меняется — отмены нет, как у «Сохранить как шаблон». Окно живёт поверх окна «Развороты».
import { Api, store } from '../../base/core/store.svelte'
import { errText } from '../../base/words/errtext'
import type { BackdropScope } from '../../base/core/types'
import { cleanName, NAME_MAX } from './decor'
import { decor } from './decor.svelte'
import { SAVED, saveTheme } from './spreadtpl'

class SpreadSave {
  open = $state(false)
  name = $state('')
  scope = $state<BackdropScope>('spread')
  theme = $state('mine')
  places = $state(true) // «Места для фото» включены, надписи — нет (сц. 4)
  texts = $state(false)
  busy = $state(false)

  // Из окна «Развороты»: область — выбранная там «Куда», тема — открытая (выборки — «Мои»).
  show() {
    const c = decor.cur
    if (!c || c.kind !== 'spread') return
    this.name = ''
    this.scope = c.scope
    this.theme = saveTheme(decor.theme)
    this.places = true
    this.texts = false
    this.open = true
  }

  close() {
    this.open = false
  }

  async commit(): Promise<boolean> {
    const c = decor.cur
    if (!this.open || !c || this.busy) return false
    const name = cleanName(this.name).slice(0, NAME_MAX)
    if (!name) return false // пустое имя: кнопка серая, Enter в пустом поле молчит
    this.busy = true
    try {
      await Api.SaveSpreadTpl(c.spread, this.scope, this.theme, name, this.places, this.texts)
    } catch (e) {
      store.notify(errText(e), 'error') // «Сохранять нечего», «уже есть в библиотеке», библиотека недоступна
      return false
    } finally {
      this.busy = false
    }
    this.open = false
    store.notify(SAVED)
    if (decor.cur) {
      decor.setTheme(this.theme)
      await decor.refresh()
    }
    return true
  }
}

export const spreadSave = new SpreadSave()
