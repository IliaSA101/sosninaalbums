// Меню «⋯» группы «Виньетка» карточки листа (спека 124 U5): два редких действия ушли из тела группы. Слова и подсказки
// прежние (были кнопками панели). Чистая часть без Go; действия по id — в InspectorSpread.
import type { MenuNode } from '../../base/ui/menulist'

export const VIG_SAVE = 'vig-save'
export const VIG_CLEAR = 'vig-clear'

// Подсказки действий виньетки: одни слова у кнопки карточки, пункта «⋯» и пункта меню «Разворот» (141).
export const VIG_SORT_TITLE = 'Ученики по алфавиту: фамилия, имя; без имени — в конце'
export const VIG_CLEAR_TITLE = 'Места, фото и подписи остаются, раскладка становится «Ручной»; пропадают только параметры виньетки'

// templates — функция «Свои шаблоны» на уровне; без неё остаётся один пункт. one — «разворот» / «лист».
export function vigMore(one: string, templates: boolean): MenuNode[] {
  const save: MenuNode = {
    id: VIG_SAVE,
    label: 'Сохранить как шаблон…',
    title: 'Запомнить места, шапку и стиль подписей этого листа как свой шаблон: потом его можно поставить на другой лист',
  }
  const clear: MenuNode = {
    id: VIG_CLEAR,
    label: `Обычный ${one}`,
    title: VIG_CLEAR_TITLE,
  }
  return templates ? [save, clear] : [clear]
}
