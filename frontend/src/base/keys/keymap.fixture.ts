// Реестр для тестов: каталог Go по умолчанию (testdata/parity/keymap.json — тот же, что у TestCatalogGolden).
// Без реестра подсказки и диспетчер пусты, поэтому тесты, где клавиши видны или нажимаются, начинаются с useCatalog().
import parity from '../../../../testdata/parity/keymap.json'
import type { KeymapView } from '../types/keys'
import { EMPTY_KEYMAP } from './keyindex'
import { keymap } from './keymap.svelte'

export const catalogView = parity.view as unknown as KeymapView

// Вид со своими клавишами: id -> полный список (пусто — без клавиши); остальное стандартное.
export function catalogWith(own: Record<string, string[]> = {}): KeymapView {
  return {
    ...catalogView,
    commands: catalogView.commands.map((e) => (own[e.id] ? { ...e, keys: own[e.id], custom: true } : e)),
    custom: own,
    changed: Object.keys(own).length,
  }
}

export function useCatalog(own: Record<string, string[]> = {}) {
  keymap.view = catalogWith(own)
}

export function dropCatalog() {
  keymap.view = EMPTY_KEYMAP
}
