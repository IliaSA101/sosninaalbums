// Фон альбома (098 §3): тексты и решения строки «Фон альбома», окна «Фоны» в этом режиме и карточки листа.
// Что рисовать на листе, считает Go (View.albumBg, album у слоёв, noAlbumBg); здесь — только слова и выбор состояния.
import type { AlbumBg, SpreadView } from '../../base/core/types'
import { isCover, ownBg } from '../../base/geom/sheet'
import type { SegItem } from '../../base/tpl/Seg.svelte'
import type { DecorItem } from '../decor/decor'

export const ALBUM_BG = 'Фон альбома'
export const ALBUM_PICK = 'Выбрать…'
export const ALBUM_CLEAR = 'Убрать'
export const ALBUM_FADE_CAP = 'Прозрачность'
export const ALBUM_CARD_NOW = 'Фон: фон альбома'
export const ALBUM_CARD_EDIT = 'Изменить…'
export const ALBUM_CARD_HIDE = 'Не показывать здесь'
export const ALBUM_CARD_OFF = 'Фон альбома скрыт'
export const ALBUM_CARD_SHOW = 'Показать'
export const ALBUM_CARD_OWN = 'Свой фон: фон альбома здесь не виден'
export const ALBUM_CARD_TO = 'Как в альбоме'
export const ALBUM_CARD_TO_TIP = 'Снять свой узор и фото фона — будет фон альбома'
export const ALBUM_NAME_FALLBACK = 'Свой фон'

// «Куда» в этом режиме: весь разворот или каждая страница (у листа виньетки страниц нет — один пункт, переключатель прячется).
export type AlbumWhere = 'spread' | 'pages'
export function albumWhere(sheet: boolean, spreadAll: string): SegItem<AlbumWhere>[] {
  const all: SegItem<AlbumWhere> = { id: 'spread', label: spreadAll }
  return sheet ? [all] : [all, { id: 'pages', label: 'Каждая страница' }]
}

// Плитка каталога «фоны», что стоит как фон альбома: свой — по ключу библиотеки, узор — по плитке пресета.
export function albumItem(ab: Pick<AlbumBg, 'lib' | 'preset'> | undefined, items: readonly DecorItem[]): DecorItem | undefined {
  const id = ab?.lib || ab?.preset
  return id ? items.find((it) => it.id === id) : undefined
}

// Отмеченная плитка в окне «Фоны»: пусто, если плитки уже нет в каталоге.
export const albumStanding = (ab: Pick<AlbumBg, 'lib' | 'preset'> | undefined, items: readonly DecorItem[]): string =>
  albumItem(ab, items)?.id ?? ''

// Имя в строке: из каталога, иначе то, что назвал Go (имя генератора или файла), иначе общее слово.
export const albumName = (ab: AlbumBg, items: readonly DecorItem[]): string => albumItem(ab, items)?.name || ab.name || ALBUM_NAME_FALLBACK

// Карточка листа: 'on' — фон альбома рисуется («Изменить…» · «Не показывать здесь»), 'off' — скрыт на этом листе («Показать»),
// 'own' — у листа свой фон, фон альбома не виден («Как в альбоме» снимает свой, 115 Г2; и при «скрыт»),
// null — карточки нет (фона альбома нет, обложка).
export type AlbumCard = 'on' | 'off' | 'own' | null
export function albumCard(sp: SpreadView | undefined, ab: AlbumBg | undefined): AlbumCard {
  if (!sp || !ab || isCover(sp)) return null
  // Свой фон перекрывает альбомный: ни «скрыт», ни «Изменить…» тут ничего не меняют на экране — только вернуть альбомный
  if (ownBg(sp.backdrops).length > 0 || ownBg(sp.patterns).length > 0) return 'own'
  if (sp.noAlbumBg) return 'off'
  const drawn = (sp.backdrops ?? []).some((b) => b.album) || (sp.patterns ?? []).some((p) => p.album)
  return drawn ? 'on' : null
}
