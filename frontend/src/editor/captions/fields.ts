// Поля подписей (030): имена, которые Go подставляет вместо `{Имя}`. Чистая логика под тесты.
// Три имени с фото есть всегда; поля шапки (Заголовок, Подзаголовок, Год, Руководитель) — на листах, где они
// осмысленны (031 dialog §4): список приходит из Go в SpreadView.fields. Регистр в тексте не важен.
export const PERSON_FIELDS: readonly string[] = ['Фамилия', 'Имя', 'Отчество']
export type Field = string

// Имена для меню «Вставить поле» на листе; Go ещё не прислал список (или листа нет) — три имени с фото.
export const fieldsOf = (sp: { fields?: readonly string[] } | null | undefined): readonly Field[] => sp?.fields ?? PERSON_FIELDS

export const token = (f: Field): string => `{${f}}`

// Вставка поля в строку на место выделения [from, to) (границы можно перепутать): новое значение и
// позиция курсора после вставки. Выход за границы зажимается.
export function insertAt(value: string, from: number, to: number, f: Field): { value: string; caret: number } {
  const lo = Math.min(from, to)
  const hi = Math.max(from, to)
  const a = Math.max(0, Math.min(lo, value.length))
  const b = Math.max(a, Math.min(hi, value.length))
  const t = token(f)
  return { value: value.slice(0, a) + t + value.slice(b), caret: a + t.length }
}
