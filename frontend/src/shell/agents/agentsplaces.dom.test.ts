// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'
import type { AgentPlace } from './agents'

// Список мест в блоке «Разрешения» (153, design §3): «Список (N) ▸» зовёт AgentPlacesLive только по раскрытию, строки, ✕, «Привязать к диску»,
// состояния диска и папки, подсказки узких мест, заметка о подъёме мест 0.34 и тост-указатель. Go подменён.
const ev = vi.hoisted(() => ({ on: new Map<string, (d: unknown) => void>() }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({
  EventsOn: (name: string, cb: (d: unknown) => void) => {
    ev.on.set(name, cb)
    return () => ev.on.delete(name)
  },
}))
const api = vi.hoisted(() => ({
  AgentsInfo: vi.fn(),
  ForgetAgentPlaces: vi.fn(),
  ChooseAgentPlace: vi.fn(),
  ForgetAgentPlace: vi.fn(),
  BindAgentPlace: vi.fn(),
  AgentPlacesLive: vi.fn(),
  DismissPlacesLifted: vi.fn(),
}))
vi.mock('../../../wailsjs/go/app/App', () => api)

const { default: AgentsDialog } = await import('./AgentsDialog.svelte')
const { agents } = await import('./agents.svelte')
const { emptyInfo, normInfo } = await import('./agents')
const { store } = await import('../../base/core/store.svelte')
const { setMacForTest } = await import('../../base/core/platform')

const place = (dir: string, o: Partial<AgentPlace> = {}): AgentPlace => ({
  key: `k:${dir}`,
  dir,
  label: '',
  chosen: false,
  bound: true,
  narrow: false,
  narrowRoot: false,
  state: '',
  here: '',
  ...o,
})
const ED = { can: ['allow', 'ask', 'deny'], why: '', note: '', group: 'edit', mode: 'allow' }
const perms = [
  { tool: 'add_folder', title: 'Добавить папку', ...ED },
  { tool: 'create_album', title: 'Создать альбом', ...ED },
]
const infoWith = (o: object = {}) => normInfo({ status: 'on', level: 1, edit: true, perms, ...o })
const notify = vi.spyOn(store, 'notify')
const row = () => document.querySelector<HTMLElement>('[data-row="places"]') as HTMLElement
const list = () => screen.queryByRole('list', { name: 'Места съёмок' })
const toggle = (n: number, open = false) => screen.getByRole('button', { name: `Список (${n}) ${open ? '▾' : '▸'}` })
const items = () => within(list() as HTMLElement).getAllByRole('listitem')

async function open(info = emptyInfo()) {
  api.AgentsInfo.mockResolvedValue(info)
  render(AgentsDialog)
  await agents.show()
}

beforeEach(() => {
  for (const f of Object.values(api)) f.mockReset()
  api.AgentPlacesLive.mockResolvedValue([])
  notify.mockReset()
  agents.close()
  agents.info = emptyInfo()
  agents.busy = false
  setMacForTest(false)
})
afterEach(() => {
  cleanup()
  agents.close()
  setMacForTest(null)
})

describe('список мест (153)', () => {
  it('свёрнут по умолчанию и диски не опрашивает; «Список (N) ▸» раскрывает и зовёт AgentPlacesLive один раз; повторный щелчок сворачивает', async () => {
    await open(infoWith({ places: [place('D:\\Фото'), place('E:\\Клиент')] }))
    expect(list()).toBeNull()
    expect(api.AgentPlacesLive).not.toHaveBeenCalled()
    await fireEvent.click(toggle(2))
    expect(api.AgentPlacesLive).toHaveBeenCalledTimes(1)
    expect(items().map((li) => li.querySelector('bdi')?.textContent)).toEqual(['D:\\Фото', 'E:\\Клиент'])
    await fireEvent.click(toggle(2, true))
    expect(list()).toBeNull()
    expect(api.AgentPlacesLive).toHaveBeenCalledTimes(1)
  })

  it('мест нет — кнопки «Список» нет; «Папка со съёмками…» и «Забыть места» остаются в строке мест', async () => {
    await open(infoWith())
    expect(screen.queryByRole('button', { name: /Список/ })).toBeNull()
    cleanup()
    await open(infoWith({ places: [place('D:\\Фото')] }))
    expect(within(row()).getByRole('button', { name: 'Папка со съёмками…' })).toBeTruthy()
    expect(within(row()).getByRole('button', { name: 'Забыть места' })).toBeTruthy()
  })

  it('до ответа — сохранённый путь и метка без ссылки «Привязать»; ответ подменяет путь живым (диск сменил букву)', async () => {
    let done!: (l: AgentPlace[]) => void
    api.AgentPlacesLive.mockReturnValue(new Promise<AgentPlace[]>((r) => (done = r)))
    const saved = place('E:\\Клиент', { label: 'Фото-архив', bound: false })
    await open(infoWith({ places: [saved] }))
    await fireEvent.click(toggle(1))
    expect(items()[0].textContent).toContain('E:\\Клиент')
    expect(items()[0].textContent).toContain('Фото-архив')
    expect(screen.queryByRole('button', { name: /Привязать/ })).toBeNull()
    done([{ ...saved, dir: 'G:\\Клиент', state: 'here', here: 'Фото-архив' }])
    await waitFor(() => expect(items()[0].textContent).toContain('G:\\Клиент'))
    expect(screen.getByRole('button', { name: 'Привязать к диску «Фото-архив»' })).toBeTruthy()
  })

  it('состояния: «Диска «…» нет» (бледная, без «Привязать»), «Диска нет», «Папка не найдена»', async () => {
    const a = place('E:\\А', { bound: false })
    const b = place('F:\\Б')
    const c = place('H:\\В')
    api.AgentPlacesLive.mockResolvedValue([
      { ...a, state: 'nodisk', label: 'Фото-архив' },
      { ...b, state: 'nodisk' },
      { ...c, state: 'nofolder' },
    ])
    await open(infoWith({ places: [a, b, c] }))
    await fireEvent.click(toggle(3))
    await waitFor(() => expect(items()[0].textContent).toContain('Диска «Фото-архив» нет'))
    expect(items()[1].textContent).toContain('Диска нет')
    expect(items()[2].textContent).toContain('Папка не найдена')
    expect(items().every((li) => li.classList.contains('dim'))).toBe(true)
    expect(screen.queryByRole('button', { name: /Привязать/ })).toBeNull()
  })

  it('подсказки узких мест: год — с именем папки над ним, корень диска — «Диск E: целиком»', async () => {
    const y = place('D:\\Фото\\2026', { narrow: true })
    const r = place('E:\\2026', { narrow: true, narrowRoot: true })
    await open(infoWith({ places: [y, r] }))
    await fireEvent.click(toggle(2))
    expect(items()[0].textContent).toContain('Новый год спросит снова. Фото целиком — «Папка со съёмками…»')
    expect(items()[1].textContent).toContain('Диск E: целиком — «Папка со съёмками…»')
    expect(items()[1].textContent).not.toContain('Новый год')
  })

  it('✕ «Убрать из списка» зовёт ForgetAgentPlace(key); aria-label с путём; список опрашивается заново; последнее место — список схлопывается', async () => {
    const a = place('D:\\Фото')
    const b = place('E:\\Клиент')
    api.ForgetAgentPlace.mockResolvedValueOnce(infoWith({ places: [b] })).mockResolvedValueOnce(infoWith())
    await open(infoWith({ places: [a, b] }))
    await fireEvent.click(toggle(2))
    const x = within(items()[0]).getByRole('button', { name: 'Убрать D:\\Фото из списка' })
    expect(x.getAttribute('title')).toBe('Убрать из списка')
    await fireEvent.click(x)
    expect(api.ForgetAgentPlace).toHaveBeenCalledWith(a.key)
    await waitFor(() => expect(items()).toHaveLength(1))
    expect(api.AgentPlacesLive).toHaveBeenCalledTimes(2)
    await fireEvent.click(within(items()[0]).getByRole('button', { name: /Убрать/ }))
    expect(api.ForgetAgentPlace).toHaveBeenLastCalledWith(b.key)
    await waitFor(() => expect(list()).toBeNull())
    expect(agents.placesOpen).toBe(false)
  })

  it('непривязанное место: «не привязано к диску» и «Привязать к диску «метка»» → BindAgentPlace(key); после — привязано, ссылки нет', async () => {
    const a = place('E:\\Клиент', { bound: false })
    const bound = { ...a, bound: true, label: 'Фото-архив' }
    api.AgentPlacesLive.mockResolvedValueOnce([{ ...a, state: 'here', here: 'Фото-архив' }]).mockResolvedValueOnce([bound])
    api.BindAgentPlace.mockResolvedValue(infoWith({ places: [bound] }))
    await open(infoWith({ places: [a] }))
    await fireEvent.click(toggle(1))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Привязать к диску «Фото-архив»' })).toBeTruthy())
    expect(items()[0].textContent).toContain('не привязано к диску')
    await fireEvent.click(screen.getByRole('button', { name: /Привязать/ }))
    expect(api.BindAgentPlace).toHaveBeenCalledExactlyOnceWith(a.key)
    await waitFor(() => expect(screen.queryByRole('button', { name: /Привязать/ })).toBeNull())
    expect(items()[0].textContent).not.toContain('не привязано')
    expect(items()[0].textContent).toContain('Фото-архив')
  })

  it('диск без метки — «Привязать к этому диску»; не узнали диск — ошибка Go с заглавной, список остаётся', async () => {
    const a = place('E:\\Клиент', { bound: false })
    api.AgentPlacesLive.mockResolvedValue([{ ...a, state: 'here' }])
    api.BindAgentPlace.mockRejectedValue(new Error('не удалось узнать диск'))
    await open(infoWith({ places: [a] }))
    await fireEvent.click(toggle(1))
    await fireEvent.click(await screen.findByRole('button', { name: 'Привязать к этому диску' }))
    await waitFor(() => expect(notify).toHaveBeenCalledWith('Не удалось узнать диск', 'error'))
    expect(items()).toHaveLength(1)
  })

  it('macOS: привязки нет — ни «не привязано», ни ссылки', async () => {
    setMacForTest(true)
    const a = place('/Volumes/A/Фото', { bound: false })
    api.AgentPlacesLive.mockResolvedValue([{ ...a, state: 'here', here: 'A' }])
    await open(infoWith({ places: [a] }))
    await fireEvent.click(toggle(1))
    await waitFor(() => expect(api.AgentPlacesLive).toHaveBeenCalled())
    expect(items()[0].textContent).not.toContain('не привязано')
    expect(screen.queryByRole('button', { name: /Привязать/ })).toBeNull()
  })

  it('ответ AgentPlacesLive упал — тост с заглавной, строки остаются сохранёнными', async () => {
    api.AgentPlacesLive.mockRejectedValue(new Error('время вышло'))
    await open(infoWith({ places: [place('D:\\Фото')] }))
    await fireEvent.click(toggle(1))
    await waitFor(() => expect(notify).toHaveBeenCalledWith('Время вышло', 'error'))
    expect(items()).toHaveLength(1)
  })

  it('закрыли окно — список свёрнут, живое забыто (диски без раскрытия не опрашиваются)', async () => {
    await open(infoWith({ places: [place('D:\\Фото')] }))
    await fireEvent.click(toggle(1))
    await waitFor(() => expect(agents.placesNow).not.toBeNull())
    agents.close()
    expect(agents.placesOpen).toBe(false)
    expect(agents.placesNow).toBeNull()
  })

  it('строка мест видна при «Запретить» у папок, если есть заметка о подъёме', async () => {
    const lifted = [{ from: 'D:\\Фото\\2026', to: 'D:\\Фото' }]
    const off = perms.map((p) => ({ ...p, mode: 'deny' }))
    await open(infoWith({ perms: off, places: [place('D:\\Фото')], lifted }))
    expect(row()).toBeTruthy()
  })
})

describe('заметка о подъёме мест 0.34 (153, 0.35-153-16)', () => {
  const lifted = [{ from: 'D:\\Фото\\2026', to: 'D:\\Фото' }]
  const note = () => document.querySelector<HTMLElement>('[data-row="lifted"]')

  it('висит над группами: текст, «Понятно», «Настроить»; нет подъёма — нет заметки', async () => {
    await open(infoWith({ places: [place('D:\\Фото')], lifted }))
    expect(note()?.textContent).toContain('Агент теперь берёт папки без вопроса из D:\\Фото (раньше — из D:\\Фото\\2026)')
    expect([...(note() as HTMLElement).querySelectorAll('button')].map((b) => b.textContent)).toEqual(['Понятно', 'Настроить'])
    cleanup()
    await open(infoWith({ places: [place('D:\\Фото')] }))
    expect(note()).toBeNull()
  })

  it('несколько мест: по строке на «куда», «откуда» через запятую', async () => {
    const two = [...lifted, { from: 'D:\\Фото\\2025', to: 'D:\\Фото' }, { from: 'E:\\2026', to: 'E:\\Архив' }]
    await open(infoWith({ lifted: two }))
    expect([...(note() as HTMLElement).querySelectorAll('p')].map((p) => p.textContent)).toEqual([
      'Агент теперь берёт папки без вопроса из D:\\Фото (раньше — из D:\\Фото\\2026, D:\\Фото\\2025)',
      'Агент теперь берёт папки без вопроса из E:\\Архив (раньше — из E:\\2026)',
    ])
  })

  it('«Понятно» зовёт DismissPlacesLifted и заметка исчезает', async () => {
    api.DismissPlacesLifted.mockResolvedValue(infoWith({ places: [place('D:\\Фото')] }))
    await open(infoWith({ places: [place('D:\\Фото')], lifted }))
    await fireEvent.click(screen.getByRole('button', { name: 'Понятно' }))
    expect(api.DismissPlacesLifted).toHaveBeenCalledTimes(1)
    await waitFor(() => expect(note()).toBeNull())
  })

  it('«Настроить» раскрывает список мест и подсвечивает строку мест; заметка остаётся до «Понятно»', async () => {
    await open(infoWith({ places: [place('D:\\Фото')], lifted }))
    await fireEvent.click(screen.getByRole('button', { name: 'Настроить' }))
    await waitFor(() => expect(list()).toBeTruthy())
    expect(api.AgentPlacesLive).toHaveBeenCalledTimes(1)
    expect(row().classList.contains('flash')).toBe(true)
    expect(note()).toBeTruthy()
  })

  it('тост-указатель agents:placesLifted: «Места съёмок для агента расширены · Посмотреть»; «Посмотреть» открывает окно на заметке', () => {
    api.AgentsInfo.mockResolvedValue(infoWith({ places: [place('D:\\Фото')], lifted }))
    const off = agents.watch()
    ev.on.get('agents:placesLifted')?.({ from: ['D:\\Фото\\2026'], to: ['D:\\Фото'] })
    expect(store.toast?.text).toBe('Места съёмок для агента расширены')
    expect(store.toast?.action?.label).toBe('Посмотреть')
    const show = vi.spyOn(agents, 'show').mockResolvedValue()
    store.toast?.action?.run()
    expect(show).toHaveBeenCalledExactlyOnceWith('lifted')
    show.mockRestore()
    off()
  })

  it('«Посмотреть» на закрытом окне: окно открывается, заметка подсвечена', async () => {
    api.AgentsInfo.mockResolvedValue(infoWith({ places: [place('D:\\Фото')], lifted }))
    render(AgentsDialog)
    const off = agents.watch()
    ev.on.get('agents:placesLifted')?.({ from: [], to: [] })
    store.toast?.action?.run()
    await waitFor(() => expect(note()).toBeTruthy())
    await waitFor(() => expect(note()?.classList.contains('flash')).toBe(true))
    off()
  })
})
