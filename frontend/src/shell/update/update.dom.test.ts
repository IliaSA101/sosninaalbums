// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const go = vi.hoisted(() => ({
  UpdateInfo: vi.fn(),
  CheckUpdate: vi.fn(),
  DownloadUpdate: vi.fn(),
  CancelUpdate: vi.fn(),
  InstallUpdate: vi.fn(),
  OpenUpdatePage: vi.fn(),
  SetUpdateCheck: vi.fn(),
}))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => go)

const { UpdateUi } = await import('./update.svelte')
const { NO_UPDATE } = await import('./updateapi')
const { store } = await import('../../base/core/store.svelte')

import type { UpdateInfo } from './updateapi'
const win = (p: Partial<UpdateInfo>): UpdateInfo => ({ ...NO_UPDATE, mode: 'install', ...p })
const tick = () => new Promise((r) => setTimeout(r, 0))
const toast = () => store.toast

let ui: InstanceType<typeof UpdateUi>
let whatsNew: ReturnType<typeof vi.fn<() => void>>

beforeEach(() => {
  for (const f of Object.values(go)) f.mockReset().mockResolvedValue(undefined)
  store.toast = null
  ui = new UpdateUi()
  whatsNew = vi.fn<() => void>()
  ui.host = { whatsNew }
})
afterEach(() => vi.restoreAllMocks())

describe('запуск: итог прошлого обновления — запросом UpdateInfo', () => {
  it('Just → «Программа обновлена до …» с «Что нового»: кнопка зовёт команду', async () => {
    go.UpdateInfo.mockResolvedValue(win({ phase: 'idle', just: '0.25.1' }))
    await ui.start()
    expect(toast()).toMatchObject({ text: 'Программа обновлена до 0.25.1', kind: 'info', action: { label: 'Что нового' } })
    toast()?.action?.run()
    expect(whatsNew).toHaveBeenCalledTimes(1)
  })

  it('Failed → красный тост «Обновление до … не установилось» с «Страница выпуска»', async () => {
    go.UpdateInfo.mockResolvedValue(win({ failed: '0.25.1' }))
    await ui.start()
    expect(toast()).toMatchObject({ text: 'Обновление до 0.25.1 не установилось', kind: 'error', action: { label: 'Страница выпуска' } })
    toast()?.action?.run()
    expect(go.OpenUpdatePage).toHaveBeenCalledTimes(1)
  })

  it('обычный запуск без итога — тоста нет; Go недоступен — тихо', async () => {
    go.UpdateInfo.mockResolvedValue(win({ phase: 'idle' }))
    await ui.start()
    expect(toast()).toBeNull()
    go.UpdateInfo.mockRejectedValue('нет Go')
    await ui.start()
    expect(toast()).toBeNull()
    expect(ui.info.phase).toBe('idle')
  })

  // Go при запуске отдаёт idle (скачанное с прошлого раза «готово» становится после «Обновить»: Fetch берёт файл без сети);
  // ready в UpdateInfo бывает при перезагрузке страницы в dev — тост тот же.
  it('состояние ready при запуске (перезагрузка страницы) → тост «готово»', async () => {
    go.UpdateInfo.mockResolvedValue(win({ phase: 'ready', version: '0.25.1' }))
    await ui.start()
    expect(toast()).toMatchObject({ text: 'Обновление 0.25.1 готово', action: { label: 'Перезапустить' } })
  })
})

describe('update:offer', () => {
  it('Windows: «Вышла версия …» с «Обновить»; кнопка скачивает в фоне и пишет тост о начале', async () => {
    let done!: (i: UpdateInfo) => void
    go.DownloadUpdate.mockReturnValue(new Promise((r) => (done = r)))
    ui.offered(win({ phase: 'available', version: '0.25.1' }))
    expect(toast()).toMatchObject({ text: 'Вышла версия 0.25.1', action: { label: 'Обновить' } })
    const offer = toast()
    offer?.action?.run()
    expect(go.DownloadUpdate).toHaveBeenCalledTimes(1)
    expect(toast()?.text).toBe('Скачиваем обновление 0.25.1…')
    expect(ui.info.phase).toBe('downloading')
    // Повторное «Обновить» из того же тоста не запускает второе скачивание.
    offer?.action?.run()
    expect(go.DownloadUpdate).toHaveBeenCalledTimes(1)
    done(win({ phase: 'ready', version: '0.25.1' }))
    await tick()
    expect(toast()).toMatchObject({ text: 'Обновление 0.25.1 готово', action: { label: 'Перезапустить' } })
  })

  it('Mac и Linux: «Скачать» открывает страницу выпуска', () => {
    ui.offered(win({ phase: 'available', mode: 'page', version: '0.25.1' }))
    expect(toast()?.action?.label).toBe('Скачать')
    toast()?.action?.run()
    expect(go.OpenUpdatePage).toHaveBeenCalledTimes(1)
    expect(go.DownloadUpdate).not.toHaveBeenCalled()
  })

  it('openPage: на стенде — тост «На стенде ссылки не открываются», Go не зовётся; не стенд — зовётся', async () => {
    store.stand = true
    try {
      await ui.openPage()
      expect(go.OpenUpdatePage).not.toHaveBeenCalled()
      expect(toast()).toMatchObject({ text: 'На стенде ссылки не открываются' })
    } finally {
      store.stand = false
    }
    await ui.openPage()
    expect(go.OpenUpdatePage).toHaveBeenCalledTimes(1)
  })

  it('предложение без версии или не в фазе available — молча', () => {
    ui.offered(win({ phase: 'available', version: '' }))
    ui.offered(win({ phase: 'latest', version: '0.25.1' }))
    expect(toast()).toBeNull()
  })
})

describe('тост «Вышла версия» при запуске (интерфейс подписался позже Go)', () => {
  it('начальное состояние available и atStart → тот же тост; событие offer того же сеанса его не дублирует', async () => {
    go.UpdateInfo.mockResolvedValue(win({ phase: 'available', version: '0.25.1', atStart: true }))
    await ui.start()
    expect(toast()).toMatchObject({ text: 'Вышла версия 0.25.1', kind: 'info', action: { label: 'Обновить' } })
    store.toast = null
    ui.offered(win({ phase: 'available', version: '0.25.1' }))
    expect(toast()).toBeNull()
    ui.offered(win({ phase: 'available', version: '0.25.2' }))
    expect(toast()?.text).toBe('Вышла версия 0.25.2')
  })

  it('сперва событие, потом опрос — тоже один тост', async () => {
    ui.offered(win({ phase: 'available', version: '0.25.1' }))
    store.toast = null
    go.UpdateInfo.mockResolvedValue(win({ phase: 'available', version: '0.25.1' }))
    await ui.start()
    expect(toast()).toBeNull()
  })

  it('atStart выключен, нет версии или другая фаза — тоста нет; Mac и Linux — «Скачать»', async () => {
    go.UpdateInfo.mockResolvedValue(win({ phase: 'available', version: '0.25.1', atStart: false }))
    await ui.start()
    expect(toast()).toBeNull()
    go.UpdateInfo.mockResolvedValue(win({ phase: 'latest', version: '0.25.1' }))
    await ui.start()
    expect(toast()).toBeNull()
    go.UpdateInfo.mockResolvedValue(win({ phase: 'available', mode: 'page', version: '0.25.1' }))
    await ui.start()
    expect(toast()?.action?.label).toBe('Скачать')
  })

  it('итог прошлого обновления (Just) важнее: его тост не заменяется', async () => {
    go.UpdateInfo.mockResolvedValue(win({ phase: 'available', version: '0.25.2', just: '0.25.1' }))
    await ui.start()
    expect(toast()?.text).toBe('Программа обновлена до 0.25.1')
  })
})

describe('ошибки из Go и «Повторить»', () => {
  it('update:state с ошибкой: первая буква заглавная', () => {
    ui.changed(win({ phase: 'error', error: 'обновление не прошло проверку — подпись не совпала' }))
    expect(ui.info.error).toBe('Обновление не прошло проверку — подпись не совпала')
    ui.changed(win({ phase: 'error', error: 'файл обновления на GitHub не найден — x' }))
    expect(ui.info.error).toBe('Файл обновления на GitHub не найден — x')
  })

  it('ошибка проверки при старой версии от прежнего предложения → «Повторить» проверяет', async () => {
    go.CheckUpdate.mockResolvedValue(win({ phase: 'error', version: '0.25.1', error: 'нет связи' }))
    ui.changed(win({ phase: 'available', version: '0.25.1' }))
    await ui.check()
    expect(ui.info.phase).toBe('error')
    expect(ui.retry).toBe('check')
  })

  it('ошибка скачивания → «Повторить» скачивает; ответ-ошибка и reject одинаково', async () => {
    ui.changed(win({ phase: 'available', version: '0.25.1' }))
    go.DownloadUpdate.mockResolvedValue(win({ phase: 'error', version: '0.25.1', error: 'обрыв' }))
    await ui.download()
    expect(ui.retry).toBe('download')
    go.DownloadUpdate.mockRejectedValue('сеть пропала')
    await ui.download()
    expect(ui.info.error).toBe('Сеть пропала')
    expect(ui.retry).toBe('download')
  })

  it('reject проверки → check; успешный ответ сбрасывает; page-режим всегда check; событие без прежней фазы — по данным Go', async () => {
    go.CheckUpdate.mockRejectedValue('нет связи')
    await ui.check()
    expect(ui.retry).toBe('check')
    go.CheckUpdate.mockResolvedValue(win({ phase: 'latest' }))
    await ui.check()
    expect(ui.retry).toBeUndefined()
    ui.changed(win({ phase: 'downloading', mode: 'page', version: '0.25.1' }))
    ui.changed(win({ phase: 'error', mode: 'page', version: '0.25.1', error: 'x' }))
    expect(ui.retry).toBe('check')
    ui.changed(win({ phase: 'idle' }))
    ui.changed(win({ phase: 'error', version: '0.25.1', error: 'x' }))
    expect(ui.retry).toBe('download')
  })
})

describe('готово и перезапуск', () => {
  it('«готово» приходит дважды (событие и ответ метода) — тост один раз; «Перезапустить» → InstallUpdate', async () => {
    ui.changed(win({ phase: 'downloading', version: '0.25.1' }))
    ui.changed(win({ phase: 'ready', version: '0.25.1' }))
    const first = toast()
    expect(first?.action?.label).toBe('Перезапустить')
    store.toast = null
    ui.changed(win({ phase: 'ready', version: '0.25.1' }))
    expect(toast()).toBeNull()
    first?.action?.run()
    await tick()
    expect(go.InstallUpdate).toHaveBeenCalledTimes(1)
  })

  it('переносная копия: тост «Установить»; страница-режим «готово» не показывает', () => {
    ui.changed(win({ phase: 'ready', mode: 'setup', version: '0.25.1' }))
    expect(toast()?.action?.label).toBe('Установить')
    store.toast = null
    ui.changed(win({ phase: 'ready', mode: 'page', version: '0.25.1' }))
    expect(toast()).toBeNull()
  })

  it('InstallUpdate не удался — красный тост, повтор возможен', async () => {
    go.InstallUpdate.mockRejectedValueOnce('установщик не запустился')
    ui.changed(win({ phase: 'ready', version: '0.25.1' }))
    await ui.install()
    expect(toast()).toMatchObject({ text: 'Установщик не запустился', kind: 'error' })
    await ui.install()
    expect(go.InstallUpdate).toHaveBeenCalledTimes(2)
  })

  it('install вне фазы ready ничего не зовёт', async () => {
    await ui.install()
    expect(go.InstallUpdate).not.toHaveBeenCalled()
  })
})

describe('ход, отмена и повторы', () => {
  it('progress вне скачивания (готово, ошибка, проверка) состояние не трогает', () => {
    for (const phase of ['ready', 'error', 'checking'] as const) {
      ui.changed(win({ phase }))
      ui.progressed({ done: 1, total: 2 })
      expect(ui.info.phase).toBe(phase)
    }
  })

  it('ход без нашего вызова (скачивание из тоста до открытия окна) переводит блок в downloading', () => {
    ui.changed(win({ phase: 'available', version: '0.25.1' }))
    ui.progressed({ done: 10, total: 100 })
    expect(ui.info).toMatchObject({ phase: 'downloading', done: 10, total: 100, version: '0.25.1' })
  })

  it('«Отменить» при скачивании без нашего вызова: после CancelUpdate состояние спрашивается заново', async () => {
    ui.changed(win({ phase: 'available', version: '0.25.1' }))
    ui.progressed({ done: 10, total: 100 })
    go.UpdateInfo.mockResolvedValue(win({ phase: 'available', version: '0.25.1' }))
    await ui.cancel()
    expect(go.CancelUpdate).toHaveBeenCalledTimes(1)
    expect(ui.info.phase).toBe('available')
    expect(ui.cancelling).toBe(false)
  })

  it('«Отменить» вне скачивания молчит; повторный щелчок не шлёт второй CancelUpdate', async () => {
    await ui.cancel()
    expect(go.CancelUpdate).not.toHaveBeenCalled()
    ui.changed(win({ phase: 'downloading' }))
    go.CancelUpdate.mockReturnValue(new Promise(() => {}))
    void ui.cancel()
    await ui.cancel()
    expect(go.CancelUpdate).toHaveBeenCalledTimes(1)
  })

  it('повторная проверка во время проверки и скачивания Go не зовёт', async () => {
    ui.changed(win({ phase: 'checking' }))
    await ui.check()
    ui.changed(win({ phase: 'downloading' }))
    await ui.check()
    expect(go.CheckUpdate).not.toHaveBeenCalled()
  })

  it('флажок: поздний ответ на прежний щелчок не затирает новый', async () => {
    let first!: (i: UpdateInfo) => void
    go.SetUpdateCheck.mockReturnValueOnce(new Promise((r) => (first = r))).mockResolvedValueOnce(win({ atStart: true }))
    void ui.setAtStart(false)
    await ui.setAtStart(true)
    first(win({ atStart: false }))
    await tick()
    expect(ui.info.atStart).toBe(true)
  })
})
