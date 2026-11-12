// Куда ходят поля имени (PersonFields): имя фото или табличка пустого места (071). Вызовы Go — здесь,
// чтобы выбор метода проверялся тестом без компонента.
import { Api, store } from '../../base/core/store.svelte'
import type { Person, PersonView } from '../../base/core/types'

// Пустое место разворота: табличка живёт у места, а не у фото (спека 071).
export interface PlaceRef {
  spread: string
  id: string
}

export interface PersonTarget {
  photo: string
  place?: PlaceRef
}

// Подсказки под полями фото, стоящего на месте с табличкой: правка меняет табличку, а не имя фото.
export const SEAT_NOTE = 'Имя места — останется здесь, если убрать фото'
// Табличка только с предметом (071-16): имя здесь принадлежит фото, на месте остаётся лишь предмет.
export const SEAT_SUBJECT_NOTE = 'Предмет места — останется здесь, если убрать фото'

// Действующее имя: табличка места или имя фото (с табличкой на месте — она же, place = ID места).
export const loadPerson = (t: PersonTarget): Promise<PersonView> =>
  (t.place ? Api.PlacePerson(t.place.spread, t.place.id) : Api.Person(t.photo)) as unknown as Promise<PersonView>

// Правка одним шагом отмены на сессию набора; у фото на месте с табличкой Go сам направит её в табличку.
export const savePerson = (t: PersonTarget, p: Person, session: string): Promise<boolean> =>
  store.run(t.place ? Api.SetPlacePerson(t.place.spread, t.place.id, p, session) : Api.SetPerson(t.photo, p, session))

// Текст подсказки: поля фото, а табличка видна (у самого места подсказка не нужна); '' — не показывать.
// seatNamed приходит от Go: в табличке есть имя; иначе на месте только предмет (в том числе когда у фото своё имя).
export const seatNoteText = (t: PersonTarget, pv: Pick<PersonView, 'place' | 'seatNamed'>): string => {
  if (t.place || !pv.place) return ''
  return pv.seatNamed ? SEAT_NOTE : SEAT_SUBJECT_NOTE
}

// Что показать в полях карточки по ответу Go. «из имени файла» (fromFile) идёт от Go как есть — в том
// числе под табличкой только с предметом: она не гасит пометку.
export const cardOf = (t: PersonTarget, pv: PersonView) => ({
  draft: { last: pv.last, first: pv.first, middle: pv.middle, subj: pv.subject },
  fromFile: pv.fromFile,
  note: seatNoteText(t, pv),
})
