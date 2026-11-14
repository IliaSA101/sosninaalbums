import { describe, expect, it } from 'vitest'
import { fillKeys, keyMarks } from './keytext'

const hint = (id: string) => ({ undo: 'Ctrl+Z', toggleBig: 'F' })[id] ?? ''

describe('fillKeys', () => {
  it('метка без шаблона: сочетание, а без клавиши — «без клавиши»', () => {
    expect(fillKeys('Нажмите {key:undo}', hint)).toBe('Нажмите Ctrl+Z')
    expect(fillKeys('Нажмите {key:nope}', hint)).toBe('Нажмите без клавиши')
  })
  it('шаблон: % — сочетание; нет клавиши — шаблон опускается целиком', () => {
    expect(fillKeys('Отмена{key:undo| (%)}.', hint)).toBe('Отмена (Ctrl+Z).')
    expect(fillKeys('Отмена{key:nope| (%)}.', hint)).toBe('Отмена.')
    expect(fillKeys('Вид{key:toggleBig|, %; и ещё %}', hint)).toBe('Вид, F; и ещё %')
  })
  it('несколько меток и текст без меток', () => {
    expect(fillKeys('{key:undo} или {key:toggleBig}', hint)).toBe('Ctrl+Z или F')
    expect(fillKeys('Просто текст {не метка}', hint)).toBe('Просто текст {не метка}')
  })
})

describe('keyMarks', () => {
  it('id всех меток текста', () => {
    expect(keyMarks('a {key:undo} b {key:toggleBig|, %} c {key:x1| %}')).toEqual(['undo', 'toggleBig', 'x1'])
    expect(keyMarks('нет меток')).toEqual([])
  })
})
