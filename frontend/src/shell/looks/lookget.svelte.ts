// «Скачать и показать» в окне «Макеты альбома» (144, design §7): набор скачивается тем же ходом, что и в «Фонах»
// (decorget.svelte), а макет только выбирается и показывается справа — «Применить» остаётся кнопкой формы (0.27-131-12).
// Окно закрыли, пока шло скачивание, — остаётся тост «Набор скачан».
import { fetchPackItem } from '../../panels/decorbrowse/decorget.svelte'
import { looks } from './looks.svelte'

export async function confirmLookGet(): Promise<void> {
  const a = looks.ask
  if (a?.kind !== 'get') return
  const seq = looks.opened
  looks.dismiss()
  const item = await fetchPackItem(a.tile, 'album', () => looks.open && looks.opened === seq)
  if (item) await looks.showGot(item)
}
