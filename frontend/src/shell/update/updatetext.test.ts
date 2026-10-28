import { describe, expect, it } from 'vitest'
import { NO_UPDATE, type UpdateInfo } from './updateapi'
import { failedText, justText, offerToast, progressLine, progressPct, readyToast, retryAct, startedText, updateView } from './updatetext'

const info = (p: Partial<UpdateInfo>): UpdateInfo => ({ ...NO_UPDATE, mode: 'install', ...p })
const labels = (i: UpdateInfo) => updateView(i).btns.map((b) => b.label)

describe('updateView: текст и кнопки по фазе (design §3.1)', () => {
  it('idle: строки нет, одна кнопка «Проверить обновления»', () => {
    const v = updateView(info({ phase: 'idle' }))
    expect(v.line).toBe('')
    expect(labels(info({ phase: 'idle' }))).toEqual(['Проверить обновления'])
    expect(v.pageLink).toBe(false)
  })

  it('checking: «Проверяем…», кнопка неактивна', () => {
    const v = updateView(info({ phase: 'checking' }))
    expect(v.line).toBe('Проверяем…')
    expect(v.btns).toEqual([{ act: 'check', label: 'Проверить обновления', disabled: true }])
  })

  it('latest: «Установлена последняя версия» и снова «Проверить»', () => {
    const v = updateView(info({ phase: 'latest' }))
    expect(v.line).toBe('Установлена последняя версия')
    expect(v.btns[0]).toMatchObject({ act: 'check', disabled: false })
  })

  it('available, install и setup: «Обновить», «Что нового», ссылка «Страница выпуска»', () => {
    for (const mode of ['install', 'setup'] as const) {
      const v = updateView(info({ phase: 'available', mode, version: '0.25.1', notes: ['а', 'б'] }))
      expect(v.line).toBe('Вышла версия 0.25.1')
      expect(v.btns).toEqual([
        { act: 'check', label: 'Проверить обновления', disabled: false },
        { act: 'download', label: 'Обновить', primary: true },
      ])
      expect(v.notes).toEqual(['а', 'б'])
      expect(v.pageLink).toBe(true)
    }
  })

  it('available, page (Mac, Linux): «Скачать» открывает страницу, второй ссылки нет', () => {
    const v = updateView(info({ phase: 'available', mode: 'page', version: '0.25.1' }))
    expect(v.btns).toEqual([
      { act: 'check', label: 'Проверить обновления', disabled: false },
      { act: 'page', label: 'Скачать', primary: true },
    ])
    expect(v.pageLink).toBe(false)
    expect(v.notes).toEqual([])
  })

  it('downloading: «3 из 8 МБ», полоса и «Отменить»', () => {
    const v = updateView(info({ phase: 'downloading', version: '0.25.1', done: 3 * 1048576, total: 8 * 1048576 }))
    expect(v.line).toBe('Скачиваем обновление — 3 из 8 МБ')
    expect(v.pct).toBe(38)
    expect(v.btns).toEqual([{ act: 'cancel', label: 'Отменить' }])
  })

  it('downloading без размера: без чисел, полоса без доли', () => {
    const v = updateView(info({ phase: 'downloading' }))
    expect(v.line).toBe('Скачиваем обновление…')
    expect(v.pct).toBeNull()
    expect(v.busy).toBe(true)
  })

  it('ready: install — «установит его и откроется снова», setup — «запустит установщик»', () => {
    const a = updateView(info({ phase: 'ready', mode: 'install', version: '0.25.1' }))
    expect(a.line).toBe('Обновление 0.25.1 готово. Программа закроется, установит его и откроется снова; альбом сохранится')
    expect(a.btns).toEqual([{ act: 'install', label: 'Перезапустить и обновить', primary: true }])
    const b = updateView(info({ phase: 'ready', mode: 'setup', version: '0.25.1' }))
    expect(b.line).toBe('Обновление 0.25.1 готово. Программа закроется и запустит установщик')
    expect(b.btns[0].label).toBe('Перезапустить и установить')
  })

  it('error: текст Go, «Повторить» и «Страница выпуска»', () => {
    const v = updateView(info({ phase: 'error', error: 'Нет связи с GitHub' }))
    expect(v.line).toBe('Нет связи с GitHub')
    expect(v.btns).toEqual([{ act: 'check', label: 'Повторить' }])
    expect(v.pageLink).toBe(true)
  })

  it('error: явно заданное «что повторять» важнее данных Go (версия осталась от прежнего предложения)', () => {
    const i = info({ phase: 'error', version: '0.25.1', error: 'нет связи' })
    expect(updateView(i).btns[0].act).toBe('download')
    expect(updateView(i, 'check').btns).toEqual([{ act: 'check', label: 'Повторить' }])
  })

  it('«Повторить»: ошибка скачивания (есть версия) — снова скачать, иначе проверить; на странице-режиме — проверить', () => {
    expect(retryAct(info({ phase: 'error', version: '0.25.1' }))).toBe('download')
    expect(retryAct(info({ phase: 'error', version: '' }))).toBe('check')
    expect(retryAct(info({ phase: 'error', version: '0.25.1', mode: 'page' }))).toBe('check')
  })

  it('setup: подсказка «Поставьте установщиком…» в любой фазе; в install и page её нет', () => {
    const hint = 'Поставьте установщиком — дальше программа будет обновляться сама'
    for (const phase of ['idle', 'latest', 'available', 'error'] as const)
      expect(updateView(info({ phase, mode: 'setup' })).hint).toBe(hint)
    expect(updateView(info({ phase: 'latest', mode: 'install' })).hint).toBe('')
    expect(updateView(info({ phase: 'latest', mode: 'page' })).hint).toBe('')
  })
})

describe('progressLine / progressPct', () => {
  it('меньше мегабайта — в КБ; доля зажата в 0…100', () => {
    expect(progressLine({ done: 512, total: 2048 })).toBe('Скачиваем обновление — 512 Б из 2 КБ')
    expect(progressPct({ done: 9, total: 8 })).toBe(100)
    expect(progressPct({ done: -1, total: 8 })).toBe(0)
    expect(progressPct({ done: 1, total: 0 })).toBeNull()
  })
})

describe('тосты', () => {
  it('offer: Windows — «Обновить», Mac и Linux — «Скачать»', () => {
    expect(offerToast(info({ version: '0.25.1' }))).toEqual({ text: 'Вышла версия 0.25.1', label: 'Обновить', act: 'download' })
    expect(offerToast(info({ version: '0.25.1', mode: 'setup' })).act).toBe('download')
    expect(offerToast(info({ version: '0.25.1', mode: 'page' }))).toEqual({ text: 'Вышла версия 0.25.1', label: 'Скачать', act: 'page' })
  })

  it('готово: «Перезапустить» (переносной — «Установить»)', () => {
    expect(readyToast(info({ version: '0.25.1' }))).toEqual({ text: 'Обновление 0.25.1 готово', label: 'Перезапустить' })
    expect(readyToast(info({ version: '0.25.1', mode: 'setup' })).label).toBe('Установить')
  })

  it('итог после перезапуска и начало скачивания', () => {
    expect(justText('0.25.1')).toBe('Программа обновлена до 0.25.1')
    expect(failedText('0.25.1')).toBe('Обновление до 0.25.1 не установилось')
    expect(startedText('0.25.1')).toBe('Скачиваем обновление 0.25.1…')
  })
})
