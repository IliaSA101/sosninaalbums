// Раздел «Можно скачать» окна «Наборы» (137): подписи строк и ссылки. Выбор строк, размеры, серое и «удалён вами» — от Go.
// Название, описание, лицензия и названия дополнений — недоверенный текст из подписанного каталога: только как текст.
import { capFirst } from '../../base/words/errtext'
import { sizeLabel } from '../dialogs/backups'
import { EMPTY_TEXT, metaLine, onlyBuiltin, type LibraryOffer, type PackAvail, type PackInfo } from './packs'

export const AVAIL_TITLE = 'Можно скачать'
export const AVAIL_DOWNLOAD = 'Скачать'
export const REMOVED_NOTE = 'удалён вами'
// До первой проверки списка нет: вместо раздела приглашение (EMPTY_TEXT — только когда список есть и пуст).
export const NOT_LISTED_TEXT = 'Наборы из интернета появятся здесь после «Обновить библиотеку»'

// «версия 1 · 12 фонов · 30 стикеров · 41 МБ» — как у стоящих; размер — вместе с дополнениями.
export const availMeta = (a: PackAvail): string => metaLine({ ...a, origin: 'net' })

// Вторая строка: «Лицензия: CC0-1.0 · дополнение к «Свадьба» · удалён вами · вместе с: «Свадьба: макеты»». Пустое пропускается.
export function availNote(a: PackAvail): string {
  const parts: string[] = []
  if (a.license) parts.push(`Лицензия: ${a.license}`)
  if (a.donor) parts.push(`дополнение к «${a.donor}»`)
  if (a.removed) parts.push(REMOVED_NOTE)
  const add = a.addonTitles ?? []
  if (add.length > 0) parts.push(`вместе с: ${add.map((t) => `«${t}»`).join(', ')}`)
  return parts.join(' · ')
}

// Почему «Скачать» серая («Нужна версия Sosnina Albums 0.30.0 или новее»): текстом в строке и подсказкой кнопки.
export const availWhy = (a: PackAvail): string => capFirst(a.why)

// Подсказка «Скачать»: серая — причина, иначе «Скачать набор «Школа» (41,0 МБ)».
export const availHint = (a: PackAvail): string => (a.why ? availWhy(a) : `Скачать набор «${a.title}» (${sizeLabel(a.bytes)})`)

// Ссылка «Скачать все (5, 380 МБ)»; нечего ставить (N = 0) — ссылки нет. Число и размер — PackList.all от Go.
export const downloadAllLabel = (all: LibraryOffer | null | undefined): string =>
  all && all.count > 0 ? `Скачать все (${all.count}, ${sizeLabel(all.bytes)})` : ''

// «по списку от 27.10» — день последней проверки (местное время); списка нет — пусто.
export function availAtLabel(at: number | null | undefined): string {
  if (!at || at <= 0) return ''
  const d = new Date(at)
  const two = (n: number) => String(n).padStart(2, '0')
  return `по списку от ${two(d.getDate())}.${two(d.getMonth() + 1)}`
}

// Что вместо строк раздела: приглашение до первой проверки; список есть, но пуст и стоят одни встроенные — «Других наборов нет».
export function availEmpty(packs: readonly PackInfo[], availAt: number, rows: number): string {
  if (availAt <= 0) return NOT_LISTED_TEXT
  return rows === 0 && onlyBuiltin(packs) ? EMPTY_TEXT : ''
}
