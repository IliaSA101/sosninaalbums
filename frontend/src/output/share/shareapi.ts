// Вызовы Go «Отправить по ссылке» (146, design §4): методы — из биндингов Wails, типы — из models.
// Здесь только узкие алиасы (state — объединение строк, link — null вместо «нет поля») и переходник ShareGo.
import * as Go from '../../../wailsjs/go/app/App'
import type { app } from '../../../wailsjs/go/models'

// state: none | incomplete | ok | changed | expired | gone | other | moved (design §3.4; moved — 167: запись не привязана к этому файлу)
export type LinkState = 'none' | 'incomplete' | 'ok' | 'changed' | 'expired' | 'gone' | 'other' | 'moved'

export type ShareService = app.ShareService
export type ShareAlbum = app.ShareAlbum
export type ShareLinkInfo = Omit<app.ShareLink, 'state'> & { state: LinkState }
export type ShareView = Omit<app.ShareView, 'link' | 'convertValues'> & {
  link: ShareLinkInfo | null // null — альбом не выгружался
}
export interface ShareOptions {
  title?: string // у «Продолжить» и «Обновить» не шлётся: служба берёт название и пароль из записи (0.32-146-T5-6)
  password?: string
  days: number // 30 | 90 | 365; 0 — продолжить или обновить без смены срока (новая ссылка с 0 — отказ)
  choice?: '' | 'update' | 'new' // ответ на экран moved (167-16); на другом экране Go отказывает
}
export type ShareChangeArgs = app.ShareChange // days: 0 — срок не менять; setPassword: true — заменить пароль ('' — снять)

// События: share:progress {phase, done, total, bytes, totalBytes}, share:done {err, stopped} — без url и id (разбор С3).
// В models их нет (шлются через EventsEmit), поэтому форма записана здесь по docs/architecture/events.md.
export interface ShareProgress {
  phase: 'pages' | 'upload' | 'publish'
  done: number
  total: number
  bytes: number
  totalBytes: number
}
export interface ShareDone {
  err: string
  stopped: boolean
}

export interface ShareBridge {
  ShareState(): Promise<ShareView>
  ShareFind(): Promise<ShareView> // поиск прежнего альбома на сервисе по привязкам файла (167)
  ShareConnect(url: string, token: string): Promise<ShareView>
  ShareForget(): Promise<ShareView>
  Share(o: ShareOptions): Promise<boolean>
  CancelShare(): Promise<void>
  ShareChange(c: ShareChangeArgs): Promise<ShareView>
  ShareRevoke(): Promise<ShareView>
  SharePassword(): Promise<string>
  ShareText(): Promise<string>
  OpenShare(): Promise<void>
}

// Ответ Go: link приходит null (или без поля), state — строка; окно работает с узким видом.
export function toView(v: app.ShareView): ShareView {
  return { ...v, link: v.link ? { ...v.link, state: v.link.state as LinkState } : null }
}

export const ShareGo: ShareBridge = {
  ShareState: async () => toView(await Go.ShareState()),
  ShareFind: async () => toView(await Go.ShareFind()),
  ShareConnect: async (url, token) => toView(await Go.ShareConnect(url, token)),
  ShareForget: async () => toView(await Go.ShareForget()),
  // Поля models обязательны, а Go пустые title и password читает как «взять из записи» (T5-6) — шлём '', не undefined.
  Share: (o) => Go.Share({ title: o.title ?? '', password: o.password ?? '', days: o.days, choice: o.choice ?? '' }),
  CancelShare: () => Go.CancelShare(),
  ShareChange: async (c) => toView(await Go.ShareChange(c)),
  ShareRevoke: async () => toView(await Go.ShareRevoke()),
  SharePassword: () => Go.SharePassword(),
  ShareText: () => Go.ShareText(),
  OpenShare: () => Go.OpenShare(),
}
