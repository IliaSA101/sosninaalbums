import { beforeEach, describe, expect, it, vi } from 'vitest'

// Переходник ShareGo над биндингами Wails (146): что уходит в Go и как ответ приводится к виду окна.
const go = vi.hoisted(() => ({
  Share: vi.fn(async (_o: unknown) => true),
  ShareState: vi.fn(),
  ShareConnect: vi.fn(),
  ShareForget: vi.fn(),
  ShareChange: vi.fn(),
  ShareRevoke: vi.fn(),
  CancelShare: vi.fn(async () => {}),
  SharePassword: vi.fn(async () => 'лето-4827'),
  ShareText: vi.fn(async () => 'текст'),
  OpenShare: vi.fn(async () => {}),
}))
vi.mock('../../../wailsjs/go/app/App', () => go)

const { ShareGo, toView } = await import('./shareapi')

const raw = (link: unknown) => ({
  service: { host: 'h', url: 'https://h', token: true, store: 'system', days: 365 },
  album: { block: '', save: false, title: 't', pages: 1, bytes: 2 },
  link,
  busy: false,
  busyOther: false,
})

beforeEach(() => vi.clearAllMocks())

describe('Share: поля models обязательны', () => {
  it('«Продолжить» и «Обновить» (только days) шлют пустые title и password, не undefined', async () => {
    await ShareGo.Share({ days: 0 })
    expect(go.Share).toHaveBeenCalledWith({ title: '', password: '', days: 0 })
    const sent = go.Share.mock.calls[0][0] as Record<string, unknown>
    expect(Object.values(sent).includes(undefined)).toBe(false)
  })
  it('новая ссылка передаёт название, пароль и срок как есть', async () => {
    await ShareGo.Share({ title: 'Свадьба', password: 'лето-1', days: 90 })
    expect(go.Share).toHaveBeenCalledWith({ title: 'Свадьба', password: 'лето-1', days: 90 })
  })
  it('false («Нет» на вопрос) доходит без изменений', async () => {
    go.Share.mockResolvedValueOnce(false)
    expect(await ShareGo.Share({ days: 30 })).toBe(false)
  })
})

describe('вид из ответа Go', () => {
  it('link: null и отсутствующий link дают null', async () => {
    go.ShareState.mockResolvedValueOnce(raw(null))
    expect((await ShareGo.ShareState()).link).toBeNull()
    expect(toView({ ...raw(null), link: undefined } as never).link).toBeNull()
  })
  it('ссылка копируется, state остаётся строкой Go', async () => {
    const l = { state: 'ok', url: 'u', title: 't', password: '', expiresAt: '', until: '', host: 'h', published: 'p' }
    go.ShareConnect.mockResolvedValueOnce(raw(l))
    const v = await ShareGo.ShareConnect('https://h', 'tok')
    expect(go.ShareConnect).toHaveBeenCalledWith('https://h', 'tok')
    expect(v.link).toEqual(l)
  })
  it.each(['ShareForget', 'ShareRevoke'] as const)('%s приводит ответ к виду окна', async (name) => {
    go[name].mockResolvedValueOnce(raw(null))
    expect((await ShareGo[name]()).link).toBeNull()
  })
  it('ShareChange передаёт правку как есть и приводит ответ', async () => {
    go.ShareChange.mockResolvedValueOnce(raw(null))
    const c = { days: 0, setPassword: true, password: '' }
    await ShareGo.ShareChange(c)
    expect(go.ShareChange).toHaveBeenCalledWith(c)
  })
  it('ошибка Go (отказ) пробрасывается окну', async () => {
    go.ShareState.mockRejectedValueOnce(new Error('сеть недоступна'))
    await expect(ShareGo.ShareState()).rejects.toThrow('сеть недоступна')
  })
})

describe('прочие вызовы', () => {
  it('пароль, текст, открыть, остановить', async () => {
    expect(await ShareGo.SharePassword()).toBe('лето-4827')
    expect(await ShareGo.ShareText()).toBe('текст')
    await ShareGo.OpenShare()
    await ShareGo.CancelShare()
    expect(go.OpenShare).toHaveBeenCalledTimes(1)
    expect(go.CancelShare).toHaveBeenCalledTimes(1)
  })
})
