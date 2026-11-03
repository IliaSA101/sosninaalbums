// Загрузка и «Убрать» своих шрифтов из списка «Шрифт» (082, design §9). Логика без Svelte — под тесты;
// FontPicker только зовёт её и держит окно вопроса. Не правки альбома: шагов отмены нет (design §10),
// кроме постановки единственной загруженной гарнитуры тексту — это обычный onpick.
import type { FontImport, FontFamily, FontUse } from '../../base/core/types'
import { capFirst, errText } from '../../base/words/errtext'
import { importSummary } from '../../base/words/importsummary'
import { HideFont, ImportFonts } from './fontapi'
import { noCyrTail, settleImport } from './fonts'

export type Notify = (text: string, kind?: 'info' | 'error') => void

// Своя гарнитура: id Go начинается с «my:» (FamilyID), группа списка — 'my'.
export const isOwnId = (id: string) => id.startsWith('my:')

export const MISSING_TIP = 'Нет на этом компьютере — вместо него Montserrat'
export const UNKNOWN_NAME = 'Свой шрифт'
export const UPLOAD_LABEL = 'Загрузить свой шрифт…'
export const HIDE_TITLE = 'Убрать из библиотеки'
export const LAT_ONLY = 'только латиница'

// Текст вопроса «Убрать из библиотеки?» — как у фонов (deleteText), но не импортом из panels/decor.
export const hideText = (name: string): string => `«${name}» будет убран из библиотеки. Где он уже стоит, не изменится.`

// Подпись кнопки шрифта: гарнитура есть — её имя; нет, а она своя (в альбоме или id «my:…») — имя из View.albumFonts
// с « (нет)», имени нет — «Свой шрифт (нет)»; чужой неизвестный id — как есть (старое поведение).
export function shownFont(
  value: string,
  fonts: readonly FontFamily[],
  used: readonly FontUse[] | undefined,
): { name: string; missing: boolean } {
  const f = fonts.find((x) => x.id === value)
  if (f) return { name: f.name, missing: false }
  const u = used?.find((x) => x.id === value)
  if (!u && !isOwnId(value)) return { name: value, missing: false }
  return { name: `${u?.name || UNKNOWN_NAME} (нет)`, missing: true }
}

// Тост загрузки: «Добавлено: 2, пропущено: 1 (…)»; для гарнитур без русских букв — хвост (082-4). Закрыли окно, ничего
// не выбрав — null.
export function uploadToast(res: Pick<FontImport, 'added' | 'skipped'>): { text: string; error: boolean } | null {
  const sum = importSummary(res)
  if (!sum) return null
  const tail = noCyrTail(res.added)
  return { text: tail ? `${sum.text}. ${capFirst(tail)} — они будут шрифтом Montserrat` : sum.text, error: sum.error }
}

export interface UploadDeps {
  fonts: () => FontFamily[] // свежий список (после события `fonts`)
  notify: Notify
  pick: (id: string) => void
}

// «Загрузить свой шрифт…»: диалог Go → проверка окном (откат отказавших) → тост → единственную гарнитуру
// сразу ставят тексту (082-20). Возвращает true, если шрифт поставлен.
export async function uploadFonts(d: UploadDeps): Promise<boolean> {
  try {
    const imp = await ImportFonts()
    if (imp.added.length === 0 && imp.skipped.length === 0) return false
    const res = await settleImport(imp, d.fonts) // геттер: список читается после проверки окном, не до неё
    const t = uploadToast(res)
    if (t) d.notify(t.text, t.error ? 'error' : 'info')
    // Ровно одна гарнитура до проверки и после неё; событие `fonts` могло не прийти — ставим, только если она в списке.
    const one = imp.added.length === 1 && res.added.length === 1 ? res.added[0] : null
    if (!one || !d.fonts().some((f) => f.id === one.id)) return false
    d.pick(one.id)
    return true
  } catch (e) {
    // ErrReadOnly и прочие ошибки Go (до диалога) — тостом.
    d.notify(errText(e), 'error')
    return false
  }
}

// «Загрузить шрифт…» в полосе «Нет шрифтов» (082-7): тот же путь и тот же тост, но тексту ничего не ставим —
// Go сам пересчитает ширины. Один источник текста тоста и разбора ошибок со списком «Шрифт».
export const uploadFontsNoPick = (d: Omit<UploadDeps, 'pick'>): Promise<boolean> => uploadFonts({ ...d, pick: () => {} })

// × у своей гарнитуры (после «Убрать»): файл остаётся, тексты не меняются; список обновит событие `fonts`.
export async function hideFont(id: string, notify: Notify): Promise<boolean> {
  try {
    await HideFont(id)
    return true
  } catch (e) {
    notify(errText(e), 'error')
    return false
  }
}
