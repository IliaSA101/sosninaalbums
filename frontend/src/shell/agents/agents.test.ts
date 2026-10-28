import { describe, expect, it } from 'vitest'
import {
  CALLS_MAX,
  callLine,
  clientNames,
  hasNames,
  hasPics,
  levelAfterNames,
  levelAfterPics,
  normInfo,
  pushCall,
  bridge,
  STUB_ERROR,
  type AgentCall,
} from './agents'

// Чистые подписи окна «ИИ-агенты (MCP)» (143, У1).
const call = (o: Partial<AgentCall> = {}): AgentCall => ({
  at: new Date(2026, 9, 28, 9, 5).getTime(),
  client: 'Claude Desktop',
  tool: 'Сводка альбома',
  ok: true,
  ms: 4,
  ...o,
})

describe('строка журнала', () => {
  it('«12:03 · клиент · инструмент · N мс», время с нулём', () => {
    expect(callLine(call())).toBe('09:05 · Claude Desktop · Сводка альбома · 4 мс')
  })
  it('неудача — «· ошибка» в конце', () => {
    expect(callLine(call({ ok: false, ms: 0 }))).toBe('09:05 · Claude Desktop · Сводка альбома · 0 мс · ошибка')
  })
  it('чужой текст остаётся текстом', () => {
    expect(callLine(call({ client: '<b>x</b>' }))).toContain('<b>x</b>')
  })
})

describe('журнал и подключённые', () => {
  it('новый вызов — наверх, журнал не длиннее предела', () => {
    let calls: AgentCall[] = []
    for (let i = 0; i < CALLS_MAX + 5; i++) calls = pushCall(calls, call({ ms: i }))
    expect(calls).toHaveLength(CALLS_MAX)
    expect(calls[0].ms).toBe(CALLS_MAX + 4)
  })
  it('имена подключённых без повторов, в порядке прихода', () => {
    const c = (name: string) => ({ name, since: 1 })
    expect(clientNames([c('Claude Code'), c('OpenCode'), c('Claude Code')])).toEqual(['Claude Code', 'OpenCode'])
    expect(clientNames([])).toEqual([])
  })
})

describe('ответ Go', () => {
  it('null вместо списков и уровень вне 1…3 приводятся к форме', () => {
    const i = normInfo({ status: 'on', level: 9, setups: null as never, clients: null as never, calls: null as never })
    expect(i).toMatchObject({ status: 'on', level: 3, setups: [], clients: [], calls: [], hint: '' })
    expect(normInfo({ level: 0 }).level).toBe(1)
    expect(normInfo(null).status).toBe('off')
  })
})

describe('флажки уровней', () => {
  it('имена с уровня 2, картинки с 3', () => {
    expect([1, 2, 3].map(hasNames)).toEqual([false, true, true])
    expect([1, 2, 3].map(hasPics)).toEqual([false, false, true])
  })
  it('уровень после щелчка: имена 1/2, картинки 2/3', () => {
    expect([levelAfterNames(false), levelAfterNames(true)]).toEqual([1, 2])
    expect([levelAfterPics(false), levelAfterPics(true)]).toEqual([2, 3])
  })
})

describe('заглушка до Г5', () => {
  it('включить нельзя: set(true) отвечает ошибкой, set(false) снимает её', async () => {
    const r = await bridge.set(true)
    expect(r).toMatchObject({ status: 'error', error: 'Доступ агентов появится в следующей сборке.' })
    expect(STUB_ERROR).toBe(r.error)
    expect((await bridge.set(false)).status).toBe('off')
    expect((await bridge.info()).error).toBe('')
  })
})

describe('status из Wails', () => {
  it('строка вне on/error сужается до off', () => {
    expect(normInfo({ status: 'ой' }).status).toBe('off')
    expect(normInfo({ status: 'error' }).status).toBe('error')
  })
})
