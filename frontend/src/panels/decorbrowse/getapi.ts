// Вызовы Go для плиток «Можно скачать» (144): тонкий слой над биндингами, как frameApi и lookApi.
import { Api } from '../../base/core/store.svelte'
import type { DecorItem } from '../decor/decor'

export const getApi = {
  // Докачка картинок-образцов фоном: Go отвечает сразу, без согласия ничего не делает. Окно зовёт её при открытии;
  // из обработчика library:previews — нельзя (петля: докачка шлёт то же событие). Сбой окну не помеха — образцов просто не будет.
  previews: (): void => {
    try {
      Api.FetchPackPreviews().catch(() => {})
    } catch {
      // метода нет (подмена Go в тесте, старая сборка) — образцов просто не будет
    }
  },
  // Плитка предмета только что скачанного набора (kind: bg | sticker | frame | spread | album); нет предмета — ошибка Go.
  item: (kind: string, pack: string, src: string): Promise<DecorItem> => Api.PackItem(kind, pack, src) as unknown as Promise<DecorItem>,
}
