// Пустой холст альбома (U2): что написать и какая кнопка главная. Чистая логика, без Svelte.

export interface EmptyCanvas {
  // Строки подсказки (между ними <br>).
  hint: string[]
  // С фото главная — «Автосборка…», «пустой разворот» уходит в ссылку; без фото — как раньше.
  primary: 'auto' | 'spread'
}

export const AUTO_LABEL = 'Автосборка…'

export function emptyCanvas(big: boolean, photos: number): EmptyCanvas {
  const primary = photos > 0 ? 'auto' : 'spread'
  if (big) return { hint: ['Выключите крупный просмотр (F), чтобы добавить фото'], primary }
  if (photos > 0) return { hint: ['Выделите фото в ленте и перетащите сюда', 'или соберите альбом автоматически'], primary }
  return { hint: ['Сначала добавьте папку с фотографиями (кнопка внизу)', 'или перетащите папку в окно'], primary }
}
