import { describe, expect, it } from 'vitest'
import { NO_UPDATE, type UpdateInfo } from './updateapi'
import {
  blockHint,
  failedText,
  justText,
  lastToast,
  offerToast,
  osName,
  progressLine,
  progressPct,
  readyToast,
  retryAct,
  startedText,
  updateView,
} from './updatetext'

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

  it('error: «Повторить» берётся из errorOf Go, а не из версии прежнего предложения', () => {
    const stale = { phase: 'error', version: '0.25.1', error: 'нет связи' } as const
    expect(updateView(info({ ...stale, errorOf: 'check' })).btns[0].act).toBe('check')
    expect(updateView(info({ ...stale, errorOf: 'download' })).btns[0].act).toBe('download')
  })

  it('«Повторить»: скачивание и установка (Go забыл файл, Fetch сверит) — снова скачать; проверка, пусто и режим «страница» — проверить', () => {
    expect(retryAct(info({ phase: 'error', errorOf: 'download' }))).toBe('download')
    expect(retryAct(info({ phase: 'error', errorOf: 'install' }))).toBe('download')
    expect(retryAct(info({ phase: 'error', errorOf: 'check', version: '0.25.1' }))).toBe('check')
    expect(retryAct(info({ phase: 'error', errorOf: '', version: '0.25.1' }))).toBe('check')
    expect(retryAct(info({ phase: 'error', errorOf: 'download', mode: 'page' }))).toBe('check')
  })

  it('setup: подсказка «Поставьте установщиком…» в любой фазе; в install и page её нет', () => {
    const hint = 'Поставьте установщиком — дальше программа будет обновляться сама'
    for (const phase of ['idle', 'latest', 'available', 'error'] as const)
      expect(updateView(info({ phase, mode: 'setup' })).hint).toBe(hint)
    expect(updateView(info({ phase: 'latest', mode: 'install' })).hint).toBe('')
    expect(updateView(info({ phase: 'latest', mode: 'page' })).hint).toBe('')
  })
})

const mac12 = { os: 'darwin', osNeed: '13.0', blocked: '0.43.0' } as const

describe('osName: система словами (design §6)', () => {
  it('darwin 13.0 → «macOS 13», 13.1 остаётся; windows, прочее, пусто', () => {
    expect(osName('darwin', '13.0')).toBe('macOS 13')
    expect(osName('darwin', '13.1')).toBe('macOS 13.1')
    expect(osName('darwin', '13')).toBe('macOS 13')
    expect(osName('windows', '10.0.19045')).toBe('Windows 10.0.19045')
    expect(osName('linux', '5.4')).toBe('система 5.4')
    expect(osName('darwin', '')).toBe('более новая система')
  })

  it('все хвостовые нули отбрасываются, значащие остаются', () => {
    expect(osName('darwin', '13.0.0')).toBe('macOS 13')
    expect(osName('darwin', '12.0')).toBe('macOS 12')
    expect(osName('darwin', '10.13.0')).toBe('macOS 10.13')
    expect(osName('darwin', '13.1')).toBe('macOS 13.1')
  })
})

describe('блокировка по системе (171): строка, подсказка, кнопки', () => {
  it('latest + blocked: «Эта версия — последняя для вашей системы», подсказка, одна кнопка «Проверить», ссылки нет', () => {
    const v = updateView(info({ phase: 'latest', mode: 'page', ...mac12 }))
    expect(v.line).toBe('Эта версия — последняя для вашей системы')
    expect(v.hint).toBe('Для версии 0.43.0 и следующих нужна macOS 13 или новее')
    expect(v.btns).toEqual([{ act: 'check', label: 'Проверить обновления', disabled: false }])
    expect(v.pageLink).toBe(false)
  })

  it('latest без blocked — прежняя строка и пустая подсказка', () => {
    const v = updateView(info({ phase: 'latest', mode: 'page', os: 'darwin' }))
    expect(v.line).toBe('Установлена последняя версия')
    expect(v.hint).toBe('')
  })

  it('available + blocked (ступень lastOn): обычное предложение и подсказка «Это последняя версия…»', () => {
    const v = updateView(info({ phase: 'available', mode: 'page', version: '0.42.0', ...mac12 }))
    expect(v.line).toBe('Вышла версия 0.42.0')
    expect(v.hint).toBe('Это последняя версия для вашей системы: для 0.43.0 и следующих нужна macOS 13 или новее')
    expect(v.btns[1]).toEqual({ act: 'page', label: 'Скачать', primary: true })
  })

  it('подсказка блокировки заменяет «Поставьте установщиком…» на переносной Windows', () => {
    const v = updateView(info({ phase: 'latest', mode: 'setup', os: 'windows', osNeed: '11', blocked: '0.50.0' }))
    expect(v.hint).toBe('Для версии 0.50.0 и следующих нужна Windows 11 или новее')
    expect(updateView(info({ phase: 'latest', mode: 'setup', os: 'windows' })).hint).toContain('Поставьте установщиком')
  })

  it('blocked в фазах не про предложение (checking, error) подсказку не меняет', () => {
    expect(updateView(info({ phase: 'checking', mode: 'page', ...mac12 })).hint).toBe('')
    expect(updateView(info({ phase: 'error', mode: 'page', error: 'x', ...mac12 })).hint).toBe('')
  })

  it('минимум неизвестен — «более новая система», без «или новее»', () => {
    expect(blockHint({ phase: 'latest', os: 'darwin', osNeed: '', blocked: '0.43.0' })).toBe(
      'Для версии 0.43.0 и следующих нужна более новая система',
    )
  })

  it('тост update:last: слово ОС и без кнопки', () => {
    expect(lastToast({ os: 'darwin', osNeed: '13.0', last: '0.43.0' })).toBe(
      'Вышла версия 0.43.0 для macOS 13 и новее. Ваша версия программы — последняя для этой системы',
    )
    expect(lastToast({ os: 'windows', osNeed: '11', last: '0.50.0' })).toContain('для Windows 11 и новее')
    expect(lastToast({ os: 'darwin', osNeed: '', last: '0.43.0' })).toContain('для более новой системы.')
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
