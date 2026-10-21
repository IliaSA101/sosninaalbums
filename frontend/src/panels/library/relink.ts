// «Найти недостающие фото в папке»: полоса MissingBar и меню фото. Диалог выбора папки — в Go.
import { store, Api } from '../../base/core/store.svelte'
import { errText } from '../../base/words/errtext'
import { relinkSummary } from './missing'

export async function relinkFolder() {
  try {
    const r = await Api.RelinkFolder()
    // Отмена диалога: Go вернул всё по нулям — молча.
    const text = relinkSummary(r)
    if (!text) return
    if (!(await store.take(r.view as never))) return // сбой GetView: красный тост цепочки уже показан (087)
    store.notify(text, r.found > 0 ? 'info' : 'error')
  } catch (e) {
    store.notify(errText(e), 'error')
  }
}
