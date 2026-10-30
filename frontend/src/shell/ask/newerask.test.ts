import { describe, expect, it } from 'vitest'
import type { NewerInfo } from '../../base/core/types'
import { newerAskText } from './newerask'

const info = (p: Partial<NewerInfo> = {}): NewerInfo => ({
  name: 'Свадьба.salbum',
  draft: false,
  need: '0.26',
  mine: '0.25',
  have: '',
  ...p,
})

describe('newerAskText: альбом', () => {
  it('новой версии на компьютере нет: «Скачать» одна, без «Открыть в»', () => {
    const t = newerAskText(info())
    expect(t.title).toBe('Альбом из новой версии')
    expect(t.lead).toBe('«Свадьба.salbum» сохранён в Sosnina Albums 0.26, а эта программа — 0.25. Открыть его может версия 0.26 или новее.')
    expect(t.next).toBe('Скачайте новую версию — альбомы и настройки сохранятся.')
    expect([t.close, t.download, t.open]).toEqual(['Закрыть', 'Скачать', ''])
  })

  it('версия уже есть: подпись «Открыть в 0.26» и фраза про закрытие программы', () => {
    const t = newerAskText(info({ have: '0.26' }))
    expect(t.next).toBe('Версия 0.26 уже есть на компьютере. Эта программа закроется, альбом откроется в 0.26.')
    expect(t.open).toBe('Открыть в 0.26')
  })

  it('найдена более поздняя, чем нужна: «Открыть в» называет найденную', () => {
    const t = newerAskText(info({ have: '0.27' }))
    expect(t.open).toBe('Открыть в 0.27')
    expect(t.lead).toContain('в Sosnina Albums 0.26')
  })

  it('версия в файле неизвестна: без номеров, только «Скачать»', () => {
    const t = newerAskText(info({ need: '', mine: '' }))
    expect(t.lead).toBe('«Свадьба.salbum» сохранён более новой версией программы.')
    expect(t.open).toBe('')
  })

  it('файл 0.25.1, программа 0.25.1: без номеров и без «Открыть его может версия»', () => {
    const t = newerAskText(info({ need: '0.25.1', mine: '0.25.1' }))
    expect(t.lead).toBe('«Свадьба.salbum» сохранён более новой версией программы.')
    expect(t.lead).not.toContain('Открыть его может')
  })

  it('файл 0.25.0, программа 0.25.1: файл не новее — без номеров', () => {
    expect(newerAskText(info({ need: '0.25.0', mine: '0.25.1' })).lead).toBe('«Свадьба.salbum» сохранён более новой версией программы.')
  })

  it('сравнение по числам: файл 0.10 новее программы 0.9, файл 0.9 — нет', () => {
    expect(newerAskText(info({ need: '0.10', mine: '0.9' })).lead).toContain('в Sosnina Albums 0.10')
    expect(newerAskText(info({ need: '0.9', mine: '0.10' })).lead).not.toContain('0.9')
  })

  it('файл 0.25.2, программа 0.25.1: номера остаются', () => {
    expect(newerAskText(info({ need: '0.25.2', mine: '0.25.1' })).lead).toBe(
      '«Свадьба.salbum» сохранён в Sosnina Albums 0.25.2, а эта программа — 0.25.1. Открыть его может версия 0.25.2 или новее.',
    )
  })

  it('своя версия неизвестна (сборка без номера): без «а эта программа — …»', () => {
    expect(newerAskText(info({ mine: '' })).lead).toBe(
      '«Свадьба.salbum» сохранён в Sosnina Albums 0.26. Открыть его может версия 0.26 или новее.',
    )
  })
})

describe('newerAskText: черновик', () => {
  const draft = (p: Partial<NewerInfo> = {}) => info({ name: '', draft: true, ...p })

  it('заголовок и фраза «здесь он не открывается»', () => {
    const t = newerAskText(draft())
    expect(t.title).toBe('Черновик из новой версии')
    expect(t.lead).toBe(
      'Черновик сохранён в Sosnina Albums 0.26, а эта программа — 0.25. Открыть его может версия 0.26 или новее. Здесь он не открывается и не изменится.',
    )
    expect(t.open).toBe('')
  })

  it('версия есть: запускается без файла — фраза без «альбом откроется»', () => {
    const t = newerAskText(draft({ have: '0.26' }))
    expect(t.next).toBe('Версия 0.26 уже есть на компьютере. Эта программа закроется, запустится 0.26.')
    expect(t.open).toBe('Открыть в 0.26')
  })

  it('версия в файле неизвестна', () => {
    expect(newerAskText(draft({ need: '' })).lead).toBe(
      'Черновик сохранён более новой версией программы. Здесь он не открывается и не изменится.',
    )
  })
})
