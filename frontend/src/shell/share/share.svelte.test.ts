// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

// Состояние окна «Отправить по ссылке» (146) без отрисовки: гонки ответов Go, повторные вызовы, признак своей выгрузки.
vi.mock('../../../wailsjs/go/app/App', () => ({ SaveProject: vi.fn(async () => ({})) }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))

const { share } = await import('./share.svelte')
const { StubShare, stubView } = await import('./sharestub')

import type { ShareProgress } from './shareapi'

type View = ReturnType<typeof stubView>
let stub: InstanceType<typeof StubShare>
beforeEach(() => {
  stub = new StubShare('plugged')
  share.bridge = stub
  share.close()
  share.view = null
  share.progress = null
  share.working = false
  share.stopping = false
  share.error = ''
  share.changeError = null
  share.waiting = false
})

describe('ответы Go и запросы', () => {
  it('запоздалый ShareState прежнего запроса отбрасывается номером', async () => {
    let release!: (v: View) => void
    const fast = stub.ShareState.bind(stub)
    stub.ShareState = () => new Promise((r) => (release = r)) // первый запрос зависает
    const old = share.refresh()
    stub.ShareState = fast
    await share.refresh() // второй, новее: «подключено»
    expect(share.view?.service.host).not.toBe('')
    release(stubView('unplugged'))
    await old
    expect(share.view?.service.host).not.toBe('') // старый ответ «не подключено» не затёр новый
  })

  it('ответ метода (Отключить) новее ShareState, ушедшего раньше него', async () => {
    let release!: (v: View) => void
    const fast = stub.ShareState.bind(stub)
    stub.ShareState = () => new Promise((r) => (release = r))
    const old = share.refresh()
    stub.ShareState = fast
    await share.forget()
    expect(share.view?.service.host).toBe('')
    release(stubView('plugged')) // запоздалый «подключено»
    await old
    expect(share.view?.service.host).toBe('')
  })

  it('show() сбрасывает прежний вид до ответа Go', async () => {
    share.view = stubView('published')
    let release!: (v: View) => void
    stub.ShareState = () => new Promise((r) => (release = r))
    const p = share.show()
    expect(share.open).toBe(true)
    expect(share.view).toBeNull() // старая ссылка не мелькает
    release(stubView('plugged'))
    await p
    expect(share.view?.link).toBeNull()
  })

  it('двойной start при работающем вызове — один Share', async () => {
    await share.refresh()
    const a = share.start({ title: 'А', password: '', days: 90 })
    const b = share.start({ title: 'А', password: '', days: 90 })
    await Promise.all([a, b])
    expect(stub.calls.filter((c) => c.startsWith('Share '))).toHaveLength(1)
  })

  it('двойной change при работающем вызове — один ShareChange, второй возвращает false', async () => {
    stub.view = stubView('published')
    await share.refresh()
    const c = { days: 0, setPassword: true, password: 'x' }
    const a = share.change(c)
    const b = share.change(c)
    expect(await b).toBe(false)
    expect(await a).toBe(true)
    expect(stub.calls.filter((x) => x.startsWith('ShareChange'))).toHaveLength(1)
  })

  it('отказ ShareChange: false, кнопки снова доступны', async () => {
    stub.view = stubView('published')
    await share.refresh()
    stub.fail.ShareChange = 'пароль слишком короткий'
    expect(await share.change({ days: 0, setPassword: true, password: 'x' })).toBe(false)
    expect(share.working).toBe(false)
  })
})

describe('своя выгрузка (пункт меню)', () => {
  it('sharing: идёт своя — да; чужая, нет выгрузки — нет', () => {
    expect(share.sharing).toBe(false)
    share.view = stubView('running')
    expect(share.sharing).toBe(true)
    share.view = stubView('busyOther')
    expect(share.sharing).toBe(false)
    share.view = stubView('plugged')
    expect(share.sharing).toBe(false)
  })

  it('share:done при закрытом окне гасит устаревшее busy', () => {
    share.view = stubView('running')
    share.finished({ err: '', stopped: false })
    expect(share.sharing).toBe(false)
  })
})

describe('другой альбом при открытой странице (QA: кнопки зависали серыми)', () => {
  it('вызов, не вернувшийся для прежнего альбома, кнопки не держит: working сброшен, новый start идёт', async () => {
    share.watchAlbum('1|a.salbum')
    await share.show()
    stub.Share = () => new Promise(() => {}) // Go не отвечает (вопрос в окне, блокировка)
    void share.start({ title: 'А', password: '', days: 90 })
    await vi.waitFor(() => expect(share.working).toBe(true))
    share.watchAlbum('2|b.salbum') // OpenRecent
    expect(share.working).toBe(false)
    stub.Share = StubShare.prototype.Share.bind(stub)
    await share.start({ title: 'Б', password: '', days: 90 })
    expect(stub.lastOptions?.title).toBe('Б')
  })

  it('запоздалый ответ прежнего альбома не трогает вид и working нового', async () => {
    share.watchAlbum('1|a')
    await share.show()
    let release!: (v: View) => void
    stub.ShareChange = () => new Promise((r) => (release = r))
    const p = share.change({ days: 0, setPassword: true, password: 'x' })
    share.watchAlbum('2|b')
    await vi.waitFor(() => expect(share.view).not.toBeNull()) // вид нового альбома перечитан
    const fresh = share.view
    release(stubView('published'))
    expect(await p).toBe(false)
    expect(share.view).toBe(fresh)
    expect(share.working).toBe(false)
  })

  it('тот же ключ (правка того же альбома) ничего не сбрасывает', async () => {
    share.watchAlbum('3|c')
    share.working = true
    share.watchAlbum('3|c')
    expect(share.working).toBe(true)
  })
})

describe('отказ пароля или срока', () => {
  it('change: строка под своей строкой (password / term), тоста нет; следующая попытка и close её снимают', async () => {
    const notify = vi.spyOn((await import('../../base/core/store.svelte')).store, 'notify')
    stub.view = stubView('published')
    await share.refresh()
    stub.fail.ShareChange = 'пароль короче 4 знаков'
    expect(await share.change({ days: 0, setPassword: true, password: 'x' })).toBe(false)
    expect(share.changeError).toEqual({ part: 'password', text: 'Пароль короче 4 знаков' })
    stub.fail.ShareChange = 'срок больше предела'
    await share.change({ days: 365, setPassword: false, password: '' })
    expect(share.changeError).toEqual({ part: 'term', text: 'Срок больше предела' })
    expect(notify).not.toHaveBeenCalled()
    expect(await share.change({ days: 30, setPassword: false, password: '' })).toBe(true)
    expect(share.changeError).toBeNull()
  })
})

describe('конец выгрузки', () => {
  it('share:done при открытом окне: ход и «Останавливаем…» держатся, пока не пришёл новый вид', async () => {
    share.view = stubView('running')
    share.progress = { phase: 'publish', done: 0, total: 0, bytes: 0, totalBytes: 0 }
    share.stopping = true
    share.open = true
    let release!: (v: View) => void
    stub.ShareState = () => new Promise((r) => (release = r))
    const p = share.finished({ err: '', stopped: true })
    expect(share.progress).not.toBeNull()
    expect(share.stopping).toBe(true)
    release(stubView('plugged'))
    await p
    expect(share.progress).toBeNull()
    expect(share.stopping).toBe(false)
    expect(share.view?.busy).toBe(false)
  })
})

describe('share:progress и ShareState (ревью: опрос тяжёлого запроса)', () => {
  const prog: ShareProgress = { phase: 'publish', done: 1, total: 10, bytes: 0, totalBytes: 0 }
  const states = () => stub.calls.filter((c) => c === 'ShareState').length
  const burst = (n: number) => {
    for (let i = 0; i < n; i++) share.progressed(prog)
  }

  it('чужая выгрузка (busyOther, в том числе с busy=false): 20 событий — ни одного ShareState', () => {
    share.open = true
    share.view = stubView('busyOther')
    burst(20)
    share.view = { ...stubView('busyOther'), busy: false }
    burst(20)
    expect(states()).toBe(0)
  })

  it('окно закрыто: 20 событий — ни одного ShareState', () => {
    share.view = stubView('plugged')
    burst(20)
    expect(states()).toBe(0)
  })

  it('своя выгрузка (busy): событий достаточно только для хода, ShareState не зовётся', () => {
    share.open = true
    share.view = stubView('running')
    burst(5)
    expect(states()).toBe(0)
    expect(share.progress).toEqual(prog)
  })

  it('окно открыто после старта (вид без busy): перечитывает и получает ход', async () => {
    stub.view = stubView('running')
    share.open = true
    share.view = stubView('plugged')
    share.progressed(prog)
    await vi.waitFor(() => expect(share.view?.busy).toBe(true))
    expect(states()).toBe(1)
  })

  it('пачка событий при медленном ShareState: ровно 2 вызова, итог свежий', async () => {
    share.open = true
    share.view = stubView('plugged')
    const resolvers: ((v: View) => void)[] = []
    stub.ShareState = () => {
      stub.calls.push('ShareState')
      return new Promise((r) => resolvers.push(r))
    }
    burst(30)
    resolvers[0](stubView('plugged'))
    await vi.waitFor(() => expect(resolvers).toHaveLength(2))
    resolvers[1](stubView('running'))
    await vi.waitFor(() => expect(share.view?.busy).toBe(true))
    expect(states()).toBe(2)
  })
})

describe('share:done и отказ ShareState', () => {
  it('ShareState упал: busy снят с вида, ход и «Остановить» не держатся, тост ошибки показан', async () => {
    share.view = stubView('running')
    share.progress = { phase: 'publish', done: 0, total: 0, bytes: 0, totalBytes: 0 }
    share.stopping = true
    share.open = true
    stub.fail.ShareState = 'сервис не отвечает'
    await share.finished({ err: '', stopped: false })
    expect(share.view?.busy).toBe(false)
    expect(share.sharing).toBe(false)
    expect(share.progress).toBeNull()
    expect(share.stopping).toBe(false)
  })
})

describe('ожидание ответа на вопрос Go (T5-11)', () => {
  const opts = { title: 'А', password: '', days: 90 }
  afterEach(() => vi.useRealTimers())

  it('подсказка включается, если Share не отвечает дольше паузы, и снимается отказом в вопросе без ошибки', async () => {
    vi.useFakeTimers()
    await share.refresh()
    let answer!: (v: boolean) => void
    stub.Share = () => new Promise<boolean>((r) => (answer = r))
    const p = share.start(opts)
    await vi.advanceTimersByTimeAsync(100)
    expect(share.waiting).toBe(false) // быстрый ответ — без мигания
    await vi.advanceTimersByTimeAsync(400)
    expect(share.waiting).toBe(true)
    answer(false) // «Нет» на вопрос
    await p
    expect(share.waiting).toBe(false)
    expect(share.working).toBe(false)
    expect(share.error).toBe('')
  })

  it('старт выгрузки снимает подсказку', async () => {
    vi.useFakeTimers()
    await share.refresh()
    let answer!: (v: boolean) => void
    stub.Share = () => new Promise<boolean>((r) => (answer = r))
    const p = share.start(opts)
    await vi.advanceTimersByTimeAsync(500)
    expect(share.waiting).toBe(true)
    answer(true)
    await p
    expect(share.waiting).toBe(false)
  })

  it('ответ раньше паузы подсказку не включает, отказ Go (ошибка) тоже снимает', async () => {
    vi.useFakeTimers()
    await share.refresh()
    await share.start(opts)
    await vi.advanceTimersByTimeAsync(1000)
    expect(share.waiting).toBe(false)
    stub.fail.Share = 'нельзя'
    await share.start(opts)
    expect(share.waiting).toBe(false)
  })

  it('смена альбома снимает подсказку', async () => {
    vi.useFakeTimers()
    share.watchAlbum('1|a')
    await share.refresh()
    stub.Share = () => new Promise<boolean>(() => {})
    void share.start(opts)
    await vi.advanceTimersByTimeAsync(500)
    expect(share.waiting).toBe(true)
    share.watchAlbum('2|b')
    expect(share.waiting).toBe(false)
  })
})
