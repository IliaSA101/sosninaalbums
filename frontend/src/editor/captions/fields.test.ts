import { describe, expect, it } from 'vitest'
import { fieldsOf, insertAt, token } from './fields'

describe('fields', () => {
  it('fieldsOf: нет списка от Go — три имени с фото; есть — как пришёл (семь на виньетке)', () => {
    expect(fieldsOf(undefined)).toEqual(['Фамилия', 'Имя', 'Отчество'])
    expect(fieldsOf({})).toEqual(['Фамилия', 'Имя', 'Отчество'])
    const seven = ['Фамилия', 'Имя', 'Отчество', 'Заголовок', 'Подзаголовок', 'Год', 'Руководитель']
    expect(fieldsOf({ fields: seven })).toEqual(seven)
    expect(token('Заголовок')).toBe('{Заголовок}')
  })

  it('вставляет в позицию курсора', () => {
    expect(insertAt('ab', 1, 1, 'Имя')).toEqual({ value: 'a{Имя}b', caret: 6 })
  })

  it('заменяет выделенное, границы можно перепутать', () => {
    expect(insertAt('abcd', 3, 1, 'Год')).toEqual({ value: 'a{Год}d', caret: 6 })
  })

  it('границы зажимаются', () => {
    expect(insertAt('ab', -5, 99, 'Класс').value).toBe('{Класс}')
    expect(insertAt('', 0, 0, 'Имя')).toEqual({ value: '{Имя}', caret: 5 })
    expect(insertAt('ab', 9, 9, 'Имя').value).toBe('ab{Имя}')
  })
})
