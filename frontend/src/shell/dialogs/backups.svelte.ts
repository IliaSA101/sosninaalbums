import { Api, store } from '../../base/core/store.svelte'
import { edit } from '../../editor/text/edit.svelte'
import { errText, capFirst } from '../../base/words/errtext'
import type { BackupInfo, View } from '../../base/core/types'
import { timeLabel } from './backups'

// Окно «Файл ▸ Резервные копии…» (спека 019 E): список копий и действия. Пока окно открыто, App не исполняет клавиши.
// Ошибки Go показываются в самом окне: тост под затемнением не виден.
class BackupsUi {
  open = $state(false)
  items = $state<BackupInfo[]>([])
  loading = $state(false)
  busy = $state(false) // идёт «Восстановить»
  error = $state('')

  async show() {
    this.open = true
    this.error = ''
    this.items = []
    this.loading = true
    try {
      this.items = ((await Api.Backups()) as unknown as BackupInfo[]) ?? []
    } catch (e) {
      this.error = capFirst(errText(e))
    } finally {
      this.loading = false
    }
  }

  close() {
    this.open = false
  }

  // Заменяет альбом копией одним шагом правки; «Вернуть» в тосте и Ctrl+Z возвращают как было.
  async recover(b: BackupInfo) {
    if (this.busy) return
    this.busy = true
    this.error = ''
    await edit.idle() // набранный текст сначала записывается
    const before = store.view.rev
    try {
      store.apply((await Api.RestoreBackup(b.name)) as unknown as View)
    } catch (e) {
      this.error = capFirst(errText(e))
      return
    } finally {
      this.busy = false
    }
    this.open = false
    // Копия совпала с альбомом: правки не было, «Вернуть» отменило бы предыдущую правку пользователя.
    if (store.view.rev === before) return store.notify('Копия совпадает с альбомом')
    store.undoToast(`Альбом восстановлен из копии от ${timeLabel(b.time)}`)
  }

  async showFolder() {
    this.error = ''
    try {
      await Api.ShowBackups()
    } catch (e) {
      this.error = capFirst(errText(e))
    }
  }
}

export const backupsUi = new BackupsUi()
