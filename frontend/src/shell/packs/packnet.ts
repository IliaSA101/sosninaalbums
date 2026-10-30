// Сеть окна «Наборы библиотеки» (067 Ф3): типы ответов Go, вопрос согласия и тексты хода и итогов.
// Название набора в ходе — недоверенный текст из подписанного каталога: только как текст.
import { errText, capFirst } from '../../base/words/errtext'
import { WHY_MAX, skippedLabel, type LibraryNet, type LibraryOffer, type PackSkip } from './packs'
import { sizeLabel } from '../dialogs/backups'

// Ответы Go (app.LibraryUpdate) и события (library:progress, library:start); LibraryNet, LibraryOffer — в packs.ts.
// Поля после bytes у старого Go нет — читаем с запасом.
export interface LibraryUpdate {
  added: number
  updated: number
  skipped: PackSkip[] | null
  bytes: number
  failed?: PackSkip[] | null // поштучные неудачи хода: имя и причина
  available?: number // строк «Скачать все» после хода
  fresh?: number // строк, которых не было в прежнем списке
  picked?: boolean // «Скачать»: нажатый набор встал
  why?: string // «Скачать»: почему не встал
}
// library:start — проверка при запуске: одно событие на всё (Go: libpacks.Start; в models.ts DTO нет).
export interface LibraryStart {
  updated: number // тихо встало наборов
  offer: LibraryOffer // обновления больше порога — не скачаны
  fresh: LibraryOffer // новые наборы каталога
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

export const NO_NET: LibraryNet = { allowed: false, atStart: false, checked: 0, skipped: 0 }

export const CONSENT_TITLE = 'Обратиться к GitHub?'
export const CONSENT_TEXT =
  'Программа скачает список наборов и роликов, картинки-образцы наборов, новые наборы и выбранное вами с github.com. Отправляется только обычный запрос: альбомы, фото и сведения о вас не передаются.'
export const START_LABEL = 'Проверять при запуске'
export const START_LABEL_ASK = 'Проверять при запуске (раз в день)'
export const REVOKE_HINT = 'Снять согласие: программа не будет обращаться к сети'
export const REVOKED_TEXT = 'Программа в сеть не обращается. Обновить библиотеку можно снова — она спросит согласие'
export const SKIP_RESET_LABEL = 'Вернуть'
export const SKIP_RESET_HINT = 'Снять пометку «удалён вами»: «Скачать все» снова будет их ставить'
export const SKIP_RESET_TEXT = 'Пометка снята: «Скачать все» снова ставит эти наборы'
export const skipLine = (n: number): string => `Удалённые вами наборы «Скачать все» пропускает (${n})`
// Пример темы (142): удалён только он — «Скачать все» к нему отношения не имеет (0.30-142-24).
export const DEMO_SKIP_LINE = 'Пример в программе удалён вами'
export const DEMO_RESET_HINT = 'Вернуть пример в библиотеку'
export const DEMO_BACK_TEXT = 'Пример вернулся'
export const DEMO_BACK_MORE = 'Пример вернулся, «Скачать все» снова ставит остальные наборы'

// Строка под списком: удалён один пример — своя фраза, иначе прежняя с числом.
export const skipText = (n: Pick<LibraryNet, 'skipped' | 'demoSkipped'>): string =>
  n.skipped === 1 && n.demoSkipped ? DEMO_SKIP_LINE : skipLine(n.skipped)
// Подсказка «Вернуть» к ней же: про «Скачать все» — только когда в пропуске есть сетевые наборы.
export const skipHint = (n: Pick<LibraryNet, 'skipped' | 'demoSkipped'>): string =>
  n.skipped === 1 && n.demoSkipped ? DEMO_RESET_HINT : SKIP_RESET_HINT
// Тост после «Вернуть»: пример встал (demoBack из ответа) — «Пример вернулся»; были и другие (skipped до вызова больше 1) — с оговоркой про них.
export const resetText = (was: Pick<LibraryNet, 'skipped'>, now: Pick<LibraryNet, 'demoBack'>): string =>
  now.demoBack ? (was.skipped > 1 ? DEMO_BACK_MORE : DEMO_BACK_TEXT) : SKIP_RESET_TEXT
export const CANCELLED_TEXT = 'Обновление отменено'
export const FRESH_TEXT = 'Всё уже свежее'
export const BACKGROUND_TEXT = 'Скачиваем наборы…'
export const CONSENT_UPDATE = 'Разрешить и обновить'
export const CONSENT_DOWNLOAD = 'Разрешить и скачать'

// «12 из 40 МБ» — в одних единицах; меньше мегабайта — как sizeLabel.
export function bytesOf(done: number, total: number): string {
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

// Что делает ход: «Обновить библиотеку», «Скачать все» или «Скачать» в строке (id и название — для итога и согласия).
export type NetAction = { kind: 'update' } | { kind: 'all' } | { kind: 'one'; id: string; title: string }

// Вопрос согласия зовёт кнопку по действию: после него будет обновление или скачивание.
export const consentLabel = (a: NetAction): string => (a.kind === 'update' ? CONSENT_UPDATE : CONSENT_DOWNLOAD)

// «не скачались: 2 (Школа — скачался с ошибкой, повторите; …)» — как skippedLabel, но «повторите», а не «пропущено».
export function failedLabel(failed: readonly PackSkip[] | null | undefined): string {
  const n = failed?.length ?? 0
  if (!failed || n === 0) return ''
  const why = failed.slice(0, WHY_MAX).map((s) => `${s.name} — ${s.why}`)
  const rest = n - why.length
  return `не скачались: ${n} (${why.join('; ')}${rest > 0 ? ` и ещё ${rest}` : ''})`
}

export type Tone = 'info' | 'error'
// Красный тост — если хоть один набор не скачался.
export const updateTone = (r: LibraryUpdate): Tone => ((r.failed?.length ?? 0) > 0 ? 'error' : 'info')

// Где окно «Наборы»: открыто — раздел виден «ниже»; закрыто — его надо открыть. noNet — ни одного набора из сети не стоит.
export interface UpdateCtx {
  open: boolean
  noNet: boolean
  kind?: 'update' | 'all'
}

// «Библиотека обновлена: новых наборов 2, обновлено 1» — только ненулевые части.
function updatedHead(r: LibraryUpdate): string {
  const parts: string[] = []
  if (r.added > 0) parts.push(`новых наборов ${r.added}`)
  if (r.updated > 0) parts.push(r.added > 0 ? `обновлено ${r.updated}` : `обновлено наборов ${r.updated}`)
  return `Библиотека обновлена: ${parts.join(', ')}`
}

// Итог «Обновить библиотеку» и «Скачать все»: ничего не встало — «Всё уже свежее»; нулевой части в тексте нет.
// Дальше: пропущенное, не скачавшееся, «можно скачать: N — ниже» и (только «Скачать все») «появились новые».
export function updateText(r: LibraryUpdate, ctx: UpdateCtx = { open: true, noNet: false }): string {
  const installed = r.added + r.updated > 0
  const skipped = skippedLabel(r.skipped)
  const failed = failedLabel(r.failed)
  const avail = r.available ?? 0
  const where = ctx.open ? 'ниже' : 'список в окне «Наборы»'
  const fresh = ctx.kind === 'all' && (r.fresh ?? 0) > 0 ? `появились новые: ${r.fresh} — список обновлён` : ''
  // Новичок: наборов из сети нет, список только что получен
  if (!installed && !failed && avail > 0 && ctx.noNet) {
    return [`Список наборов получен: можно скачать ${avail} — ${where}`, skipped, fresh].filter(Boolean).join('; ')
  }
  const head = installed ? updatedHead(r) : failed ? '' : FRESH_TEXT
  const more = avail > 0 ? `можно скачать: ${avail} — ${where}` : ''
  return capFirst([head, skipped, failed, more, fresh].filter(Boolean).join('; '))
}

// Итог «Скачать» в строке: встал нажатый набор — «скачан» (+ дополнения), иначе «не поставлен: причина» от Go.
export function oneText(r: LibraryUpdate, title: string): string {
  if (!r.picked) return r.why ? `Набор «${title}» не поставлен: ${r.why}` : `Набор «${title}» не поставлен`
  const extra = r.added - 1 // сам набор — одна из добавленных
  const head = `Набор «${title}» скачан${extra > 0 ? `, дополнений: ${extra}` : ''}`
  return [head, failedLabel(r.failed)].filter(Boolean).join('; ')
}
// Тон красный только при сбое скачивания; «не встал» без сбоя (версия новее, уже стоит) — обычный тост.
export const oneTone = (r: LibraryUpdate): Tone => ((r.failed?.length ?? 0) > 0 ? 'error' : 'info')

// Тост проверки при запуске одной строкой: «Обновлено наборов: 1; есть новые наборы: 2 (160 МБ)». Нечего сказать — пусто.
export function startText(s: LibraryStart): string {
  const parts: string[] = []
  if (s.updated > 0) parts.push(`обновлено наборов: ${s.updated}`)
  if (s.offer.count > 0) parts.push(`есть обновления наборов: ${s.offer.count} (${sizeLabel(s.offer.bytes)})`)
  if (s.fresh.count > 0) parts.push(`есть новые наборы: ${s.fresh.count} (${sizeLabel(s.fresh.bytes)})`)
  return capFirst(parts.join('; '))
}

// Кнопка тоста: есть предложенные обновления — «Скачать» (UpdateLibrary), иначе новые — «Показать» (окно «Наборы»).
export const startAction = (s: LibraryStart): 'download' | 'show' | null =>
  s.offer.count > 0 ? 'download' : s.fresh.count > 0 ? 'show' : null

// «Отменить»: Go отвечает ошибкой «обновление библиотеки отменено» — это не сбой.
// caps-ok: сверка по тексту, не показ
export const isCancelled = (e: unknown): boolean => /обновление библиотеки отменено/i.test(errText(e))

// Набор пропал из каталога (ErrGone от Go) — не сбой: список уже обновлён, тост обычный.
// caps-ok: сверка по тексту, не показ
export const isGone = (e: unknown): boolean => /набора больше нет в списке/i.test(errText(e))
