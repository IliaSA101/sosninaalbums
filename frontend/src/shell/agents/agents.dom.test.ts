// @vitest-environment happy-dom
import { tick } from 'svelte'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte'

// Окно «ИИ-агенты (MCP)» (143, У1) на подменённых биндингах wailsjs (AgentsInfo, SetAgents…): вопрос включения, флажки уровней, подсказка hint,
// тексты настройки, кнопка файла только при setup.file, подключённые и журнал из событий. Решения — в Go, здесь только вызовы.
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
  SetAgentsEdit: vi.fn(),
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
const noBtn = (name: string | RegExp) => screen.queryByRole('button', { name })

const spies = {
  info: api.AgentsInfo,
  set: api.SetAgents,
  setLevel: api.SetAgentsLevel,
  clearLog: api.ClearAgentsLog,
  saveExtension: api.SaveClaudeExtension,
  setEdit: api.SetAgentsEdit,
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

describe('включение доступа', () => {
  it('выключено: строка состояния и «Разрешить…», совета настройки нет', async () => {
    await open()
    expect(screen.getByText('Агенты ИИ не видят программу.')).toBeTruthy()
    expect(btn('Разрешить агентам читать альбом…')).toBeTruthy()
    expect(screen.queryByText('Как подключить')).toBeNull()
    expect(noBtn('Копировать')).toBeNull()
  })

  it('«Разрешить…» → вопрос; «Отмена» ничего не зовёт', async () => {
    await open()
    await fireEvent.click(btn('Разрешить агентам читать альбом…'))
    expect(screen.getByText('Разрешить агентам ИИ читать альбом?')).toBeTruthy()
    expect(screen.getByText(/Программа не сообщает агенту пути к папкам/)).toBeTruthy()
    expect(screen.getByText(/не защищают от агента, у которого есть свой доступ к файлам/)).toBeTruthy()
    await fireEvent.click(btn('Отмена'))
    expect(screen.queryByText('Разрешить агентам ИИ читать альбом?')).toBeNull()
    expect(spies.set).not.toHaveBeenCalled()
    expect(spies.setLevel).not.toHaveBeenCalled()
  })

  it('«Отмена» слева, «Разрешить» справа', async () => {
    await open()
    await fireEvent.click(btn('Разрешить агентам читать альбом…'))
    const ask = btn('Разрешить').parentElement!
    expect([...ask.querySelectorAll('button')].map((b) => b.textContent)).toEqual(['Отмена', 'Разрешить'])
  })

  it('«Разрешить» → SetAgents(true), окно показывает «Включено»', async () => {
    spies.set.mockResolvedValue(infoOn())
    await open()
    await fireEvent.click(btn('Разрешить агентам читать альбом…'))
    await fireEvent.click(btn('Разрешить'))
    expect(spies.set).toHaveBeenCalledExactlyOnceWith(true)
    await waitFor(() => expect(screen.getByText('Включено')).toBeTruthy())
    expect(noBtn('Разрешить агентам читать альбом…')).toBeNull()
  })

  it('Go ответил ошибкой статуса — слова ошибки, кнопка «Разрешить…» остаётся', async () => {
    spies.set.mockResolvedValue(normInfo({ status: 'error', error: 'Не найдена папка данных' }))
    await open()
    await fireEvent.click(btn('Разрешить агентам читать альбом…'))
    await fireEvent.click(btn('Разрешить'))
    await waitFor(() => expect(screen.getByRole('alert').textContent).toBe('Не найдена папка данных'))
    expect(btn('Разрешить агентам читать альбом…')).toBeTruthy()
    expect(btn('Выключить')).toBeTruthy() // ошибка запуска: включение можно снять
  })

  it('ошибка запуска: «Выключить» зовёт set(false), Go отвечает «выключено» — ссылка и ошибка уходят', async () => {
    spies.set.mockResolvedValue(emptyInfo())
    await open(normInfo({ status: 'error', error: 'Не найдена папка данных' }))
    await fireEvent.click(btn('Выключить'))
    expect(spies.set).toHaveBeenCalledWith(false)
    await waitFor(() => expect(noBtn('Выключить')).toBeNull())
    expect(screen.queryByRole('alert')).toBeNull()
  })

  it('«Выключить» — обычная кнопка окна, не ссылка (контраст на тёмной теме)', async () => {
    await open(infoOn())
    expect(btn('Выключить').classList.contains('link')).toBe(false)
    cleanup()
    agents.close()
    await open(normInfo({ status: 'error', error: 'Не найдена папка данных' }))
    expect(btn('Выключить').classList.contains('link')).toBe(false)
  })

  it('выключено без ошибки — «Выключить» нет', async () => {
    await open()
    expect(noBtn('Выключить')).toBeNull()
  })

  it('SetAgents бросил — тост с ошибкой, состояние перечитано', async () => {
    spies.set.mockRejectedValue(new Error('нет сокета'))
    await open()
    await fireEvent.click(btn('Разрешить агентам читать альбом…'))
    const before = spies.info.mock.calls.length
    await fireEvent.click(btn('Разрешить'))
    await waitFor(() => expect(notify).toHaveBeenCalledWith('Нет сокета', 'error'))
    await waitFor(() => expect(spies.info.mock.calls.length).toBeGreaterThan(before))
  })

  it('«Выключить» → SetAgents(false)', async () => {
    spies.set.mockResolvedValue(emptyInfo())
    await open(infoOn())
    await fireEvent.click(btn('Выключить'))
    expect(spies.set).toHaveBeenCalledExactlyOnceWith(false)
    await waitFor(() => expect(screen.getByText('Агенты ИИ не видят программу.')).toBeTruthy())
  })
})

describe('флажки уровней', () => {
  // Подпись флажка начинается с «+ »; заголовок вопроса («Показать агенту имена…») под маску не подходит.
  const box = (label: string) => screen.getByLabelText(new RegExp(`^\\+ ${label}`)) as HTMLInputElement

  it('уровень 1: оба сняты; вид флажка не меняется, пока Go не ответил', async () => {
    await open(infoOn())
    expect(box('имена файлов и тексты').checked).toBe(false)
    expect(box('картинки').checked).toBe(false)
    await fireEvent.click(box('имена файлов и тексты'))
    expect(box('имена файлов и тексты').checked).toBe(false)
    expect(spies.setLevel).not.toHaveBeenCalled()
  })

  it('имена: вопрос → «Показать» → SetAgentsLevel(2); «Отмена» — ничего', async () => {
    spies.setLevel.mockResolvedValue(infoOn({ level: 2 }))
    await open(infoOn())
    await fireEvent.click(box('имена файлов и тексты'))
    expect(screen.getByText('Показать агенту имена файлов и тексты?')).toBeTruthy()
    expect(screen.getByText(/ФИО детей/)).toBeTruthy()
    await fireEvent.click(btn('Отмена'))
    expect(spies.setLevel).not.toHaveBeenCalled()
    await fireEvent.click(box('имена файлов и тексты'))
    await fireEvent.click(btn('Показать'))
    expect(spies.setLevel).toHaveBeenCalledExactlyOnceWith(2)
    await waitFor(() => expect(box('имена файлов и тексты').checked).toBe(true))
    expect(box('картинки').checked).toBe(false)
  })

  it('картинки: вопрос → SetAgentsLevel(3), включает и первый флажок', async () => {
    spies.setLevel.mockResolvedValue(infoOn({ level: 3 }))
    await open(infoOn())
    await fireEvent.click(box('картинки'))
    expect(screen.getByText('Показать агенту картинки?')).toBeTruthy()
    await fireEvent.click(btn('Показать'))
    expect(spies.setLevel).toHaveBeenCalledExactlyOnceWith(3)
    await waitFor(() => expect(box('картинки').checked).toBe(true))
    expect(box('имена файлов и тексты').checked).toBe(true)
  })

  it('снятие без вопроса: имён на уровне 3 — уровень 1 (снимает и картинки), картинок — уровень 2', async () => {
    spies.setLevel.mockResolvedValue(infoOn({ level: 1 }))
    await open(infoOn({ level: 3 }))
    await fireEvent.click(box('имена файлов и тексты'))
    expect(spies.setLevel).toHaveBeenLastCalledWith(1)
    await waitFor(() => expect(box('картинки').checked).toBe(false))
    expect(screen.queryByText('Показать агенту имена файлов и тексты?')).toBeNull()

    spies.setLevel.mockResolvedValue(infoOn({ level: 2 }))
    agents.info = infoOn({ level: 3 })
    await waitFor(() => expect(box('картинки').checked).toBe(true))
    await fireEvent.click(box('картинки'))
    expect(spies.setLevel).toHaveBeenLastCalledWith(2)
    await waitFor(() => expect(box('картинки').checked).toBe(false))
    expect(box('имена файлов и тексты').checked).toBe(true)
  })

  it('флажков нет, пока доступ выключен', async () => {
    await open()
    expect(screen.queryByLabelText(/^\+ картинки/)).toBeNull()
  })
})

describe('как подключить', () => {
  it('настройка клиентов свёрнута (журнал виден без прокрутки); hint разворачивает её', async () => {
    await open(infoOn())
    const det = document.querySelector<HTMLDetailsElement>('details.setup')!
    expect(det.open).toBe(false)
    expect(det.querySelector('summary')!.textContent).toBe('Как подключить')
    await fireEvent.click(det.querySelector('summary')!)
    await waitFor(() => expect(det.open).toBe(true))
    // События Go (подключение, вызов) не захлопывают раскрытый пользователем раздел
    ev.on.get('agents:state')!(infoOn({ clients: [{ name: 'Claude Desktop', since: '2026-10-29T10:00:00Z' }] }))
    ev.on.get('agents:call')!({ client: 'Claude Desktop', tool: 'get_album', ok: true, at: '2026-10-29T10:00:01Z', ms: 3, today: 1 })
    await tick()
    expect(det.open).toBe(true)
    cleanup()
    agents.close()
    await open(infoOn({ hint: 'Путь к программе устарел' }))
    expect(document.querySelector<HTMLDetailsElement>('details.setup')!.open).toBe(true)
  })

  it('строка hint — над настройками; пустая — строки нет', async () => {
    await open(infoOn({ hint: 'Программа запущена из временной папки — путь в настройке устареет' }))
    const hint = screen.getByRole('note')
    expect(hint.textContent).toBe('Программа запущена из временной папки — путь в настройке устареет')
    const head = screen.getByText('Как подключить')
    expect(head.compareDocumentPosition(hint) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    cleanup()
    agents.close()
    await open(infoOn({ hint: '' }))
    expect(screen.queryByRole('note')).toBeNull()
  })

  it('пустые setups при мёртвом пути exe: заголовок и hint есть, настройки и «Копировать» нет, окно живёт', async () => {
    await open(infoOn({ setups: [], hint: 'Путь к программе устарел' }))
    expect(screen.getByRole('note').textContent).toBe('Путь к программе устарел')
    expect(noBtn('Копировать')).toBeNull()
    expect(screen.queryByRole('list', { name: 'Как подключить' })).toBeNull()
    expect(screen.getByText('Включено')).toBeTruthy()
  })

  it('null вместо списков в ответе Go не ломает окно', async () => {
    await open(normInfo({ status: 'on', setups: null as never, clients: null as never, calls: null as never }))
    expect(screen.getByText('Сейчас никто не подключён.')).toBeTruthy()
    expect(screen.getByText('Пока агенты ничего не спрашивали.')).toBeTruthy()
  })

  it('«Копировать» кладёт в буфер текст из Go и отвечает тостом', async () => {
    await open(infoOn({ setups: [setup('a', { text: '{"mcpServers":{}}' })] }))
    await fireEvent.click(btn('Копировать'))
    expect(writeText).toHaveBeenCalledExactlyOnceWith('{"mcpServers":{}}')
    expect(notify).toHaveBeenCalledWith('Настройка скопирована', 'info')
  })

  it('отказ буфера — свой текст ошибки', async () => {
    writeText.mockRejectedValue(new Error('NotAllowedError'))
    await open(infoOn())
    await fireEvent.click(btn('Копировать'))
    await waitFor(() => expect(notify).toHaveBeenCalledWith('Не удалось скопировать в буфер обмена', 'error'))
  })

  it('текст настройки и заметка показываются как текст, не как разметка', async () => {
    await open(infoOn({ setups: [setup('a', { text: '<b>x</b>', note: 'Файл: <i>y</i>' })] }))
    expect(document.querySelector('.code')!.textContent).toBe('<b>x</b>')
    expect(document.querySelector('.code b')).toBeNull()
    expect(document.querySelector('.head i')).toBeNull()
  })

  it('«Сохранить файл для Claude Desktop…» — только у строки с file', async () => {
    await open(infoOn({ setups: [setup('a', { file: true }), setup('b'), setup('c')] }))
    expect(screen.getAllByRole('button', { name: 'Сохранить файл для Claude Desktop…' })).toHaveLength(1)
    expect(screen.getAllByRole('button', { name: 'Копировать' })).toHaveLength(3)
    cleanup()
    agents.close()
    await open(infoOn({ setups: [setup('b'), setup('c')] }))
    expect(noBtn('Сохранить файл для Claude Desktop…')).toBeNull()
  })

  it('сохранение: тост из text; отказ от диалога (canceled) — тишина; ошибка — тост-ошибка', async () => {
    await open(infoOn({ setups: [setup('a', { file: true })] }))
    spies.saveExtension.mockResolvedValue({ canceled: false, text: 'Файл сохранён — он для этого компьютера' })
    await fireEvent.click(btn('Сохранить файл для Claude Desktop…'))
    await waitFor(() => expect(notify).toHaveBeenCalledWith('Файл сохранён — он для этого компьютера', 'info', undefined, 9000))
    notify.mockReset()
    spies.saveExtension.mockResolvedValue({ canceled: true, text: '' })
    await fireEvent.click(btn('Сохранить файл для Claude Desktop…'))
    await waitFor(() => expect(spies.saveExtension).toHaveBeenCalledTimes(2))
    expect(notify).not.toHaveBeenCalled()
    spies.saveExtension.mockRejectedValue(new Error('только Windows и Mac'))
    await fireEvent.click(btn('Сохранить файл для Claude Desktop…'))
    await waitFor(() => expect(notify).toHaveBeenCalledWith('Только Windows и Mac', 'error'))
  })

  it('переносная версия — строка про обновление; обычная — нет', async () => {
    await open(infoOn({ portable: true }))
    expect(screen.getByText(/после обновления сохраните файл или скопируйте настройку заново/)).toBeTruthy()
    cleanup()
    agents.close()
    await open(infoOn({ portable: false }))
    expect(screen.queryByText(/после обновления сохраните файл/)).toBeNull()
  })
})

describe('подключённые и журнал', () => {
  const at = new Date(2026, 9, 28, 12, 3).getTime()
  const call = (o: object = {}) => ({ at, client: 'Claude Desktop', tool: 'Сводка альбома', ok: true, ms: 4, ...o })

  it('agents:state → «Подключены», agents:call → строка журнала и счётчик', async () => {
    await open(infoOn())
    expect(screen.getByText('Сейчас никто не подключён.')).toBeTruthy()
    ev.on.get('agents:state')!(infoOn({ clients: [{ name: 'Claude Desktop', since: at }] }))
    await waitFor(() => expect(screen.getByText('Подключены: Claude Desktop')).toBeTruthy())
    ev.on.get('agents:call')!({ ...call(), today: 7 })
    await waitFor(() => expect(screen.getByText('12:03 · Claude Desktop · Сводка альбома · 4 мс')).toBeTruthy())
    expect(screen.getByText('Сегодня запросов: 7')).toBeTruthy()
  })

  it('новые вызовы — сверху; неудачный помечен; два окна одного клиента — одна строка', async () => {
    await open(infoOn({ clients: [1, 2].map((since) => ({ name: 'Claude Code', since })) }))
    expect(screen.getByText('Подключены: Claude Code')).toBeTruthy()
    ev.on.get('agents:call')!({ ...call({ tool: 'Первый' }), today: 1 })
    ev.on.get('agents:call')!({ ...call({ tool: 'Второй', ok: false }), today: 2 })
    await waitFor(() => expect(document.querySelectorAll('.log li')).toHaveLength(2))
    const rows = [...document.querySelectorAll('.log li')].map((li) => li.textContent)
    expect(rows[0]).toBe('12:03 · Claude Desktop · Второй · 4 мс · ошибка')
    expect(rows[1]).toBe('12:03 · Claude Desktop · Первый · 4 мс')
    expect(document.querySelector('.log li.bad')).not.toBeNull()
  })

  it('имя клиента, инструмент и журнал с HTML выводятся текстом, а не разметкой', async () => {
    const evil = '<img src=x onerror=alert(1)>Клиент'
    await open(infoOn({ clients: [{ name: evil, since: at }], calls: [call({ client: evil, tool: '<b>tool</b>' })] }))
    expect(screen.getByText(`Подключены: ${evil}`)).toBeTruthy()
    expect(screen.getByText(`12:03 · ${evil} · <b>tool</b> · 4 мс`)).toBeTruthy()
    expect(document.querySelector('img')).toBeNull()
    expect(document.querySelector('.log b')).toBeNull()
    ev.on.get('agents:call')!({ ...call({ client: '<i>x</i>' }), today: 1 })
    await waitFor(() => expect(screen.getByText(/<i>x<\/i>/)).toBeTruthy())
    expect(document.querySelector('.log i')).toBeNull()
  })

  it('agents:state целиком заменяет вид (вызов приходит после — ложится сверху, счётчик берётся из события)', async () => {
    await open(infoOn({ calls: [call({ tool: 'Старый' })], today: 3 }))
    ev.on.get('agents:state')!(infoOn({ calls: [call({ tool: 'Из state' })], today: 5 }))
    await waitFor(() => expect(screen.getByText('Сегодня запросов: 5')).toBeTruthy())
    expect(document.querySelectorAll('.log li')).toHaveLength(1)
    ev.on.get('agents:call')!({ ...call({ tool: 'Новый' }), today: 6 })
    await waitFor(() => expect(document.querySelectorAll('.log li')).toHaveLength(2))
    expect(document.querySelector('.log li')!.textContent).toContain('Новый')
    expect(screen.getByText('Сегодня запросов: 6')).toBeTruthy()
  })

  it('журнал не длиннее 50: 51-й вызов вытесняет самый старый', async () => {
    await open(infoOn())
    for (let i = 0; i < 51; i++) ev.on.get('agents:call')!({ ...call({ tool: `T${i}` }), today: i })
    await waitFor(() => expect(document.querySelectorAll('.log li')).toHaveLength(50))
    expect(document.querySelector('.log li')!.textContent).toContain('T50')
    expect(screen.queryByText(/ T0 /)).toBeNull()
  })

  it('«Очистить журнал» → ClearAgentsLog; пустой журнал — кнопка серая', async () => {
    spies.clearLog.mockResolvedValue(infoOn())
    await open(infoOn({ calls: [call()] }))
    await fireEvent.click(btn('Очистить журнал'))
    expect(spies.clearLog).toHaveBeenCalledTimes(1)
    await waitFor(() => expect(screen.getByText('Пока агенты ничего не спрашивали.')).toBeTruthy())
    expect((btn('Очистить журнал') as HTMLButtonElement).disabled).toBe(true)
  })
})

describe('правки агентов (149)', () => {
  const edit = () => screen.getByLabelText('Разрешить агентам менять альбом') as HTMLInputElement
  const call = (o: object = {}) => ({ at: Date.now(), client: 'Claude Code', tool: 'Сводка альбома', ok: true, ms: 4, ...o })

  it('блок «Правки» виден при включённом доступе, флажок снят; при выключенном доступе блока нет', async () => {
    await open(infoOn())
    expect(screen.getByRole('heading', { name: 'Правки' })).toBeTruthy()
    expect(edit().checked).toBe(false)
    expect(edit().disabled).toBe(false)
    cleanup()
    agents.close()
    await open()
    expect(screen.queryByLabelText('Разрешить агентам менять альбом')).toBeNull()
  })

  it('флажок не зависит от уровня чтения: на уровнях 1, 2 и 3 он такой же', async () => {
    for (const level of [1, 2, 3]) {
      await open(infoOn({ level }))
      expect(edit().disabled).toBe(false)
      cleanup()
      agents.close()
    }
  })

  it('щелчок → вопрос по тексту §10, вид флажка прежний, SetAgentsEdit не зовут', async () => {
    // Клавиша отмены в тексте — из реестра: у владельца (Windows, Linux) это дословно «Ctrl+Z»
    setMacForTest(false)
    keymap.apply({ commands: [{ id: 'undo', keys: ['Mod+KeyZ'] }] } as never)
    await open(infoOn())
    await fireEvent.click(edit())
    expect(edit().checked).toBe(false)
    expect(screen.getByText('Разрешить агентам ИИ менять альбом?')).toBeTruthy()
    expect(
      screen.getByText(
        'Агент сможет создавать альбомы, собирать развороты и менять раскладки. Каждая правка — один шаг Ctrl+Z; перед первой правкой — резервная копия. Агент может предложить папку — программа откроет выбор папки, решаете вы.',
      ),
    ).toBeTruthy()
    expect(spies.setEdit).not.toHaveBeenCalled()
    setMacForTest(null)
  })

  it('вопрос: «Отмена» слева, «Разрешить» справа; «Отмена» ничего не зовёт', async () => {
    await open(infoOn())
    await fireEvent.click(edit())
    const ask = btn('Разрешить').parentElement!
    expect([...ask.querySelectorAll('button')].map((b) => b.textContent)).toEqual(['Отмена', 'Разрешить'])
    await fireEvent.click(btn('Отмена'))
    expect(screen.queryByText('Разрешить агентам ИИ менять альбом?')).toBeNull()
    expect(spies.setEdit).not.toHaveBeenCalled()
  })

  it('«Разрешить» → SetAgentsEdit(true), флажок включён по ответу Go', async () => {
    spies.setEdit.mockResolvedValue(infoOn({ edit: true }))
    await open(infoOn())
    await fireEvent.click(edit())
    await fireEvent.click(btn('Разрешить'))
    expect(spies.setEdit).toHaveBeenCalledExactlyOnceWith(true)
    await waitFor(() => expect(edit().checked).toBe(true))
  })

  it('снятие — сразу, без вопроса: SetAgentsEdit(false)', async () => {
    spies.setEdit.mockResolvedValue(infoOn({ edit: false }))
    await open(infoOn({ edit: true }))
    expect(edit().checked).toBe(true)
    await fireEvent.click(edit())
    expect(screen.queryByText('Разрешить агентам ИИ менять альбом?')).toBeNull()
    expect(spies.setEdit).toHaveBeenCalledExactlyOnceWith(false)
    await waitFor(() => expect(edit().checked).toBe(false))
  })

  it('Go не включил (ошибка) — тост, флажок остаётся снятым', async () => {
    spies.setEdit.mockRejectedValue(new Error('не удалось записать настройку'))
    await open(infoOn())
    await fireEvent.click(edit())
    await fireEvent.click(btn('Разрешить'))
    await waitFor(() => expect(notify).toHaveBeenCalledWith('Не удалось записать настройку', 'error'))
    expect(edit().checked).toBe(false)
  })

  it('от администратора: флажок серый (disabled), рядом причина и что делать; щелчок ничего не открывает', async () => {
    await open(infoOn({ editBlocked: 'elevated' }))
    expect(edit().disabled).toBe(true)
    expect(screen.getByText('Программа запущена от администратора — запустите её обычным способом')).toBeTruthy()
    await fireEvent.click(edit())
    expect(screen.queryByText('Разрешить агентам ИИ менять альбом?')).toBeNull()
    expect(spies.setEdit).not.toHaveBeenCalled()
  })

  it('не от администратора — подписи причины нет', async () => {
    await open(infoOn())
    expect(screen.queryByText(/запущена от администратора/)).toBeNull()
  })

  it('«Разрешить вопросы» только при asksMuted: AllowAgentAsks, затем состояние перечитано', async () => {
    await open(infoOn())
    expect(noBtn('Разрешить вопросы')).toBeNull()
    expect(screen.queryByText('Вопросы агентов отключены после трёх отказов')).toBeNull()
    cleanup()
    agents.close()
    spies.allowAsks.mockResolvedValue(undefined)
    await open(infoOn({ asksMuted: true }))
    expect(screen.getByText('Вопросы агентов отключены после трёх отказов')).toBeTruthy()
    spies.info.mockResolvedValue(infoOn({ asksMuted: false }))
    await fireEvent.click(btn('Разрешить вопросы'))
    expect(spies.allowAsks).toHaveBeenCalledTimes(1)
    await waitFor(() => expect(noBtn('Разрешить вопросы')).toBeNull())
  })

  it('AllowAgentAsks бросил — тост с ошибкой, кнопка остаётся', async () => {
    spies.allowAsks.mockRejectedValue(new Error('ещё не готово'))
    await open(infoOn({ asksMuted: true }))
    await fireEvent.click(btn('Разрешить вопросы'))
    await waitFor(() => expect(notify).toHaveBeenCalledWith('Ещё не готово', 'error'))
    expect(btn('Разрешить вопросы')).toBeTruthy()
  })

  it('журнал: правка помечена ✎, итог словами Go вместо «ошибка»; чтение без значка', async () => {
    await open(
      infoOn({
        calls: [
          call({ edit: true, tool: 'Создать альбом', ok: false, note: 'отказано' }),
          call({ edit: true, tool: 'Собрать', note: '' }),
          call({ tool: 'Сводка альбома' }),
        ],
      }),
    )
    const lines = screen.getAllByRole('listitem').map((l) => l.textContent)
    expect(lines.some((l) => l?.includes('✎ Создать альбом') && l.endsWith('· отказано') && !l.includes('ошибка'))).toBe(true)
    expect(lines.some((l) => l?.includes('✎ Собрать') && !l.includes('ошибка'))).toBe(true)
    expect(lines.some((l) => l?.includes('Сводка альбома') && !l.includes('✎'))).toBe(true)
  })
})

describe('закрытие', () => {
  it('«Закрыть» убирает окно', async () => {
    await open()
    // Кнопка внизу окна (у крестика в шапке то же имя): та, что с автофокусом.
    const bottom = screen.getAllByRole('button', { name: 'Закрыть' }).find((b) => b.hasAttribute('data-autofocus'))!
    await fireEvent.click(bottom)
    await waitFor(() => expect(agents.open).toBe(false))
    expect(screen.queryByRole('button', { name: 'Закрыть' })).toBeNull()
  })

  it('вопрос закрывается вместе с окном', async () => {
    await open()
    await fireEvent.click(btn('Разрешить агентам читать альбом…'))
    agents.close()
    await waitFor(() => expect(screen.queryByText('Разрешить агентам ИИ читать альбом?')).toBeNull())
    expect(agents.open).toBe(false)
  })
})
