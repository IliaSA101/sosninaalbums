// Сеть окна «Наборы библиотеки» (067 Ф3): типы ответов Go, вопрос согласия и тексты хода и итогов.
// Название набора в ходе — недоверенный текст из подписанного каталога: только как текст.
import { errText } from '../../base/words/errtext'
import { skippedLabel, type LibraryNet, type PackSkip } from './packs'
import { sizeLabel } from './backups'

// Ответы Go (app.LibraryUpdate) и события (library:progress, library:offer); LibraryNet — в packs.ts.
export interface LibraryUpdate {
  added: number
  updated: number
  skipped: PackSkip[] | null
  bytes: number
}
// library:changed: названия (PackInfo.Title) поставленных и обновлённых наборов; при удалении пусто.
export interface LibraryChanged {
  installed: string[] | null
}
export interface LibraryProgress {
  stage: string // catalog | download | install
  title: string
  done: number
  total: number
}
export interface LibraryOffer {
  count: number
  bytes: number
}

export const NO_NET: LibraryNet = { allowed: false, atStart: false, checked: 0, skipped: 0 }

export const CONSENT_TITLE = 'Обратиться к GitHub?'
export const CONSENT_TEXT =
  'Программа скачает список наборов и новые наборы с github.com. Отправляется только обычный запрос: альбомы, фото и сведения о вас не передаются.'
export const START_LABEL = 'Проверять при запуске'
export const START_LABEL_ASK = 'Проверять при запуске (раз в день)'
export const REVOKE_HINT = 'Снять согласие: программа не будет обращаться к сети'
export const REVOKED_TEXT = 'Программа в сеть не обращается. Обновить библиотеку можно снова — она спросит согласие'
export const SKIP_RESET_LABEL = 'Вернуть'
export const SKIP_RESET_HINT = 'Убрать пропуск: удалённые наборы вернутся при следующем обновлении'
export const SKIP_RESET_TEXT = 'Удалённые наборы вернутся при следующем обновлении'
export const skipLine = (n: number): string => `Удалённые вами наборы не возвращаются при обновлении (${n})`
export const CANCELLED_TEXT = 'Обновление отменено'
export const FRESH_TEXT = 'Всё уже свежее'
export const BACKGROUND_TEXT = 'Скачиваем новые наборы…'

// «12 из 40 МБ» — в одних единицах; меньше мегабайта — как sizeLabel.
function bytesOf(done: number, total: number): string {
  if (total < 1048576) return `${sizeLabel(done)} из ${sizeLabel(total)}`
  const mb = (n: number) => String(Math.round(n / 1048576))
  return `${mb(Math.min(done, total))} из ${mb(total)} МБ`
}

// Строка полосы хода: «Скачиваем «Айвори» — 12 из 40 МБ». Не знаем этапа — «Обновляем…».
export function progressText(p: LibraryProgress | null): string {
  if (!p) return 'Обновляем библиотеку…'
  switch (p.stage) {
    case 'catalog':
      return 'Получаем список наборов…'
    case 'download':
      return p.total > 0 ? `Скачиваем «${p.title}» — ${bytesOf(p.done, p.total)}` : `Скачиваем «${p.title}»…`
    case 'install':
      return `Ставим «${p.title}»…`
    default:
      return 'Обновляем библиотеку…'
  }
}

// Доля полосы 0…100; null — неопределённая (список, установка, размер неизвестен).
export function progressPercent(p: LibraryProgress | null): number | null {
  if (!p || p.stage !== 'download' || p.total <= 0) return null
  return Math.max(0, Math.min(100, Math.round((p.done / p.total) * 100)))
}

// Итог «Обновить библиотеку» и проверки при запуске: ничего не поставлено — «Всё уже свежее»; нулевой части в тексте нет.
// Пропущенное (нужна новая программа и т. п.) — в конце.
export function updateText(r: LibraryUpdate): string {
  const skipped = skippedLabel(r.skipped)
  if (r.added + r.updated === 0) return [FRESH_TEXT, skipped].filter(Boolean).join('; ')
  const parts: string[] = []
  if (r.added > 0) parts.push(`новых наборов ${r.added}`)
  if (r.updated > 0) parts.push(r.added > 0 ? `обновлено ${r.updated}` : `обновлено наборов ${r.updated}`)
  return [`Библиотека обновлена: ${parts.join(', ')}`, skipped].filter(Boolean).join('; ')
}

// Предложение при запуске: «Есть новые наборы: 2 (120 МБ)».
export const offerText = (o: LibraryOffer): string => `Есть новые наборы: ${o.count} (${sizeLabel(o.bytes)})`

// «Отменить»: Go отвечает ошибкой «обновление библиотеки отменено» — это не сбой.
export const isCancelled = (e: unknown): boolean => /обновление библиотеки отменено/i.test(errText(e))
