import { describe, expect, it } from 'vitest'
import type { UseSrc } from '../../base/words/whereuse'
import { elsewhere, lineAt, useRows, USES_MENU_MAX } from './usemenu'

// Общие строки вхождений меню ленты и меню холста (106, 108).
const use = (spread: string, pick: string, kind = 'place', what = 'место'): UseSrc => ({
  sheet: 0,
  kind,
  spread,
  page: `Разворот ${spread}`,
  what,
  pick,
})

describe('elsewhere: без текущего места', () => {
  const lines = [use('1', 'a'), use('2', 'a'), use('1', 'left', 'backdrop', 'фон'), use('1', 'b')]

  it('убирает только место этого листа с этой записью фото', () => {
    expect(elsewhere(lines, { spread: '1', pick: 'a' })).toEqual([lines[1], lines[2], lines[3]])
  })
  it('фон того же листа и то же фото на другом листе остаются', () => {
    const r = elsewhere(lines, { spread: '1', pick: 'left' })
    expect(r).toHaveLength(4)
  })
  it('нет текущего (лента) или ответа ещё нет — как есть', () => {
    expect(elsewhere(lines)).toBe(lines)
    expect(elsewhere(null, { spread: '1', pick: 'a' })).toBeNull()
  })
  it('единственное вхождение — пустой список', () => {
    expect(elsewhere([use('1', 'a')], { spread: '1', pick: 'a' })).toEqual([])
  })
})

describe('useRows и lineAt', () => {
  it('«…» пока ждём, пояснение при пустом ответе — своё у ленты и холста', () => {
    expect(useRows(null, 'x')).toEqual([{ id: 'wait', label: '…', kind: 'note' }])
    expect(useRows([], 'Больше нигде не стоит')[0].label).toBe('Больше нигде не стоит')
  })
  it('предел строк и «и ещё N»; номер строки ведёт к вхождению', () => {
    const lines = Array.from({ length: USES_MENU_MAX + 2 }, (_, i) => use(String(i + 1), `p${i}`))
    const rows = useRows(lines, '')
    expect(rows).toHaveLength(USES_MENU_MAX + 1)
    expect(rows.at(-1)).toMatchObject({ label: 'и ещё 2', kind: 'note' })
    expect(lineAt(lines, '3')).toMatchObject({ spread: '4', pick: 'p3' })
    expect(lineAt(lines, String(USES_MENU_MAX))).toBeNull()
  })
})
