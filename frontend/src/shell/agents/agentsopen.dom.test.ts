// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'
import type { AgentPlace } from './agents'

// Строка «Открыть альбом» в «Разрешениях» (160, класс O): три значения, согласие ASK_OPEN, места, группа строку не трогает.
// Остальное окно — agentsperms.dom.test.ts. Биндинги wailsjs подменены.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
const api = vi.hoisted(() => ({
  AgentsInfo: vi.fn(),
  SetAgentPerm: vi.fn(),
  SetAgentPermGroup: vi.fn(),
  ChooseAgentPlace: vi.fn(),
  ForgetAgentPlaces: vi.fn(),
  AllowAgentAsks: vi.fn(),
}))
vi.mock('../../../wailsjs/go/app/App', () => api)

const { default: AgentsDialog } = await import('./AgentsDialog.svelte')
const { agents } = await import('./agents.svelte')
const { emptyInfo, normInfo } = await import('./agents')

const TITLE = 'Открывать альбомы по просьбе агента без вопроса?'
const NOTE = 'Без вопроса — альбомы с фото в местах съёмок'
const perm = (tool: string, title: string, group: string, mode: string, note = '') => ({
  tool,
  title,
  group,
  mode,
  can: ['allow', 'ask', 'deny'],
  why: '',
  note,
})
// Как отдаёт Go: правки включены, папки — «Спрашивать», открытие — в группе other.
const info = (mode = 'ask', extra: object = {}) =>
  normInfo({
    status: 'on',
    level: 1,
    edit: true,
    perms: [
      perm('create_album', 'Создать альбом', 'edit', 'ask'),
      perm('add_folder', 'Добавить папку', 'edit', 'ask'),
      perm('open_album', 'Открыть альбом', 'other', mode, NOTE),
    ],
    ...extra,
  })
const place = (dir: string): AgentPlace => ({
  key: dir,
  dir,
  label: '',
  chosen: true,
  bound: true,
  bindable: true,
  narrow: false,
  narrowRoot: false,
  state: '',
  here: '',
})
const rowOf = (tool: string) => document.querySelector<HTMLElement>(`[data-row="tool:${tool}"]`) as HTMLElement
const inRow = (name: string) => within(rowOf('open_album')).getByRole('button', { name })
const inEdit = (name: string) => within(screen.getByRole('group', { name: 'Правки' })).getByRole('button', { name })
const pressed = (el: HTMLElement) => el.getAttribute('aria-pressed') === 'true'
const consentBox = () => screen.getAllByRole('dialog')[1]
const btn = (name: string) => screen.getByRole('button', { name })

async function open(i = info()) {
  api.AgentsInfo.mockResolvedValue(i)
  render(AgentsDialog)
  await agents.show()
}

beforeEach(() => {
  for (const f of Object.values(api)) f.mockReset()
  agents.close()
  agents.info = emptyInfo()
  agents.busy = false
})
afterEach(() => {
  cleanup()
  agents.close()
})

describe('строка открытия: «Всегда» и согласие (160)', () => {
  it('три значения, «Всегда» живое; «Спрашивает всегда» нет; подпись про места — только при «Всегда»', async () => {
    await open(info('ask'))
    expect([...rowOf('open_album').querySelectorAll('button')].map((b) => b.textContent)).toEqual(['Всегда', 'Спрашивать', 'Запретить'])
    expect(inRow('Всегда').getAttribute('aria-disabled')).not.toBe('true')
    expect(pressed(inRow('Спрашивать'))).toBe(true)
    expect(within(rowOf('open_album')).queryByText('Спрашивает всегда')).toBeNull()
    expect(within(rowOf('open_album')).queryByText(NOTE)).toBeNull()
    cleanup()
    agents.close()
    await open(info('allow'))
    expect(within(rowOf('open_album')).getByText(NOTE)).toBeTruthy()
  })

  it('«Всегда» → окно согласия с местами; «Отмена» — вид прежний, SetAgentPerm не зовут; вопросов про правки и папки нет', async () => {
    await open(info('ask', { places: [place('D:\\Фото'), place('E:\\Архив'), place('F:\\Ещё')] }))
    await fireEvent.click(inRow('Всегда'))
    expect(screen.getByText(TITLE)).toBeTruthy()
    expect(screen.queryByText('Разрешить агентам ИИ менять альбом?')).toBeNull()
    expect(screen.queryByText('Добавлять папки по просьбе агента без вопроса?')).toBeNull()
    const t = consentBox().textContent
    expect(t).toContain('Агент сможет сам открыть альбом из «Последних» — в том числе прочитав просьбу на странице или в файле.')
    expect(t).toContain('Без вопроса — только если все фото альбома лежат в ваших папках со съёмками: D:\\Фото, E:\\Архив')
    expect(t).not.toContain('F:\\Ещё')
    expect(t).toContain('; не чаще 3 раз за 10 минут. Иначе программа спросит.')
    expect(t).toContain('Облачный агент увидит открытый альбом — в пределах «Что видит агент».')
    await fireEvent.click(btn('Отмена'))
    expect(screen.queryByText(TITLE)).toBeNull()
    expect(pressed(inRow('Спрашивать'))).toBe(true)
    expect(api.SetAgentPerm).not.toHaveBeenCalled()
  })

  it('«Разрешить» → SetAgentPerm(open_album, allow); мест нет — «не выбраны» и «Папка со съёмками…»', async () => {
    api.SetAgentPerm.mockResolvedValue(info('allow'))
    await open(info('deny'))
    await fireEvent.click(inRow('Всегда'))
    expect(consentBox().textContent).toContain('ваших папках со съёмками: не выбраны ·')
    expect(within(consentBox()).getByRole('button', { name: 'Папка со съёмками…' })).toBeTruthy()
    await fireEvent.click(btn('Разрешить'))
    expect(api.SetAgentPerm).toHaveBeenCalledExactlyOnceWith('open_album', 'allow')
    await waitFor(() => expect(pressed(inRow('Всегда'))).toBe(true))
  })

  it('«Всегда» уже стоит — щелчок без вопроса и вызова; «Спрашивать» и «Запретить» — сразу, без согласия', async () => {
    api.SetAgentPerm.mockResolvedValue(info('ask'))
    await open(info('allow'))
    await fireEvent.click(inRow('Всегда'))
    expect(api.SetAgentPerm).not.toHaveBeenCalled()
    await fireEvent.click(inRow('Спрашивать'))
    expect(api.SetAgentPerm).toHaveBeenLastCalledWith('open_album', 'ask')
    await fireEvent.click(inRow('Запретить'))
    expect(api.SetAgentPerm).toHaveBeenLastCalledWith('open_album', 'deny')
    expect(screen.queryByText(TITLE)).toBeNull()
  })

  it('строка мест в «Правках» — при «Всегда» у открытия (как у папки), при «Спрашивать» её нет', async () => {
    const places = [place('D:\\Фото')]
    await open(info('ask', { places }))
    expect(document.querySelector('[data-row="places"]')).toBeNull()
    cleanup()
    agents.close()
    await open(info('allow', { places }))
    expect(document.querySelector('[data-row="places"]')?.textContent).toContain('D:\\Фото')
  })

  it('групповой щелчок «Правки ▸ Запретить» строку открытия не трогает и её согласия не просит', async () => {
    api.SetAgentPermGroup.mockResolvedValue(info('ask'))
    await open(info('ask'))
    await fireEvent.click(inEdit('Запретить'))
    expect(api.SetAgentPermGroup).toHaveBeenCalledExactlyOnceWith('edit', 'deny')
    expect(screen.queryByText(TITLE)).toBeNull()
    expect(pressed(inRow('Спрашивать'))).toBe(true)
  })

  it('групповой «Всегда» у «Правок» при открытии в «Спрашивать»: согласие только папок, не открытия', async () => {
    await open(info('ask'))
    await fireEvent.click(inEdit('Всегда'))
    expect(screen.getByText('Добавлять папки по просьбе агента без вопроса?')).toBeTruthy()
    expect(screen.queryByText(TITLE)).toBeNull()
  })
})
