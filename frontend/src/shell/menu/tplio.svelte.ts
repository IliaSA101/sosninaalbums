// «Файл ▸ Мои шаблоны ▸ Сохранить в файл… / Загрузить из файла…» (086): диалоги и разбор файла — в Go,
// здесь только вызов и тост с итогом. Отмены (Ctrl+Z) нет: это данные программы, не альбома.
import { Api, store } from '../../base/core/store.svelte'
import { capFirst, errText } from '../../base/words/errtext'
import { vigOwn } from '../../panels/vignette/vigown.svelte'

class TplIo {
  // Растёт после загрузки: у Go событий нет, галерея шаблонов сеток перезапрашивает список по нему
  // (виньетки — по vigOwn.rev, его же поднимаем).
  seq = $state(0)

  // Итог — текст Go как есть («Сохранено шаблонов: 14»); отказ от диалога — молча.
  async save() {
    try {
      const r = await Api.ExportTemplates()
      if (!r.canceled) store.notify(r.text)
    } catch (e) {
      store.notify(capFirst(errText(e)), 'error')
    }
  }

  async load() {
    try {
      const r = await Api.ImportTemplates()
      if (r.canceled) return
      this.seq++
      vigOwn.rev++
      store.notify(r.text)
    } catch (e) {
      store.notify(capFirst(errText(e)), 'error')
    }
  }
}

export const tplio = new TplIo()
