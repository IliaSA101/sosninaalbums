import { describe, expect, it } from 'vitest'
import { capFirst, collectDoneText, errText } from './errtext'

describe('errText', () => {
  it('берёт message у Error без префикса', () => {
    expect(errText(new Error('нет файла'))).toBe('нет файла')
    expect(errText(new Error('Error: нет файла'))).toBe('нет файла')
  })
  it('строку отдаёт как есть, срезая «Error: »', () => {
    expect(errText('нельзя склеить')).toBe('нельзя склеить')
    expect(errText('Error: нельзя склеить')).toBe('нельзя склеить')
  })
  it('прочее приводит к строке', () => {
    expect(errText(undefined)).toBe('')
    expect(errText(42)).toBe('42')
  })
})

describe('capFirst', () => {
  it('поднимает первую букву', () => {
    expect(capFirst('мест нет')).toBe('Мест нет')
    expect(capFirst('Уже так')).toBe('Уже так')
    expect(capFirst('')).toBe('')
  })
})

describe('collectDoneText', () => {
  it('успех — обычный тост с «Открыть папку», текст Go как есть', () => {
    expect(collectDoneText('', 'Проект собран: 312 файлов, 6,2 ГБ', false)).toEqual({
      text: 'Проект собран: 312 файлов, 6,2 ГБ',
      kind: 'info',
      open: true,
    })
  })
  it('остановка — красный тост без «Открыть папку»', () => {
    const r = collectDoneText('', 'Сбор остановлен: скопировано 120 из 312.', true)
    expect(r).toEqual({ text: 'Сбор остановлен: скопировано 120 из 312.', kind: 'error', open: false })
  })
  it('ошибка важнее остановки и заметки', () => {
    const r = collectDoneText('Диск заполнен: скопировано 120 из 312. Альбом в папку не записан.', 'x', true)
    expect(r.kind).toBe('error')
    expect(r.text).toContain('Диск заполнен')
    expect(r.open).toBe(false)
  })
  it('Go не прислал note — запасные слова', () => {
    expect(collectDoneText('', '', false).text).toBe('Проект собран')
    expect(collectDoneText('', '', true).text).toBe('Сбор остановлен')
  })
})
