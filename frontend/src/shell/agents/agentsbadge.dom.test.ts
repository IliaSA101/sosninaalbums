// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte'

// Значок «ИИ» в верхней строке (143, У1): виден только при включённом доступе, точка — пока кто-то подключён,
// щелчок открывает окно. Состояние приходит из Go (`AgentsInfo`, `agents:state`).
// Подписки считаем списком (а не по имени): двойная подписка на то же событие не должна прятаться.
const ev = vi.hoisted(() => {
  const subs: { name: string; cb: (d: unknown) => void }[] = []
  return {
    subs,
    on: {
      get: (name: string) => subs.find((s) => s.name === name)?.cb,
      has: (name: string) => subs.some((s) => s.name === name),
    },
    emit: (name: string, d: unknown) => subs.filter((s) => s.name === name).forEach((s) => s.cb(d)),
  }
})
vi.mock('../../../wailsjs/runtime/runtime', () => ({
  EventsOn: (name: string, cb: (d: unknown) => void) => {
    const sub = { name, cb }
    ev.subs.push(sub)
    return () => {
      const i = ev.subs.indexOf(sub)
      if (i >= 0) ev.subs.splice(i, 1)
    }
  },
}))
const api = vi.hoisted(() => ({ AgentsInfo: vi.fn() }))
vi.mock('../../../wailsjs/go/app/App', () => api)

const { default: AgentsBadge } = await import('./AgentsBadge.svelte')
const { default: AgentsDialog } = await import('./AgentsDialog.svelte')
const { agents } = await import('./agents.svelte')
const { emptyInfo, normInfo } = await import('./agents')

const info = api.AgentsInfo
const badge = () => screen.queryByRole('button', { name: /^ИИ/ })
const on = (o: object = {}) => normInfo({ status: 'on', ...o })

beforeEach(() => {
  info.mockReset()
  ev.subs.length = 0
  agents.close()
  agents.info = emptyInfo()
})
afterEach(() => {
  cleanup()
  agents.close()
})

describe('значок «ИИ»', () => {
  it('выключено — значка нет', async () => {
    info.mockResolvedValue(emptyInfo())
    render(AgentsBadge)
    await waitFor(() => expect(info).toHaveBeenCalled())
    expect(badge()).toBeNull()
  })

  it('ошибка запуска (status error) — значка нет', async () => {
    info.mockResolvedValue(normInfo({ status: 'error', error: 'нет папки' }))
    render(AgentsBadge)
    await waitFor(() => expect(info).toHaveBeenCalled())
    expect(badge()).toBeNull()
  })

  it('включено — значок с подсказкой, без точки', async () => {
    info.mockResolvedValue(on())
    render(AgentsBadge)
    await waitFor(() => expect(badge()).not.toBeNull())
    expect(badge()!.getAttribute('title')).toBe('Доступ ИИ-агентов к альбому включён — открыть окно')
    expect(document.querySelector('.dot')).toBeNull()
  })

  it('точка и имена в подсказке — пока есть подключение; пропала — исчезла', async () => {
    info.mockResolvedValue(on())
    render(AgentsBadge)
    await waitFor(() => expect(badge()).not.toBeNull())
    ev.on.get('agents:state')!(on({ clients: [{ name: 'Claude Desktop', since: 1 }] }))
    await waitFor(() => expect(document.querySelector('.dot')).not.toBeNull())
    expect(badge()!.getAttribute('title')).toBe('Агент подключён: Claude Desktop — открыть окно доступа')
    ev.on.get('agents:state')!(on())
    await waitFor(() => expect(document.querySelector('.dot')).toBeNull())
  })

  it('agents:state «выключено» убирает значок, «включено» — возвращает', async () => {
    info.mockResolvedValue(on())
    render(AgentsBadge)
    await waitFor(() => expect(badge()).not.toBeNull())
    ev.on.get('agents:state')!(emptyInfo())
    await waitFor(() => expect(badge()).toBeNull())
    ev.on.get('agents:state')!(on())
    await waitFor(() => expect(badge()).not.toBeNull())
  })

  it('щелчок открывает окно и перечитывает состояние', async () => {
    info.mockResolvedValue(on())
    render(AgentsBadge)
    await waitFor(() => expect(badge()).not.toBeNull())
    expect(agents.open).toBe(false)
    await fireEvent.click(badge()!)
    expect(agents.open).toBe(true)
    await waitFor(() => expect(info).toHaveBeenCalledTimes(2))
  })

  it('подписка снимается, когда значок убран', async () => {
    info.mockResolvedValue(on())
    render(AgentsBadge)
    await waitFor(() => expect(ev.on.has('agents:state')).toBe(true))
    cleanup()
    expect(ev.subs).toHaveLength(0)
  })

  it('окно и значок вместе: одна пара подписок; убрали одного — события идут; убрали обоих — подписок 0', async () => {
    info.mockResolvedValue(on())
    const badgeView = render(AgentsBadge)
    await waitFor(() => expect(badge()).not.toBeNull())
    const dialogView = render(AgentsDialog)
    expect(ev.subs.map((s) => s.name).sort()).toEqual(['agents:call', 'agents:state']) // не по две
    badgeView.unmount()
    expect(ev.subs).toHaveLength(2)
    ev.emit('agents:call', { at: 1, client: 'Claude Code', tool: 'Сводка', ok: true, ms: 3, today: 7 })
    expect(agents.info.today).toBe(7)
    expect(agents.info.calls).toHaveLength(1)
    dialogView.unmount()
    expect(ev.subs).toHaveLength(0)
  })

  it('снятие подписки повторно безопасно и не уводит счётчик ниже нуля', async () => {
    info.mockResolvedValue(on())
    const off1 = agents.watch()
    const off2 = agents.watch()
    expect(ev.subs).toHaveLength(2)
    off1()
    off1() // второй вызов того же снятия не должен убрать чужую подписку
    expect(ev.subs).toHaveLength(2)
    off2()
    expect(ev.subs).toHaveLength(0)
    off2()
    const off3 = agents.watch() // счётчик цел: новая подписка снова подписывает
    expect(ev.subs).toHaveLength(2)
    off3()
    expect(ev.subs).toHaveLength(0)
  })
})
