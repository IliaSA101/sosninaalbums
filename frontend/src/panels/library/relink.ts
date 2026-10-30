// «Найти недостающие фото в папке»: полоса MissingBar и меню фото. Диалог выбора папки — в Go.
import { store, Api } from '../../base/core/store.svelte'
import { errText } from '../../base/words/errtext'
import { relinkSummary } from './missing'
import { relinking } from './relinkbusy.svelte'

export async function relinkFolder() {
  relinking.on = true // агентам на это время — «занят» (149-32)
  try {
    const r = await Api.RelinkFolder()
    // Отмена диалога: Go вернул всё по нулям — молча.
    const text = relinkSummary(r)
    if (!text) return
    if (!(await store.take(r.view as never))) return // сбой GetView: красный тост цепочки уже показан (087)
    store.notify(text, r.found > 0 ? 'info' : 'error')
  } catch (e) {
    store.notify(errText(e), 'error')
  } finally {
    relinking.on = false
  }
}
