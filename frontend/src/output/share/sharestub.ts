// Двойник службы «Отправить по ссылке» для тестов окна (146): все состояния design §3.4/§4 в памяти, без сети.
// Тесты берут `new StubShare(сценарий)` и подставляют в `share.bridge`; в приложении работают настоящие биндинги (shareapi.ts, T5-9).
import type { ShareBridge, ShareChangeArgs, ShareLinkInfo, ShareOptions, ShareView } from './shareapi'

export type Scenario =
  | 'unplugged' // не подключено
  | 'plugged' // подключено, альбом не выгружался
  | 'running' // идёт выгрузка
  | 'published' // выгружено, всё актуально
  | 'changed' // выгружено, альбом правили
  | 'incomplete' // обрыв посреди выгрузки
  | 'expired' // срок истёк
  | 'gone' // сервис ответил 404
  | 'other' // ссылка выгружена на другой сервис
  | 'unsaved' // альбом не сохранён
  | 'busyOther' // идёт выгрузка другого альбома
  | 'moved' // запись из другой папки этого компьютера (167): вопрос «обновить или как новый»
  | 'movedElsewhere' // найдена на сервисе, привязки к пути нет (другой компьютер)
  | 'find' // в файле есть привязка, записи нет: окно ищет альбом на сервисе
  | 'lost' // искали — не нашли
  | 'fail' // поиск не удался
  | 'expiredForeign' // срок прошёл, запись не этого файла
  | 'otherForeign' // ссылка другого сервиса, запись не этого файла

const HOST = 'albums.sosnina-films.ru'
const URL_ = 'https://' + HOST

function link(state: ShareLinkInfo['state'], over: Partial<ShareLinkInfo> = {}): ShareLinkInfo {
  return {
    state,
    url: URL_ + '/a/k3Jx7Qw2/',
    title: 'Свадьба И. и А.',
    password: 'лето-4827',
    expiresAt: '2026-11-29T00:00:00Z',
    until: 'до 29 ноября 2026',
    host: HOST,
    published: 'Выгружено 29 октября 2026 как «Свадьба И. и А.»',
    own: true,
    passwordUnknown: false,
    from: '',
    here: '',
    ...over,
  }
}

// Запись другого файла (167): не своя, без названия и пароля; файл здесь — «Петров», выгружал «Иванов».
const foreign = (state: ShareLinkInfo['state'], over: Partial<ShareLinkInfo> = {}): ShareLinkInfo =>
  link(state, { own: false, title: '', password: '', ...over })
const MOVED_HERE = 'Этот файл: папка E:\\Работа, «Петров.salbum»'
const MOVED_PUBLISHED = 'Выгружено 29 октября 2026 как «Иванов»'

// Чем сценарий отличается от «подключено, 62 страницы»
const PATCH: Record<Scenario, (v: ShareView) => void> = {
  unplugged: (v) => (v.service = { host: '', url: '', token: false, store: '', days: 0 }),
  plugged: () => {},
  running: (v) => (v.busy = true),
  published: (v) => (v.link = link('ok')),
  changed: (v) => (v.link = link('changed')),
  incomplete: (v) => (v.link = link('incomplete', { url: '', until: '' })),
  expired: (v) => (v.link = link('expired')),
  gone: (v) => (v.link = link('gone')),
  other: (v) => (v.link = link('other', { host: 'albums.example.org' })),
  unsaved: (v) => {
    v.album.block = 'Сначала сохраните альбом — ссылка хранится у файла альбома'
    v.album.save = true
  },
  busyOther: (v) => {
    v.busy = true
    v.busyOther = true
    v.album.block = 'Идёт выгрузка другого альбома — дождитесь её конца'
  },
  moved: (v) => {
    v.link = foreign('moved', { from: 'Из папки D:\\Свадьбы, файл «Иванов.salbum»', here: MOVED_HERE, published: MOVED_PUBLISHED })
    v.album.title = 'Петров'
  },
  movedElsewhere: (v) => {
    v.link = foreign('moved', { from: 'На другом компьютере или из другого файла', here: MOVED_HERE, published: 'Выгружено как «Иванов»' })
    v.album.title = 'Петров'
  },
  find: (v) => (v.album.find = 'need'),
  lost: (v) => (v.album.find = 'lost'),
  fail: (v) => {
    v.album.find = 'fail'
    v.album.findErr = 'Сервис не ответил — проверьте связь'
  },
  expiredForeign: (v) => {
    v.link = foreign('expired')
    v.album.title = 'Петров'
  },
  otherForeign: (v) => {
    v.link = foreign('other', { host: 'albums.example.org' })
    v.album.title = 'Петров'
  },
}

// Вид для сценария; поля, которых сценарий не трогает, — «подключено, 62 страницы».
export function stubView(kind: Scenario): ShareView {
  const v: ShareView = {
    service: { host: HOST, url: URL_, token: true, store: 'system', days: 365 },
    album: { block: '', save: false, title: 'Свадьба И. и А.', pages: 62, bytes: 24_000_000, find: '', findErr: '' },
    link: null,
    busy: false,
    busyOther: false,
  }
  PATCH[kind](v)
  return v
}

// Заглушка: ответы по сценарию; fail — текст ошибки следующего вызова метода (как отказ Go), calls — журнал вызовов.
export class StubShare implements ShareBridge {
  view: ShareView
  calls: string[] = []
  fail: Partial<Record<keyof ShareBridge, string>> = {}
  lastOptions: ShareOptions | null = null
  lastChange: ShareChangeArgs | null = null
  afterFind: Scenario = 'movedElsewhere' // что вернёт ShareFind: moved, lost или fail
  text = 'Альбом «Свадьба И. и А.»: https://albums.sosnina-films.ru/a/k3Jx7Qw2/\nПароль: лето-4827\nСсылка работает до 29 ноября 2026.'

  constructor(kind: Scenario = 'unplugged') {
    this.view = stubView(kind)
  }

  private async call<K extends keyof ShareBridge>(name: K, arg?: unknown): Promise<void> {
    this.calls.push(arg === undefined ? name : `${name} ${JSON.stringify(arg)}`)
    const f = this.fail[name]
    if (f) {
      delete this.fail[name]
      throw new Error(f)
    }
  }
  private snap(): ShareView {
    return structuredClone(this.view)
  }

  async ShareState() {
    await this.call('ShareState')
    return this.snap()
  }
  async ShareFind() {
    await this.call('ShareFind')
    const service = this.view.service
    this.view = stubView(this.afterFind)
    this.view.service = service
    return this.snap()
  }
  async ShareConnect(url: string, token: string) {
    await this.call('ShareConnect', [url, token ? '***' : ''])
    if (!token) throw new Error('введите токен')
    if (token === 'bad') throw new Error('токен не подошёл')
    this.view.service = { host: new URL(url).host, url, token: true, store: 'system', days: 365 }
    return this.snap()
  }
  async ShareForget() {
    await this.call('ShareForget')
    this.view.service = { ...this.view.service, host: '', token: false, store: '' }
    return this.snap()
  }
  async Share(o: ShareOptions) {
    await this.call('Share', o)
    this.lastOptions = o
    this.view.busy = true
    return true
  }
  async CancelShare() {
    await this.call('CancelShare')
  }
  async ShareChange(c: ShareChangeArgs) {
    await this.call('ShareChange', c)
    this.lastChange = c
    if (this.view.link) {
      if (c.setPassword) this.view.link.password = c.password
      if (c.days > 0) this.view.link.until = 'до 29 декабря 2026'
    }
    return this.snap()
  }
  async ShareRevoke() {
    await this.call('ShareRevoke')
    this.view.link = null
    return this.snap()
  }
  async SharePassword() {
    await this.call('SharePassword')
    return 'лето-4827'
  }
  async ShareText() {
    await this.call('ShareText')
    return this.text
  }
  async OpenShare() {
    await this.call('OpenShare')
  }
}

// Заглушка по умолчанию (shareapi.ts): «не подключено».
export const stubShare = new StubShare('unplugged')
