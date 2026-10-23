// Окно «О программе» (101): флаг открытия и открытие ссылок. Адреса зашиты в Go (App.OpenLink), здесь только id.
import { Api, store } from '../../base/core/store.svelte'
import { errText } from '../../base/words/errtext'

export type LinkId = 'source' | 'license' | 'telegram' | 'marina' | 'bug' | 'idea'

// Условие 7(b) лицензии (NOTICE.md): изменённые версии показывают эту строку и адрес в этом окне.
export const BASED_ON = 'Основано на Sosnina Albums, автор Илья Соснин'
export const REPO_TEXT = 'github.com/IliaSA101/sosninaalbums'

class AboutUi {
  open = $state(false)

  show() {
    this.open = true
  }

  close() {
    this.open = false
  }

  // Окно остаётся открытым (как «Открыть Фотосферу» в «Готово»); сбой — тостом.
  async link(id: LinkId) {
    try {
      await Api.OpenLink(id)
    } catch (e) {
      store.notify(errText(e), 'error')
    }
  }
}

export const aboutUi = new AboutUi()
