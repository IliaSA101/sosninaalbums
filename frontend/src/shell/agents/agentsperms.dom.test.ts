// @vitest-environment happy-dom
import { tick } from 'svelte'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'
import type { AgentPlace } from './agents'

// Блок «Разрешения» окна «ИИ-агенты» (150): строки, группы, согласия с местами, «Спрашивать» у чтения, окно на строке.
// Биндинги wailsjs подменены; решения — в Go, здесь только вызовы.
const ev = vi.hoisted(() => ({ on: new Map<string, (d: unknown) => void>() }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({
  EventsOn: (name: string, cb: (d: unknown) => void) => {
    ev.on.set(name, cb)
    return () => ev.on.delete(name)
  },
}))
const api = vi.hoisted(() => ({
  AgentsInfo: vi.fn(),
  SetAgents: vi.fn(),
  SetAgentsLevel: vi.fn(),
  ClearAgentsLog: vi.fn(),
  SaveClaudeExtension: vi.fn(),
  SetAgentPerm: vi.fn(),
  SetAgentPermGroup: vi.fn(),
  ForgetAgentPlaces: vi.fn(),
  ChooseAgentPlace: vi.fn(),
  AllowAgentAsks: vi.fn(),
}))
vi.mock('../../../wailsjs/go/app/App', () => api)

const { default: AgentsDialog } = await import('./AgentsDialog.svelte')
const { agents } = await import('./agents.svelte')
const { emptyInfo, normInfo } = await import('./agents')
const { store } = await import('../../base/core/store.svelte')
const { keymap } = await import('../../base/keys/keymap.svelte')
const { setMacForTest } = await import('../../base/core/platform')

const setup = (id: string, o: object = {}) => ({ id, title: `Клиент ${id}`, text: `команда-${id}`, note: '', file: false, ...o })
const infoOn = (o: object = {}) => normInfo({ status: 'on', level: 1, setups: [setup('a')], ...o })
const btn = (name: string | RegExp) => screen.getByRole('button', { name })

const spies = {
  info: api.AgentsInfo,
  set: api.SetAgents,
  setLevel: api.SetAgentsLevel,
  clearLog: api.ClearAgentsLog,
  saveExtension: api.SaveClaudeExtension,
  setPerm: api.SetAgentPerm,
  setGroup: api.SetAgentPermGroup,
  forget: api.ForgetAgentPlaces,
  choosePlace: api.ChooseAgentPlace,
  allowAsks: api.AllowAgentAsks,
}
const notify = vi.spyOn(store, 'notify')
const writeText = vi.fn(async (_t: string) => {})

// Открыть окно с ответом Go на AgentsInfo.
async function open(info = emptyInfo()) {
  spies.info.mockResolvedValue(info)
  render(AgentsDialog)
  await agents.show()
}

beforeEach(() => {
  for (const s of Object.values(spies)) s.mockReset()
  notify.mockReset()
  writeText.mockReset()
  Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
  agents.close()
  agents.info = emptyInfo()
  agents.busy = false
})
afterEach(() => {
  cleanup()
  agents.close()
})

// Строки разрешений как их даёт Go (design §6): чтение без «Спрашивать», правки с тремя значениями, запуск — вне группы.
const WHY_READ = 'Чтение не спрашивает: на одну просьбу агент читает десятки раз. Можно разрешить или запретить.'
const WHY_READ_FULL = WHY_READ + ' Ограничить чтение можно выше, в «Что видит агент».'
// Подпись, которую Go кладёт у папки и альбома (AgentPerm.note).
const NOTE_DIR = 'Без вопроса — в местах съёмок'
const rd = (tool: string, title: string, o: object = {}) => ({
  tool,
  title,
  group: 'read',
  mode: 'allow',
  can: ['allow', 'deny'],
  why: WHY_READ,
  note: '',
  ...o,
})
const ed = (tool: string, title: string, mode = 'deny', o: object = {}) => ({
  tool,
  title,
  group: 'edit',
  mode,
  can: ['allow', 'ask', 'deny'],
  why: '',
  note: '',
  ...o,
})
const permRows = (m: Record<string, string> = {}) => [
  rd('get_album', 'Сводка альбома'),
  rd('list_spreads', 'Список разворотов'),
  ed('create_album', 'Создать альбом', m.create_album, { note: NOTE_DIR }),
  ed('add_folder', 'Добавить папку', m.add_folder, { note: NOTE_DIR }),
  ed('autobuild', 'Автосборка', m.autobuild),
  {
    tool: 'start_program',
    title: 'Запустить программу',
    group: 'other',
    mode: 'allow',
    can: ['allow', 'deny'],
    why: 'Программа закрыта — спросить негде',
    note: '',
  },
  { tool: 'show_spread', title: 'Показать разворот', group: 'other', mode: 'allow', can: ['allow', 'ask', 'deny'], why: '', note: '' },
]
const place = (dir: string, chosen = false, o: Partial<AgentPlace> = {}): AgentPlace => ({
  key: dir,
  dir,
  label: '',
  chosen,
  bound: true,
  narrow: false,
  narrowRoot: false,
  state: '',
  here: '',
  ...o,
})
const rowOf = (tool: string) => document.querySelector<HTMLElement>(`[data-row="tool:${tool}"]`) as HTMLElement
const inRow = (tool: string, name: string) => within(rowOf(tool)).getByRole('button', { name })
const groupSeg = (name: string) => screen.getByRole('group', { name })
const inGroup = (group: string, name: string) => within(groupSeg(group)).getByRole('button', { name })
const pressed = (el: HTMLElement) => el.getAttribute('aria-pressed') === 'true'
const placesRow = () => document.querySelector<HTMLElement>('[data-row="places"]') as HTMLElement
const consentBox = () => screen.getAllByRole('dialog')[1]

describe('разрешения агентов (150)', () => {
  const withPerms = (o: object = {}, m: Record<string, string> = {}) => infoOn({ perms: permRows(m), ...o })
  const ALL = { create_album: 'allow', add_folder: 'allow', autobuild: 'allow' }

  it('блок «Разрешения» при включённом доступе: «Только чтение» свёрнута, «Правки» раскрыта, у обеих счётчик; доступ выключен — блока нет', async () => {
    await open(withPerms())
    expect(screen.getByRole('heading', { name: 'Разрешения' })).toBeTruthy()
    expect(screen.getByText('Когда агент может пользоваться инструментами программы. «Спрашивать» — вопрос в окне программы.')).toBeTruthy()
    const [read, edit] = [...document.querySelectorAll<HTMLDetailsElement>('details.grp')]
    expect([read.open, edit.open]).toEqual([false, true])
    expect(read.querySelector('.count')?.textContent).toBe('2')
    expect(edit.querySelector('.count')?.textContent).toBe('5') // три правки и два «вне группы»
    cleanup()
    agents.close()
    await open()
    expect(screen.queryByRole('heading', { name: 'Разрешения' })).toBeNull()
  })

  it('у группы и строки три значения; строка отмечает своё, группа — общее или «разные»', async () => {
    await open(withPerms({ edit: true }, ALL))
    const names = [...rowOf('autobuild').querySelectorAll('button')].map((b) => b.textContent)
    expect(names).toEqual(['Всегда', 'Спрашивать', 'Запретить'])
    expect(pressed(inRow('autobuild', 'Всегда'))).toBe(true)
    expect(pressed(inGroup('Правки', 'Всегда'))).toBe(true)
    expect(screen.queryByText('разные')).toBeNull()
    cleanup()
    agents.close()
    await open(withPerms({ edit: true }, { create_album: 'allow', add_folder: 'ask' }))
    expect(screen.getByText('разные')).toBeTruthy()
    expect(within(groupSeg('Правки')).getAllByRole('button').some(pressed)).toBe(false)
  })

  it('правки уже включены: смена значения — сразу SetAgentPerm, без согласия', async () => {
    spies.setPerm.mockResolvedValue(withPerms({ edit: true }, { autobuild: 'ask' }))
    await open(withPerms({ edit: true }, { autobuild: 'allow' }))
    await fireEvent.click(inRow('autobuild', 'Спрашивать'))
    expect(spies.setPerm).toHaveBeenCalledExactlyOnceWith('autobuild', 'ask')
    expect(screen.queryByText('Разрешить агентам ИИ менять альбом?')).toBeNull()
    await waitFor(() => expect(pressed(inRow('autobuild', 'Спрашивать'))).toBe(true))
  })

  it('повторный щелчок по отмеченному ничего не зовёт; «Запретить» — сразу, без вопроса', async () => {
    spies.setPerm.mockResolvedValue(withPerms({ edit: true }))
    await open(withPerms({ edit: true }, { autobuild: 'allow' }))
    await fireEvent.click(inRow('autobuild', 'Всегда'))
    expect(spies.setPerm).not.toHaveBeenCalled()
    await fireEvent.click(inRow('autobuild', 'Запретить'))
    expect(spies.setPerm).toHaveBeenCalledExactlyOnceWith('autobuild', 'deny')
    expect(screen.queryByText('Разрешить агентам ИИ менять альбом?')).toBeNull()
  })

  it('первый выход правки из «Запретить» → согласие 149 (клавиша отмены из реестра), вид прежний, SetAgentPerm не зовут', async () => {
    // Клавиша отмены в тексте — из реестра: у владельца (Windows, Linux) это дословно «Ctrl+Z»
    setMacForTest(false)
    keymap.apply({ commands: [{ id: 'undo', keys: ['Mod+KeyZ'] }] } as never)
    await open(withPerms())
    await fireEvent.click(inRow('autobuild', 'Спрашивать'))
    expect(pressed(inRow('autobuild', 'Запретить'))).toBe(true)
    expect(screen.getByText('Разрешить агентам ИИ менять альбом?')).toBeTruthy()
    expect(
      screen.getByText(
        'Агент сможет создавать альбомы, собирать развороты и менять раскладки. Каждая правка — один шаг Ctrl+Z; перед первой правкой — резервная копия. Что спрашивать, а что делать сразу — настраивается ниже.',
      ),
    ).toBeTruthy()
    expect(screen.queryByText(/Облачный агент увидит имена файлов/)).toBeNull() // у обычной правки абзаца о папках нет
    expect(spies.setPerm).not.toHaveBeenCalled()
    setMacForTest(null)
  })

  it('согласие: «Отмена» слева, «Разрешить» справа; «Отмена» ничего не зовёт, «Разрешить» → SetAgentPerm', async () => {
    spies.setPerm.mockResolvedValue(withPerms({ edit: true }, { autobuild: 'ask' }))
    await open(withPerms())
    await fireEvent.click(inRow('autobuild', 'Спрашивать'))
    const ask = btn('Разрешить').parentElement!
    expect([...ask.querySelectorAll('button')].map((b) => b.textContent)).toEqual(['Отмена', 'Разрешить'])
    await fireEvent.click(btn('Отмена'))
    expect(screen.queryByText('Разрешить агентам ИИ менять альбом?')).toBeNull()
    expect(spies.setPerm).not.toHaveBeenCalled()
    await fireEvent.click(inRow('autobuild', 'Спрашивать'))
    await fireEvent.click(btn('Разрешить'))
    expect(spies.setPerm).toHaveBeenCalledExactlyOnceWith('autobuild', 'ask')
    await waitFor(() => expect(pressed(inRow('autobuild', 'Спрашивать'))).toBe(true))
  })

  it('Go отказал (ошибка) — тост, строка остаётся прежней', async () => {
    spies.setPerm.mockRejectedValue(new Error('не удалось записать настройку'))
    spies.info.mockResolvedValue(withPerms({ edit: true }, { autobuild: 'allow' }))
    await open(withPerms({ edit: true }, { autobuild: 'allow' }))
    await fireEvent.click(inRow('autobuild', 'Запретить'))
    await waitFor(() => expect(notify).toHaveBeenCalledWith('Не удалось записать настройку', 'error'))
    expect(pressed(inRow('autobuild', 'Всегда'))).toBe(true)
  })

  it('группа «Правки» из «Запретить» в «Всегда»: один вопрос с обоими абзацами → SetAgentPermGroup(edit, allow)', async () => {
    spies.setGroup.mockResolvedValue(withPerms({ edit: true }, ALL))
    await open(withPerms({ places: [place('D:\\Фото', true)] }))
    await fireEvent.click(inGroup('Правки', 'Всегда'))
    expect(screen.getAllByRole('dialog')).toHaveLength(2) // окно и один вопрос
    expect(screen.getByText('Разрешить агентам ИИ менять альбом?')).toBeTruthy()
    expect(consentBox().textContent).toContain('Агент сможет сам добавить папку, которую назовёт')
    expect(spies.setGroup).not.toHaveBeenCalled()
    await fireEvent.click(btn('Разрешить'))
    expect(spies.setGroup).toHaveBeenCalledExactlyOnceWith('edit', 'allow')
  })

  it('группа «Правки»: «Спрашивать» — вызов группы один; «Запустить программу» и «Показать разворот» подписаны как не меняющие альбом', async () => {
    spies.setGroup.mockResolvedValue(withPerms({ edit: true }))
    await open(withPerms({ edit: true }, ALL))
    await fireEvent.click(inGroup('Правки', 'Спрашивать'))
    expect(spies.setGroup).toHaveBeenCalledExactlyOnceWith('edit', 'ask')
    expect(spies.setPerm).not.toHaveBeenCalled()
    expect(screen.getByText('Не меняют альбом — групповой выбор их не трогает')).toBeTruthy()
  })

  it('«Всегда» у папки при включённых правках → вопрос только про папки: заголовок, три абзаца, два пути из мест, «Отмена» слева', async () => {
    const places = [place('D:\\Фото', true), place('D:\\Фото\\2026'), place('E:\\Архив')]
    await open(withPerms({ edit: true, places }, { add_folder: 'ask', autobuild: 'ask' }))
    await fireEvent.click(inRow('add_folder', 'Всегда'))
    expect(screen.getByText('Добавлять папки по просьбе агента без вопроса?')).toBeTruthy()
    expect(screen.queryByText('Разрешить агентам ИИ менять альбом?')).toBeNull()
    const t = consentBox().textContent
    expect(t).toContain(
      'Агент сможет сам добавить папку, которую назовёт, — в том числе прочитав её на странице или в файле, — и создать из неё альбом.',
    )
    expect(t).toContain('Без вопроса — только внутри ваших папок со съёмками: D:\\Фото, D:\\Фото\\2026 и ещё 1')
    expect(t).toContain('; на этом компьютере и не больше 5000 фото. Иначе, и в первый раз из новой папки, программа спросит.')
    expect(t).toContain('Облачный агент увидит имена файлов, а при «+ картинки» — и фото.')
    expect([...consentBox().querySelectorAll('bdi')].map((b) => b.textContent)).toEqual(['D:\\Фото', 'D:\\Фото\\2026'])
    expect([...btn('Разрешить').parentElement!.querySelectorAll('button')].map((b) => b.textContent)).toEqual(['Отмена', 'Разрешить'])
  })

  it('«Всегда» у уже «Всегда» папки и «Запретить» у «Спрашивать» согласия не просят', async () => {
    spies.setPerm.mockResolvedValue(withPerms({ edit: true }))
    await open(withPerms({ edit: true }, { add_folder: 'allow', create_album: 'ask' }))
    await fireEvent.click(inRow('add_folder', 'Всегда'))
    await fireEvent.click(inRow('create_album', 'Запретить'))
    expect(screen.queryByText('Добавлять папки по просьбе агента без вопроса?')).toBeNull()
    expect(spies.setPerm).toHaveBeenCalledExactlyOnceWith('create_album', 'deny')
  })

  it('мест нет: в согласии «не выбраны»; «Папка со съёмками…» зовёт ChooseAgentPlace, места в вопросе обновляются; «Разрешить» можно и без мест', async () => {
    spies.choosePlace.mockResolvedValue(withPerms({ edit: true, places: [place('D:\\Съёмки', true)] }, { add_folder: 'ask' }))
    spies.setPerm.mockResolvedValue(withPerms({ edit: true }, { add_folder: 'allow' }))
    await open(withPerms({ edit: true }, { add_folder: 'ask' }))
    await fireEvent.click(inRow('add_folder', 'Всегда'))
    expect(consentBox().textContent).toContain('ваших папок со съёмками: не выбраны · Папка со съёмками…')
    await fireEvent.click(within(consentBox()).getByRole('button', { name: 'Папка со съёмками…' }))
    expect(spies.choosePlace).toHaveBeenCalledTimes(1)
    await waitFor(() => expect(consentBox().textContent).toContain('ваших папок со съёмками: D:\\Съёмки · Папка со съёмками…'))
    await fireEvent.click(btn('Разрешить'))
    expect(spies.setPerm).toHaveBeenCalledExactlyOnceWith('add_folder', 'allow')
  })

  it('строка мест при «Всегда» у папки: два пути, «и ещё N», «Папка со съёмками…», «Забыть места» → ForgetAgentPlaces', async () => {
    spies.forget.mockResolvedValue(withPerms({ edit: true }, { add_folder: 'allow' }))
    const places = [place('D:\\Фото', true), place('D:\\Фото\\2026'), place('E:\\Архив'), place('F:\\Старое')]
    await open(withPerms({ edit: true, places }, { add_folder: 'allow' }))
    expect(placesRow().textContent).toContain('Без вопроса — внутри: D:\\Фото, D:\\Фото\\2026 и ещё 2')
    expect([...placesRow().querySelectorAll('bdi')].map((b) => b.textContent)).toEqual(['D:\\Фото', 'D:\\Фото\\2026'])
    await fireEvent.click(within(placesRow()).getByRole('button', { name: 'Забыть места' }))
    expect(spies.forget).toHaveBeenCalledTimes(1)
  })

  it('в строке мест — «Список (N) ▸» со счётом всех мест (не только показанных двух); список свёрнут, диски не опрашиваются', async () => {
    const places = [place('D:\\Фото', true), place('D:\\Фото\\2026'), place('E:\\Архив'), place('F:\\Старое')]
    await open(withPerms({ edit: true, places }, { add_folder: 'allow' }))
    expect(within(placesRow()).getByRole('button', { name: 'Список (4) ▸' }).getAttribute('aria-expanded')).toBe('false')
    expect(screen.queryByRole('list', { name: 'Места съёмок' })).toBeNull()
  })

  it('мест нет: «Без вопроса — нигде…», «Забыть места» нет; нет «Всегда» у папок — строки мест нет', async () => {
    await open(withPerms({ edit: true }, { add_folder: 'allow' }))
    expect(placesRow().textContent).toContain('Без вопроса — нигде: выберите папку со съёмками')
    expect(within(placesRow()).getByRole('button', { name: 'Папка со съёмками…' })).toBeTruthy()
    expect(within(placesRow()).queryByRole('button', { name: 'Забыть места' })).toBeNull()
    cleanup()
    agents.close()
    await open(withPerms({ edit: true }, { add_folder: 'ask', create_album: 'deny' }))
    expect(document.querySelector('[data-row="places"]')).toBeNull()
  })

  it('подпись «Без вопроса — в местах съёмок» у папки и альбома — только при «Всегда»; «Спрашивать» и «Запретить» её прячут', async () => {
    const seen = () => ['create_album', 'add_folder'].map((t) => rowOf(t).textContent?.includes(NOTE_DIR))
    await open(withPerms({ edit: true }, { create_album: 'allow', add_folder: 'allow' }))
    expect(seen()).toEqual([true, true])
    cleanup()
    agents.close()
    await open(withPerms({ edit: true }, { create_album: 'ask', add_folder: 'ask' }))
    expect(seen()).toEqual([false, false])
    cleanup()
    agents.close()
    await open(withPerms({ edit: true }, { create_album: 'deny', add_folder: 'allow' }))
    expect(seen()).toEqual([false, true])
  })

  it('«Папка со съёмками…» бросила — тост с ошибкой', async () => {
    spies.choosePlace.mockRejectedValue(new Error('выбор папки не открылся'))
    await open(withPerms({ edit: true }, { add_folder: 'allow' }))
    await fireEvent.click(within(placesRow()).getByRole('button', { name: 'Папка со съёмками…' }))
    await waitFor(() => expect(notify).toHaveBeenCalledWith('Выбор папки не открылся', 'error'))
  })

  it('«Спрашивать» у чтения: не серый, значение то же, Go не зовётся, под строкой причина, «Что видит агент» подсвечено', async () => {
    await open(withPerms())
    ;(document.querySelector('details.grp') as HTMLDetailsElement).open = true
    await tick()
    const ask = inRow('get_album', 'Спрашивать')
    expect(ask.getAttribute('aria-disabled')).toBeNull()
    await fireEvent.click(ask)
    expect(spies.setPerm).not.toHaveBeenCalled()
    expect(pressed(inRow('get_album', 'Всегда'))).toBe(true)
    expect(within(rowOf('get_album')).getByText(WHY_READ_FULL)).toBeTruthy()
    expect(screen.getByRole('heading', { name: 'Что видит агент' }).classList.contains('flash')).toBe(true)
    expect(within(rowOf('list_spreads')).queryByText(/Чтение не спрашивает/)).toBeNull() // только у нажатой строки
  })

  it('«Спрашивать» у группы чтения: причина под заголовком группы, подсветка, группа не меняется', async () => {
    await open(withPerms())
    await fireEvent.click(inGroup('Только чтение', 'Спрашивать'))
    expect(spies.setGroup).not.toHaveBeenCalled()
    expect(screen.getByText(WHY_READ_FULL)).toBeTruthy()
    expect(screen.getByRole('heading', { name: 'Что видит агент' }).classList.contains('flash')).toBe(true)
  })

  it('«Спрашивать» у запуска: причина строки из Go, «Что видит агент» не подсвечено', async () => {
    await open(withPerms({ edit: true }))
    await fireEvent.click(inRow('start_program', 'Спрашивать'))
    expect(spies.setPerm).not.toHaveBeenCalled()
    expect(within(rowOf('start_program')).getByText('Программа закрыта — спросить негде')).toBeTruthy()
    expect(screen.getByRole('heading', { name: 'Что видит агент' }).classList.contains('flash')).toBe(false)
  })

  it('чтение «Запретить» — сразу, без согласия: SetAgentPermGroup(read, deny)', async () => {
    spies.setGroup.mockResolvedValue(withPerms())
    await open(withPerms())
    await fireEvent.click(inGroup('Только чтение', 'Запретить'))
    expect(spies.setGroup).toHaveBeenCalledExactlyOnceWith('read', 'deny')
  })

  it('значение вне can (у Go его нет) серое и не зовёт Go', async () => {
    await open(infoOn({ perms: [ed('export_album', 'Выгрузить', 'ask', { can: ['ask', 'deny'] })], edit: true }))
    const all = inRow('export_album', 'Всегда')
    expect(all.getAttribute('aria-disabled')).toBe('true')
    await fireEvent.click(all)
    expect(spies.setPerm).not.toHaveBeenCalled()
  })

  it('окно на строке: show(rowKey) раскрывает свёрнутую группу, подсвечивает строку и снимает подсветку позже', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    try {
      spies.info.mockResolvedValue(withPerms())
      render(AgentsDialog)
      await agents.show('tool:get_album')
      await tick()
      expect((document.querySelector('details.grp') as HTMLDetailsElement).open).toBe(true)
      expect(rowOf('get_album').classList.contains('flash')).toBe(true)
      vi.advanceTimersByTime(1700)
      await tick()
      expect(rowOf('get_album').classList.contains('flash')).toBe(false)
    } finally {
      vi.useRealTimers()
    }
  })

  it('окно на строке мест: подсветка; окно уже открыто — открывается заново', async () => {
    spies.info.mockResolvedValue(withPerms({ edit: true }, { add_folder: 'allow' }))
    render(AgentsDialog)
    await agents.show()
    await agents.show('places')
    await tick()
    expect(agents.open).toBe(true)
    expect(placesRow().classList.contains('flash')).toBe(true)
  })

  it('от администратора: сегменты правок серые и ничего не зовут, рядом причина', async () => {
    await open(withPerms({ editBlocked: 'elevated' }))
    expect(screen.getByText('Программа запущена от администратора — запустите её обычным способом')).toBeTruthy()
    const b = inRow('autobuild', 'Спрашивать')
    expect(b.getAttribute('aria-disabled')).toBe('true')
    await fireEvent.click(b)
    await fireEvent.click(inGroup('Правки', 'Всегда'))
    expect(spies.setPerm).not.toHaveBeenCalled()
    expect(spies.setGroup).not.toHaveBeenCalled()
    expect(screen.queryByText('Разрешить агентам ИИ менять альбом?')).toBeNull()
  })

  it('не от администратора — подписи причины нет', async () => {
    await open(withPerms())
    expect(screen.queryByText(/запущена от администратора/)).toBeNull()
  })

  it('perms = null и places = null от Go не ломают окно: щелчок по пустой группе ничего не зовёт', async () => {
    await open(infoOn({ perms: null, places: null }))
    expect(screen.getByRole('heading', { name: 'Разрешения' })).toBeTruthy()
    await fireEvent.click(inGroup('Правки', 'Всегда'))
    expect(spies.setGroup).not.toHaveBeenCalled()
  })
})

// Смешанные строки (150, design §6): AgentsInfo.edit = «правки не все «Запретить»», а не зеркало одной строки.
describe('согласие на правки при смешанных строках (150)', () => {
  const ASK_EDIT_TITLE = 'Разрешить агентам ИИ менять альбом?'
  const ASK_FOLDER_TITLE = 'Добавлять папки по просьбе агента без вопроса?'
  // Как после первого согласия: «Автосборка» — «Спрашивать», прочие правки — «Запретить».
  const mixed = (edit: boolean, m: Record<string, string> = {}) =>
    infoOn({
      edit,
      perms: [...permRows({ autobuild: edit ? 'ask' : 'deny', ...m }), ed('vary_layouts', 'Изменить раскладки', m.vary_layouts)],
    })

  it('edit: false, все правки «Запретить» → «Автосборка» в «Спрашивать» спрашивает согласие на правки', async () => {
    await open(mixed(false))
    await fireEvent.click(inRow('autobuild', 'Спрашивать'))
    expect(screen.getByText(ASK_EDIT_TITLE)).toBeTruthy()
    expect(screen.queryByText(ASK_FOLDER_TITLE)).toBeNull()
    expect(spies.setPerm).not.toHaveBeenCalled()
  })

  it('edit: true, «Автосборка» «Спрашивать», прочие «Запретить» → «Изменить раскладки» в «Всегда» без согласия, применяется сразу', async () => {
    spies.setPerm.mockResolvedValue(mixed(true, { vary_layouts: 'allow' }))
    await open(mixed(true))
    await fireEvent.click(inRow('vary_layouts', 'Всегда'))
    expect(screen.queryByText(ASK_EDIT_TITLE)).toBeNull()
    expect(spies.setPerm).toHaveBeenCalledExactlyOnceWith('vary_layouts', 'allow')
    await waitFor(() => expect(pressed(inRow('vary_layouts', 'Всегда'))).toBe(true))
  })

  it('edit: true, «Добавить папку» в «Всегда» → согласие только папки, без абзаца правок', async () => {
    await open(mixed(true, { add_folder: 'deny' }))
    await fireEvent.click(inRow('add_folder', 'Всегда'))
    expect(screen.getByText(ASK_FOLDER_TITLE)).toBeTruthy()
    expect(screen.queryByText(ASK_EDIT_TITLE)).toBeNull()
    expect(consentBox().textContent).toContain('Агент сможет сам добавить папку')
    expect(consentBox().textContent).not.toContain('Агент сможет создавать альбомы')
    expect(spies.setPerm).not.toHaveBeenCalled()
  })

  it('после «Разрешить» окно получает info с edit: true — следующий выход строки из «Запретить» идёт без вопроса', async () => {
    spies.setPerm.mockResolvedValueOnce(mixed(true)).mockResolvedValueOnce(mixed(true, { vary_layouts: 'allow' }))
    await open(mixed(false))
    await fireEvent.click(inRow('autobuild', 'Спрашивать'))
    await fireEvent.click(btn('Разрешить'))
    expect(spies.setPerm).toHaveBeenCalledExactlyOnceWith('autobuild', 'ask')
    await waitFor(() => expect(agents.info.edit).toBe(true))
    await fireEvent.click(inRow('vary_layouts', 'Всегда'))
    expect(screen.queryByText(ASK_EDIT_TITLE)).toBeNull()
    expect(spies.setPerm).toHaveBeenLastCalledWith('vary_layouts', 'allow')
    expect(spies.setPerm).toHaveBeenCalledTimes(2)
  })
})

// Строки «только с вопросом» (152, класс Q): Go отдаёт их в группе other с can = [ask, deny].
describe('строки выгрузки и открытия: только с вопросом (152)', () => {
  const q = (tool: string, title: string, mode = 'ask') => ({ tool, title, group: 'other', mode, can: ['ask', 'deny'], why: '', note: '' })
  const withQ = (mode = 'ask') =>
    infoOn({
      edit: true,
      perms: [...permRows({ create_album: 'ask' }), q('export_album', 'Выгрузить в JPEG', mode), q('open_album', 'Открыть альбом', mode)],
    })

  it('две строки среди «не меняют альбом»: «Всегда» серое, «Спрашивать» отмечено, пояснение «Спрашивает всегда»', async () => {
    await open(withQ())
    for (const t of ['export_album', 'open_album']) {
      const names = [...rowOf(t).querySelectorAll('button')].map((b) => b.textContent)
      expect(names).toEqual(['Всегда', 'Спрашивать', 'Запретить'])
      expect(inRow(t, 'Всегда').getAttribute('aria-disabled')).toBe('true')
      expect(pressed(inRow(t, 'Спрашивать'))).toBe(true)
      expect(within(rowOf(t)).getByText('Спрашивает всегда')).toBeTruthy()
    }
    expect(screen.getByText('Не меняют альбом — групповой выбор их не трогает')).toBeTruthy()
    expect(within(rowOf('create_album')).queryByText('Спрашивает всегда')).toBeNull()
  })

  it('щелчок по серому «Всегда» — ничего не уходит в Go; «Запретить» → SetAgentPerm(tool, deny)', async () => {
    spies.setPerm.mockResolvedValue(withQ('deny'))
    await open(withQ())
    await fireEvent.click(inRow('export_album', 'Всегда'))
    expect(spies.setPerm).not.toHaveBeenCalled()
    await fireEvent.click(inRow('export_album', 'Запретить'))
    expect(spies.setPerm).toHaveBeenCalledExactlyOnceWith('export_album', 'deny')
  })

  it('с why от Go «Всегда» не серое: щелчок значения не меняет, под строкой — пояснение; «Запретить» работает', async () => {
    const WHY = 'Выгрузка и открытие альбома спрашивают всегда: папку и файл подтверждаете вы. Можно запретить.'
    const info = withQ()
    info.perms = info.perms.map((p) => (p.can.length === 2 ? { ...p, why: WHY } : p))
    spies.setPerm.mockResolvedValue(info)
    await open(info)
    const b = inRow('export_album', 'Всегда')
    expect(b.getAttribute('aria-disabled')).not.toBe('true')
    expect(within(rowOf('export_album')).queryByText(WHY)).toBeNull()
    await fireEvent.click(b)
    expect(spies.setPerm).not.toHaveBeenCalled()
    expect(within(rowOf('export_album')).getByText(WHY)).toBeTruthy()
    expect(within(rowOf('open_album')).queryByText(WHY)).toBeNull()
    expect(pressed(inRow('export_album', 'Спрашивать'))).toBe(true)
    // обычная строка: «Всегда» работает как прежде
    await fireEvent.click(inRow('create_album', 'Всегда'))
    expect(spies.setPerm).toHaveBeenCalledTimes(0) // папка — сначала согласие
    expect(screen.getByText('Добавлять папки по просьбе агента без вопроса?')).toBeTruthy()
  })

  it('повторный щелчок по уже выбранному «Спрашивать» — без вызова Go', async () => {
    await open(withQ())
    await fireEvent.click(inRow('open_album', 'Спрашивать'))
    expect(spies.setPerm).not.toHaveBeenCalled()
  })

  it('групповой щелчок «Правки ▸ Запретить» Q-строк не касается', async () => {
    spies.setGroup.mockResolvedValue(withQ())
    await open(withQ())
    await fireEvent.click(inGroup('Правки', 'Запретить'))
    expect(spies.setGroup).toHaveBeenCalledExactlyOnceWith('edit', 'deny')
    expect(pressed(inRow('export_album', 'Спрашивать'))).toBe(true)
  })
})
