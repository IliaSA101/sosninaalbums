import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { KeymapSetResult, KeymapView } from '../../base/types/keys'
import { ownsKeys, type RecEvent } from './keyrec'

// Go подменён: каждый метод — заглушка, ответ задаёт тест.
const api = vi.hoisted(() => ({
  Keymap: vi.fn(),
  SetHotkey: vi.fn(),
  RemoveHotkey: vi.fn(),
  ResetHotkey: vi.fn(),
  ResetHotkeys: vi.fn(),
  RestoreHotkeys: vi.fn(),
}))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => api)
vi.mock('../../editor/text/fonts', () => ({ loadFonts: async () => {} }))
vi.stubGlobal('window', globalThis)

const { keysUi } = await import('./keysui.svelte')
const { keymap } = await import('../../base/keys/keymap.svelte')
const { catalogView, catalogWith, dropCatalog, useCatalog } = await import('../../base/keys/keymap.fixture')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { store } = await import('../../base/core/store.svelte')

const ev = (key: string, code: string, o: Partial<RecEvent> = {}): RecEvent => ({
  key,
  code,
  ctrlKey: false,
  shiftKey: false,
  altKey: false,
  metaKey: false,
  repeat: false,
  isComposing: false,
  ...o,
})
const ok = (view: KeymapView, extra: Partial<KeymapSetResult> = {}): KeymapSetResult => ({
  view,
  conflict: null,
  replaced: null,
  shadows: [],
  ...extra,
})
const flush = () => new Promise((r) => setTimeout(r, 0))

beforeEach(() => {
  vi.clearAllMocks()
  api.Keymap.mockResolvedValue(catalogView)
  prefs.level = 'full'
  useCatalog()
  keysUi.close()
  keysUi.show()
})

describe('открытие', () => {
  it('сбрасывает выбор, запрос и строку статуса, берёт свежий вид от Go', async () => {
    keysUi.selectKey('KeyF')
    keysUi.query = 'x'
    keysUi.onlyOwn = true
    api.Keymap.mockResolvedValue(catalogWith({ toggleBig: ['KeyB'] }))
    keysUi.show()
    expect(keysUi).toMatchObject({ open: true, sel: null, query: '', onlyOwn: false, status: null, pend: null })
    await flush()
    expect(keymap.keys('toggleBig')).toEqual(['KeyB'])
  })
  it('ошибка Go при открытии окно не ломает', async () => {
    api.Keymap.mockRejectedValue('нет связи')
    keysUi.show()
    await flush()
    expect(keysUi.open).toBe(true)
  })
})

describe('выбор клавиши и строки — одна картина', () => {
  it('нажатие клавиши: выбрана клавиша, слой по её модификаторам, список — к её команде', () => {
    keysUi.selectKey('Mod+KeyS')
    expect(keysUi.sel).toEqual({ kind: 'key', chord: 'Mod+KeyS', pick: null })
    expect(keysUi.mods).toEqual({ mod: true, alt: false, shift: false })
    expect(keysUi.cur).toBe('save')
    expect(keysUi.scroll?.id).toBe('save')
  })
  it('свободная клавиша: команды нет, прокручивать нечего', () => {
    keysUi.selectKey('KeyD')
    expect(keysUi.cur).toBeNull()
    expect(keysUi.scroll).toBeNull()
  })
  it('строка списка: выбрана команда, карта показывает слой её клавиши', () => {
    keysUi.selectRow('save')
    expect(keysUi.sel).toEqual({ kind: 'cmd', id: 'save' })
    expect(keysUi.cur).toBe('save')
    expect(keysUi.mods).toEqual({ mod: true, alt: false, shift: false })
  })
  it('команда без клавиши слой не меняет', () => {
    keysUi.toggleMod('alt')
    keysUi.selectRow('collect')
    expect(keysUi.mods.alt).toBe(true)
  })
  it('несколько команд на клавише: переключатель меняет команду, клавиша остаётся выбранной', () => {
    keysUi.selectKey('Digit0')
    const [a, b] = keysUi.ctx.index.get('Digit0')!.map((e) => e.id)
    expect(keysUi.cur).toBe(a)
    keysUi.pickCmd(b)
    expect(keysUi.cur).toBe(b)
    expect(keysUi.sel).toMatchObject({ kind: 'key', chord: 'Digit0', pick: b })
    keysUi.selectRow(a) // строка команды этой же клавиши — тоже просто смена команды
    expect(keysUi.sel).toMatchObject({ kind: 'key', chord: 'Digit0', pick: a })
  })
  it('строка чужой команды при выбранной клавише — выбор команды, а не клавиши', () => {
    keysUi.selectKey('Digit0')
    keysUi.selectRow('save')
    expect(keysUi.sel).toEqual({ kind: 'cmd', id: 'save' })
  })
  it('служебная клавиша команду не выбирает', () => {
    keysUi.selectKey('Escape')
    expect(keysUi.cur).toBeNull()
  })
  it('команда выше уровня на клавише не выбирается (на уровне «Простой» Ctrl+R пуста)', () => {
    prefs.level = 'simple'
    keysUi.selectKey('Mod+KeyR')
    expect(keysUi.cur).toBeNull()
  })
  it('pickCmd без выбранной клавиши ничего не делает', () => {
    keysUi.pickCmd('save')
    expect(keysUi.sel).toBeNull()
  })
})

describe('нажатия в открытом окне', () => {
  it('клавиша выбирается на карте и не доходит до программы; Ctrl+Z — тоже клавиша, а не отмена', () => {
    expect(keysUi.keydown(ev('d', 'KeyD'), false)).toBe('eat')
    expect(keysUi.sel).toMatchObject({ kind: 'key', chord: 'KeyD' })
    expect(keysUi.keydown(ev('я', 'KeyZ', { ctrlKey: true }), false)).toBe('eat')
    expect(keysUi.cur).toBe('undo')
  })
  it('в поле поиска клавиши — полю; Esc и Tab идут дальше', () => {
    expect(keysUi.keydown(ev('d', 'KeyD'), true)).toBe('pass')
    expect(keysUi.sel).toBeNull()
    expect(keysUi.keydown(ev('Escape', 'Escape'), false)).toBe('pass')
    expect(keysUi.keydown(ev('Tab', 'Tab'), false)).toBe('pass')
  })
  it('под вопросом «Заменить?» нажатия не забираются', () => {
    keysUi.ask = {
      kind: 'set',
      id: 'toggleBig',
      old: '',
      chord: 'KeyG',
      holder: { id: 'toggleSafe', label: 'x', chord: 'KeyG' },
      fromKey: false,
    }
    expect(keysUi.keydown(ev('d', 'KeyD'), false)).toBe('pass')
  })
  it('закрытое окно нажатия не трогает', () => {
    keysUi.close()
    expect(keysUi.keydown(ev('d', 'KeyD'), false)).toBe('pass')
  })
  it('модификаторы: зажатый Ctrl включает слой, отпущенный — выключает', () => {
    expect(keysUi.keydown(ev('Control', 'ControlLeft', { ctrlKey: true }), false)).toBe('eat')
    expect(keysUi.mods.mod).toBe(true)
    keysUi.keyup(ev('Control', 'ControlLeft'), false)
    expect(keysUi.mods.mod).toBe(false)
    keysUi.keyup(ev('Shift', 'ShiftLeft', { shiftKey: true }), true) // в поле поиска слой не трогаем
    expect(keysUi.mods.shift).toBe(false)
  })
  it('Enter, Пробел и стрелки на кнопке, флажке и в списке — самому элементу; с модификатором и вне виджета — карте', () => {
    const on = (what: string) => ({ closest: (sel: string) => (sel.includes(what) ? {} : null) })
    for (const what of ['button', "input[type='checkbox']", "[role='listbox']"]) {
      const own = ownsKeys(on(what))
      expect(own, what).toBe(true)
      for (const [key, code] of [
        ['Enter', 'Enter'],
        [' ', 'Space'],
        ['ArrowDown', 'ArrowDown'],
        ['ArrowLeft', 'ArrowLeft'],
      ])
        expect(keysUi.keydown(ev(key, code), false, own), `${what} ${code}`).toBe('pass')
    }
    expect(keysUi.sel).toBeNull()
    expect(keysUi.keydown(ev('ArrowDown', 'ArrowDown', { shiftKey: true }), false, true)).toBe('eat') // Shift+стрелка — клавиша карты
    expect(keysUi.keydown(ev('d', 'KeyD'), false, true)).toBe('eat') // буква на кнопке — выбор на карте
    expect(ownsKeys(on('div'))).toBe(false)
    expect(ownsKeys(null)).toBe(false)
    expect(keysUi.keydown(ev('Enter', 'Enter'), false, false)).toBe('eat') // не на виджете — по-прежнему клавиша карты
  })
  it('во время записи Enter, Пробел и стрелки забирает запись, даже если фокус на кнопке', async () => {
    api.SetHotkey.mockResolvedValue(ok(catalogWith({ toggleBig: ['Enter'] })))
    keysUi.startRec('toggleBig', '')
    expect(keysUi.keydown(ev('Enter', 'Enter'), false, true)).toBe('eat')
    await flush()
    expect(api.SetHotkey).toHaveBeenCalledWith('toggleBig', '', 'Enter', false)
  })
  it('F12 идёт дальше (инструменты разработчика)', () => {
    expect(keysUi.keydown(ev('F12', 'F12'), false)).toBe('pass')
  })
  it('слой — кнопки Ctrl, Shift, Alt переключают, повторный щелчок выключает', () => {
    keysUi.toggleMod('shift')
    expect(keysUi.mods.shift).toBe(true)
    keysUi.toggleMod('shift')
    expect(keysUi.mods.shift).toBe(false)
  })
})

describe('запись сочетания', () => {
  it('чип → нажатие → SetHotkey(id, старое, сочетание, false) → статус с «Вернуть»', async () => {
    api.SetHotkey.mockResolvedValue(ok(catalogWith({ toggleBig: ['KeyB'] })))
    keysUi.startRec('toggleBig', 'KeyF')
    expect(keysUi.rec).toEqual({ id: 'toggleBig', old: 'KeyF', partial: '' })
    expect(keysUi.keydown(ev('b', 'KeyB'), false)).toBe('eat')
    await flush()
    expect(api.SetHotkey).toHaveBeenCalledWith('toggleBig', 'KeyF', 'KeyB', false)
    expect(keysUi.rec).toBeNull()
    expect(keysUi.status?.text).toBe('B — «Во весь экран»')
    expect(keysUi.status?.prev).toEqual({})
    expect(keymap.keys('toggleBig')).toEqual(['KeyB'])
  })
  it('Esc — отмена записи, Go не зовётся, окно остаётся', () => {
    keysUi.startRec('toggleBig', 'KeyF')
    expect(keysUi.keydown(ev('Escape', 'Escape'), false)).toBe('eat')
    expect(keysUi.rec).toBeNull()
    expect(keysUi.open).toBe(true)
    expect(api.SetHotkey).not.toHaveBeenCalled()
  })
  it('Esc окна (Modal.onescape): сначала отмена записи, потом отмена «выберите команду», потом — закрытие', () => {
    keysUi.startRec('toggleBig', '')
    expect(keysUi.escape()).toBe(true)
    expect(keysUi.rec).toBeNull()
    keysUi.assignKey('KeyD')
    expect(keysUi.escape()).toBe(true)
    expect(keysUi.pend).toBeNull()
    expect(keysUi.escape()).toBe(false)
  })
  it('одни модификаторы — чип ждёт клавишу; отпустили — снова «Нажмите сочетание…»', () => {
    keysUi.startRec('toggleBig', 'KeyF')
    keysUi.keydown(ev('Control', 'ControlLeft', { ctrlKey: true }), false)
    expect(keysUi.rec?.partial).toBe('Ctrl+…')
    keysUi.keyup(ev('Control', 'ControlLeft'), false)
    expect(keysUi.rec?.partial).toBe('')
    expect(api.SetHotkey).not.toHaveBeenCalled()
  })
  it('автоповтор и ввод IME пропускаются: запись не начата и не сорвана', () => {
    keysUi.startRec('toggleBig', 'KeyF')
    keysUi.keydown(ev('b', 'KeyB', { repeat: true }), false)
    keysUi.keydown(ev('б', 'KeyB', { isComposing: true }), false)
    expect(api.SetHotkey).not.toHaveBeenCalled()
    expect(keysUi.rec).not.toBeNull()
  })
  it('потеря фокуса окна (Alt+Tab) отменяет запись; без записи — ничего', () => {
    keysUi.startRec('toggleBig', 'KeyF')
    keysUi.blur()
    expect(keysUi.rec).toBeNull()
    keysUi.blur()
    expect(keysUi.rec).toBeNull()
  })
  it('Ctrl+Alt уходит в Go; отказ с причиной — строка под чипом, запись остаётся', async () => {
    api.SetHotkey.mockRejectedValue('ctrl+alt на части раскладок Windows — это AltGr, выберите другое')
    keysUi.startRec('toggleBig', 'KeyF')
    keysUi.keydown(ev('e', 'KeyE', { ctrlKey: true, altKey: true }), false)
    await flush()
    expect(api.SetHotkey).toHaveBeenCalledWith('toggleBig', 'KeyF', 'Mod+Alt+KeyE', false)
    expect(keysUi.recErr).toEqual({ id: 'toggleBig', text: 'Ctrl+alt на части раскладок Windows — это AltGr, выберите другое' })
    expect(keysUi.rec).toMatchObject({ id: 'toggleBig' })
    expect(keysUi.status).toBeNull()
  })
  it('клавиша вне белого списка — строка «нельзя» без вызова Go', () => {
    keysUi.startRec('toggleBig', 'KeyF')
    keysUi.keydown(ev('AudioVolumeUp', 'AudioVolumeUp'), false)
    expect(keysUi.recErr?.text).toBe('Эту клавишу назначить нельзя.')
    expect(api.SetHotkey).not.toHaveBeenCalled()
  })
  it('новое сочетание «+»: старое пустое', async () => {
    api.SetHotkey.mockResolvedValue(ok(catalogWith({ toggleBig: ['KeyF', 'KeyB'] })))
    keysUi.startRec('toggleBig', '')
    keysUi.keydown(ev('b', 'KeyB'), false)
    await flush()
    expect(api.SetHotkey).toHaveBeenCalledWith('toggleBig', '', 'KeyB', false)
  })
  it('пока ответ Go не пришёл, вторая клавиша не принимается', async () => {
    let done: (r: KeymapSetResult) => void = () => {}
    api.SetHotkey.mockReturnValue(new Promise<KeymapSetResult>((r) => (done = r)))
    keysUi.startRec('toggleBig', 'KeyF')
    keysUi.keydown(ev('b', 'KeyB'), false)
    keysUi.keydown(ev('n', 'KeyN'), false)
    expect(api.SetHotkey).toHaveBeenCalledTimes(1)
    done(ok(catalogView))
    await flush()
  })
  it('перекрытие в другой области — строки «там сработает она» под статусом', async () => {
    api.SetHotkey.mockResolvedValue(
      ok(catalogWith({ toggleDetails: ['Digit0'] }), { shadows: [{ id: 'markRating0', label: 'Звёзды: убрать', chord: 'Digit0' }] }),
    )
    keysUi.startRec('toggleDetails', 'KeyI')
    keysUi.keydown(ev('0', 'Digit0'), false)
    await flush()
    expect(keysUi.status?.text).toBe('0 — «Панель деталей»')
    expect(keysUi.notes).toEqual(['В ленте фото 0 — «Звёзды: убрать», там сработает она'])
  })
})

describe('«Заменить?»', () => {
  const conflict = { id: 'toggleSafe', label: 'Безопасная зона', chord: 'KeyG' }
  const rec = async () => {
    api.SetHotkey.mockResolvedValueOnce(ok(catalogView, { conflict }))
    keysUi.startRec('toggleBig', 'KeyF')
    keysUi.keydown(ev('g', 'KeyG'), false)
    await flush()
  }
  it('занято в той же области — вопрос; запись закончена, клавиша не записана', async () => {
    await rec()
    expect(keysUi.ask).toMatchObject({ kind: 'set', id: 'toggleBig', old: 'KeyF', chord: 'KeyG', holder: conflict })
    expect(keysUi.rec).toBeNull()
    expect(keysUi.askText(keysUi.ask!)).toBe('Клавиша G уже у «Безопасная зона». Если заменить, у «Безопасная зона» этой клавиши не будет.')
    expect(keysUi.status).toBeNull()
  })
  it('«Отмена» — ничего не изменилось, Go больше не зовётся', async () => {
    await rec()
    keysUi.cancelAsk()
    expect(keysUi.ask).toBeNull()
    expect(api.SetHotkey).toHaveBeenCalledTimes(1)
    expect(keymap.keys('toggleBig')).toEqual(['KeyF'])
  })
  it('«Заменить» — SetHotkey(…, true), в статусе «(у «…» больше нет)», «Вернуть» вернёт прежнее', async () => {
    await rec()
    api.SetHotkey.mockResolvedValueOnce(ok(catalogWith({ toggleBig: ['KeyG'], toggleSafe: [] }), { replaced: conflict }))
    keysUi.confirmAsk()
    await flush()
    expect(api.SetHotkey).toHaveBeenLastCalledWith('toggleBig', 'KeyF', 'KeyG', true)
    expect(keysUi.ask).toBeNull()
    expect(keysUi.status?.text).toBe('G — «Во весь экран» (у «Безопасная зона» больше нет)')
    api.RestoreHotkeys.mockResolvedValue(catalogView)
    await keysUi.restore()
    expect(api.RestoreHotkeys).toHaveBeenCalledWith({})
    expect(keysUi.status).toBeNull()
  })
  it('держатель скрыт на этом уровне — вопрос это называет', async () => {
    prefs.level = 'simple'
    const hidden = { id: 'toggleRulers', label: 'Линейки', chord: 'Mod+KeyR' }
    api.SetHotkey.mockResolvedValueOnce(ok(catalogView, { conflict: hidden }))
    keysUi.startRec('toggleBig', 'KeyF')
    keysUi.keydown(ev('r', 'KeyR', { ctrlKey: true }), false)
    await flush()
    expect(keysUi.askText(keysUi.ask!)).toContain('«Линейки» (скрыта на этом уровне)')
  })
})

describe('переход стрелкой по строкам', () => {
  it('browse = выбор строки как щелчком', () => {
    keysUi.browse('save')
    expect(keysUi.sel).toEqual({ kind: 'cmd', id: 'save' })
    expect(keysUi.mods).toEqual({ mod: true, alt: false, shift: false })
  })
  it('пока клавиша ждёт команду, стрелка не назначает: ничего не выбирает и Go не зовёт', async () => {
    keysUi.assignKey('KeyB')
    keysUi.browse('toggleBig')
    await flush()
    expect(api.SetHotkey).not.toHaveBeenCalled()
    expect(keysUi.sel).toBeNull()
    expect(keysUi.pend).toBe('KeyB')
  })
  it('стрелки во время записи окно забирает: выбор не меняется', () => {
    keysUi.selectRow('save')
    keysUi.startRec('save', 'Mod+KeyS')
    expect(keysUi.keydown(ev('ArrowDown', 'ArrowDown'), false, true)).toBe('eat')
    expect(keysUi.sel).toEqual({ kind: 'cmd', id: 'save' })
  })
  it('в поле поиска стрелки не трогаются окном', () => {
    keysUi.selectRow('save')
    expect(keysUi.keydown(ev('ArrowDown', 'ArrowDown'), true)).toBe('pass')
    expect(keysUi.sel).toEqual({ kind: 'cmd', id: 'save' })
  })
  it('на строке списка ↑/↓/Home/End идут самой строке (own), а не выбору клавиши на карте', () => {
    keysUi.selectRow('save')
    for (const k of ['ArrowUp', 'ArrowDown', 'Home', 'End']) expect(keysUi.keydown(ev(k, k), false, true)).toBe('pass')
    expect(keysUi.sel).toEqual({ kind: 'cmd', id: 'save' })
  })
})

describe('назначение свободной клавиши (макет Г)', () => {
  it('«Назначить команду…» → строка списка → SetHotkey(id, "", клавиша); клавиша остаётся выбранной с этой командой', async () => {
    keysUi.selectKey('KeyB')
    keysUi.assignKey('KeyB')
    api.SetHotkey.mockResolvedValue(ok(catalogWith({ toggleBig: ['KeyF', 'KeyB'] })))
    keysUi.selectRow('toggleBig')
    await flush()
    expect(api.SetHotkey).toHaveBeenCalledWith('toggleBig', '', 'KeyB', false)
    expect(keysUi.pend).toBeNull()
    expect(keysUi.sel).toEqual({ kind: 'key', chord: 'KeyB', pick: 'toggleBig' })
    expect(keysUi.scroll?.id).toBe('toggleBig')
  })
  it('служебной команде клавишу не назначить — строка-причина, Go не зовётся, выбор остаётся', async () => {
    keysUi.assignKey('KeyB')
    keysUi.selectRow('guide')
    await flush()
    expect(api.SetHotkey).not.toHaveBeenCalled()
    expect(keysUi.notes[0]).toMatch(/Служебной команде/)
    expect(keysUi.pend).toBe('KeyB')
  })
  it('отмена «выберите команду»', () => {
    keysUi.assignKey('KeyB')
    keysUi.cancelPend()
    expect(keysUi.pend).toBeNull()
  })
  it('отказ Go (уже три сочетания) — строка в статусе, выбор остаётся', async () => {
    keysUi.assignKey('KeyB')
    api.SetHotkey.mockRejectedValue('у команды уже 3 сочетания')
    keysUi.selectRow('toggleBig')
    await flush()
    expect(keysUi.notes).toEqual(['У команды уже 3 сочетания'])
    expect(keysUi.pend).toBe('KeyB')
  })
  it('конфликт при назначении с клавиши — вопрос; «Заменить» оставляет клавишу выбранной', async () => {
    const conflict = { id: 'toggleSafe', label: 'Безопасная зона', chord: 'KeyG' }
    keysUi.selectKey('KeyG')
    keysUi.assignKey('KeyG')
    api.SetHotkey.mockResolvedValueOnce(ok(catalogView, { conflict }))
    keysUi.selectRow('toggleBig')
    await flush()
    expect(keysUi.ask?.fromKey).toBe(true)
    api.SetHotkey.mockResolvedValueOnce(ok(catalogView, { replaced: conflict }))
    keysUi.confirmAsk()
    await flush()
    expect(keysUi.sel).toEqual({ kind: 'key', chord: 'KeyG', pick: 'toggleBig' })
  })
})

describe('«Убрать», «Стандартное», «Сбросить всё», «Вернуть»', () => {
  it('«Убрать» у чипа: RemoveHotkey, статус «… убрана», «Вернуть» отдаёт прежнее', async () => {
    api.RemoveHotkey.mockResolvedValue(catalogWith({ toggleBig: [] }))
    await keysUi.remove('toggleBig', 'KeyF')
    expect(api.RemoveHotkey).toHaveBeenCalledWith('toggleBig', 'KeyF')
    expect(keysUi.status?.text).toBe('F убрана у «Во весь экран»')
    expect(keymap.keys('toggleBig')).toEqual([])
    api.RestoreHotkeys.mockResolvedValue(catalogView)
    await keysUi.restore()
    expect(keymap.keys('toggleBig')).toEqual(['KeyF'])
  })
  it('«Стандартное»: ResetHotkey(id, false), статус', async () => {
    useCatalog({ toggleBig: ['KeyB'] })
    api.ResetHotkey.mockResolvedValue(ok(catalogView))
    await keysUi.resetOne('toggleBig')
    expect(api.ResetHotkey).toHaveBeenCalledWith('toggleBig', false)
    expect(keysUi.status?.text).toBe('«Во весь экран» — стандартные клавиши')
    expect(keysUi.status?.prev).toEqual({ toggleBig: ['KeyB'] })
  })
  it('«Стандартное», упёршееся в чужое сочетание, — тот же вопрос; «Заменить» → ResetHotkey(id, true)', async () => {
    useCatalog({ toggleBig: ['KeyB'], toggleSafe: ['KeyF'] })
    const conflict = { id: 'toggleSafe', label: 'Безопасная зона', chord: 'KeyF' }
    api.ResetHotkey.mockResolvedValueOnce(ok(catalogView, { conflict }))
    await keysUi.resetOne('toggleBig')
    expect(keysUi.ask).toMatchObject({ kind: 'reset', id: 'toggleBig', chord: 'KeyF' })
    expect(keysUi.status).toBeNull()
    api.ResetHotkey.mockResolvedValueOnce(ok(catalogView))
    keysUi.confirmAsk()
    await flush()
    expect(api.ResetHotkey).toHaveBeenLastCalledWith('toggleBig', true)
    expect(keysUi.status?.text).toBe('«Во весь экран» — стандартные клавиши')
  })
  it('«Сбросить всё как было» без вопроса: «своих было N», «Вернуть» отдаёт своё', async () => {
    useCatalog({ toggleBig: ['KeyB'], toggleSafe: ['KeyS'], undo: [] })
    api.ResetHotkeys.mockResolvedValue(catalogView)
    await keysUi.resetAll()
    expect(keysUi.status?.text).toBe('Все клавиши — как было (своих было 3)')
    expect(keysUi.ask).toBeNull()
    api.RestoreHotkeys.mockResolvedValue(catalogWith({ toggleBig: ['KeyB'], toggleSafe: ['KeyS'], undo: [] }))
    await keysUi.restore()
    expect(api.RestoreHotkeys).toHaveBeenCalledWith({ toggleBig: ['KeyB'], toggleSafe: ['KeyS'], undo: [] })
    expect(keymap.view.changed).toBe(3)
  })
  it('«Вернуть» без правки ничего не делает; ошибка Go — тост, статус остаётся', async () => {
    await keysUi.restore()
    expect(api.RestoreHotkeys).not.toHaveBeenCalled()
    api.RemoveHotkey.mockResolvedValue(catalogWith({ toggleBig: [] }))
    await keysUi.remove('toggleBig', 'KeyF')
    api.RestoreHotkeys.mockRejectedValue('не удалось записать настройки')
    await keysUi.restore()
    expect(keysUi.status).not.toBeNull()
    expect(store.toast?.text).toBe('Не удалось записать настройки')
  })
  it('ошибки Go у «Убрать», «Стандартное», «Сбросить всё» — тост, вид не меняется', async () => {
    api.RemoveHotkey.mockRejectedValue('сбой')
    api.ResetHotkey.mockRejectedValue('сбой')
    api.ResetHotkeys.mockRejectedValue('сбой')
    await keysUi.remove('toggleBig', 'KeyF')
    await keysUi.resetOne('toggleBig')
    await keysUi.resetAll()
    expect(keymap.keys('toggleBig')).toEqual(['KeyF'])
    expect(keysUi.status).toBeNull()
  })
})

describe('перекрытия в карточке', () => {
  it('Digit0: у «Звёзды: убрать» (лента) — «там сработает» про «Вписать» с более широкой областью не бывает; у «Вписать» — про ленту', () => {
    const fit = keymap.entry('fitZoom')!
    const notes = keysUi.notesFor(fit)
    expect(notes.some((n) => n.includes('«Звёзды: убрать»'))).toBe(true)
    expect(keysUi.notesFor(keymap.entry('toggleBig')!)).toEqual([])
  })
})

describe('имена команд в «Горячих клавишах» (128-11, Т7)', () => {
  it('F — «Во весь экран» / «Весь экран», Z — «Крупный просмотр фото» / «Крупно»', () => {
    expect(keymap.entry('toggleBig')).toMatchObject({ label: 'Во весь экран', short: 'Весь экран' })
    expect(keymap.entry('photoPreview')).toMatchObject({ label: 'Крупный просмотр фото', short: 'Крупно' })
    expect(keymap.entry('toggleBig')?.keys).toEqual(['KeyF'])
  })
})

describe('закрытие', () => {
  it('запись, вопрос и «выберите команду» гаснут; повторное открытие начинает с чистого листа', () => {
    keysUi.startRec('toggleBig', '')
    keysUi.assignKey('KeyD')
    keysUi.close()
    expect(keysUi).toMatchObject({ open: false, rec: null, ask: null, pend: null })
    dropCatalog()
  })
})
