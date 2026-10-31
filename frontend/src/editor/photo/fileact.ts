// Действия группы «Файл» (спека 106, 108): «Копировать имя файла» и «Показать в …» — общие для меню фото на холсте
// (PhotoMenu) и меню миниатюры ленты (ThumbMenu): одни тосты и один текст отказа.
import * as Api from '../../../wailsjs/go/app/App'
import { store } from '../../base/core/store.svelte'
import { errText } from '../../base/words/errtext'

// Имена файлов для буфера: только найденные (у пропавших имени во фронте нет), в порядке ids.
export function namesOf(ids: readonly string[], nameOf: (id: string) => string | undefined, gone: (id: string) => boolean): string[] {
  return ids.filter((id) => !gone(id)).flatMap((id) => nameOf(id) ?? [])
}

// В буфер — имена найденных фото, по одному в строке; тост об итоге. gone — «файла нет» (у ленты шире, чем у холста).
export async function copyNames(ids: readonly string[], gone: (id: string) => boolean): Promise<void> {
  const names = namesOf(ids, (id) => store.byId.get(id)?.name, gone)
  if (names.length === 0) return
  try {
    await navigator.clipboard.writeText(names.join('\n'))
    store.notify(names.length > 1 ? 'Имена файлов скопированы' : 'Имя файла скопировано', 'info')
  } catch {
    // Текст отказа буфера приходит от браузера по-английски — свой.
    store.notify('Не удалось скопировать в буфер обмена', 'error')
  }
}

// «Показать в проводнике»: ошибку Go показываем тостом.
export async function reveal(id: string): Promise<void> {
  try {
    await Api.ShowPhotoInExplorer(id)
  } catch (e) {
    store.notify(errText(e), 'error')
  }
}
