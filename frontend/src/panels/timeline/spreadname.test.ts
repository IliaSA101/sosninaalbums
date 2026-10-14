import { describe, expect, it } from 'vitest'
import { NAME_MAX, labelTitle, nameChanged, normName, spreadLabel } from './spreadname'

describe('spreadLabel', () => {
  it('номер и название', () => {
    expect(spreadLabel(0, 'Сборы')).toBe('1 · Сборы')
    expect(spreadLabel(11, 'Банкет')).toBe('12 · Банкет')
  })
  it('без названия — только номер', () => {
    expect(spreadLabel(2, '')).toBe('3')
    expect(spreadLabel(2, undefined)).toBe('3')
  })
})

describe('labelTitle', () => {
  it('в ленте — с подсказкой про двойной щелчок', () => {
    expect(labelTitle(0, 'Сборы', true)).toBe('1 · Сборы\nДвойной щелчок — переименовать')
    expect(labelTitle(0, '', true)).toBe('Двойной щелчок — название разворота')
    expect(labelTitle(0, '', true, true)).toBe('Двойной щелчок — название листа')
  })
  it('в плитках — только название; без названия подсказки нет', () => {
    expect(labelTitle(0, 'Сборы')).toBe('1 · Сборы')
    expect(labelTitle(0, '')).toBeUndefined()
  })
})

describe('normName', () => {
  it('режет пробелы и переводы строк', () => {
    expect(normName('  Сборы  ')).toBe('Сборы')
    expect(normName('А\nБ')).toBe('А Б')
    expect(normName('   ')).toBe('')
  })
  it('не длиннее NAME_MAX знаков, суррогатные пары целиком', () => {
    expect(Array.from(normName('я'.repeat(100)))).toHaveLength(NAME_MAX)
    expect(Array.from(normName('😀'.repeat(100)))).toHaveLength(NAME_MAX)
  })
})

describe('nameChanged', () => {
  it('то же название — не менялось', () => {
    expect(nameChanged('Сборы', ' Сборы ')).toBe(false)
    expect(nameChanged('', '  ')).toBe(false)
    expect(nameChanged(undefined, '')).toBe(false)
  })
  it('новое, очищенное — менялось', () => {
    expect(nameChanged('Сборы', 'Банкет')).toBe(true)
    expect(nameChanged('Сборы', '')).toBe(true)
    expect(nameChanged('', 'Сборы')).toBe(true)
  })
})
