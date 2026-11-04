import { afterEach, describe, expect, it } from 'vitest'
import parity from '../../../../testdata/parity/keymap.json'
import { setMacForTest } from '../core/platform'
import type { KeymapView } from '../types/keys'
import { buildIndex, EMPTY_KEYMAP } from './keyindex'
import { keymap } from './keymap.svelte'

// Вид по умолчанию от Go (тот же файл, что у TestCatalogGolden): тип JSON шире, чем KeymapView.
const view = parity.view as unknown as KeymapView

afterEach(() => {
  keymap.view = EMPTY_KEYMAP
  setMacForTest(null)
})

describe('buildIndex', () => {
  it('ранг области главнее, внутри области — порядок каталога', () => {
    const idx = buildIndex(view)
    // 0 в ленте фото — «Звёзды: убрать», на холсте и везде — «Вписать» (design: перекрытие 0/Del/Backspace).
    const zero = (idx.get('Digit0') ?? []).map((e) => e.id)
    expect(zero.length).toBeGreaterThan(1)
    const rank = new Map(view.scopes.map((s) => [s.id, s.rank]))
    const ranks = (idx.get('Digit0') ?? []).map((e) => rank.get(e.scope) ?? 0)
    expect(ranks).toEqual([...ranks].sort((a, b) => b - a))
    // порядок каталога внутри равного ранга
    const order = new Map(view.commands.map((e, i) => [e.id, i]))
    const list = idx.get('Digit0') ?? []
    for (let i = 1; i < list.length; i++) {
      if (rank.get(list[i - 1].scope) === rank.get(list[i].scope)) expect(order.get(list[i - 1].id)!).toBeLessThan(order.get(list[i].id)!)
    }
  })

  it('Delete: режим фона старше холста', () => {
    const del = (buildIndex(view).get('Delete') ?? []).map((e) => e.scope)
    expect(del[0]).toBe('bg')
    expect(del).toContain('canvas')
  })

  it('все стандартные сочетания каталога есть в индексе, команда — под своей клавишей', () => {
    const idx = buildIndex(view)
    for (const e of view.commands)
      for (const k of e.keys)
        expect(
          idx.get(k)?.some((x) => x.id === e.id),
          `${e.id} ${k}`,
        ).toBe(true)
  })

  it('команда без клавиш в индекс не входит; испорченное сочетание пропущено', () => {
    const base = view.commands[0]
    const v: KeymapView = {
      ...view,
      commands: [
        { ...base, id: 'a', keys: [] },
        { ...base, id: 'b', keys: ['Mod+KeyN', 'NoSuchKey', 'Shift+Mod+KeyN'] },
      ],
    }
    const idx = buildIndex(v)
    // Shift+Mod+KeyN приводится к каноническому Mod+Shift+KeyN, NoSuchKey отброшен
    expect([...idx.keys()]).toEqual(['Mod+KeyN', 'Mod+Shift+KeyN'])
  })

  it('неизвестная область — самый низкий ранг', () => {
    const base = view.commands[0]
    const v: KeymapView = {
      ...view,
      commands: [
        { ...base, id: 'x', scope: 'nowhere' as never, keys: ['KeyQ'] },
        { ...base, id: 'y', scope: 'app', keys: ['KeyQ'] },
      ],
    }
    expect(
      buildIndex(v)
        .get('KeyQ')
        ?.map((e) => e.id),
    ).toEqual(['y', 'x'])
  })

  it('пустой вид — пустой индекс', () => {
    expect(buildIndex(EMPTY_KEYMAP).size).toBe(0)
  })
})

describe('keymap', () => {
  it('до загрузки: команд нет, hint пустой', () => {
    expect(keymap.entry('undo')).toBeUndefined()
    expect(keymap.keys('undo')).toEqual([])
    expect(keymap.hint('undo')).toBe('')
    expect(keymap.index.size).toBe(0)
  })

  it('apply: индекс и hint обновляются', () => {
    keymap.apply(view)
    expect(keymap.entry('undo')?.label).toBeTruthy()
    expect(keymap.keys('undo')).toEqual(['Mod+KeyZ'])
    setMacForTest(false)
    expect(keymap.hint('undo')).toBe('Ctrl+Z')
    setMacForTest(true)
    expect(keymap.hint('undo')).toBe('⌘Z')
    expect(keymap.index.get('Mod+KeyZ')?.map((e) => e.id)).toContain('undo')
  })

  it('hint — первая клавиша; команда без клавиши и неизвестная — пусто', () => {
    const undo = view.commands.find((e) => e.id === 'undo')!
    const noKey = view.commands.find((e) => e.keys.length === 0)!
    keymap.apply({ ...view, commands: [{ ...undo, keys: ['Mod+KeyY', 'Mod+KeyZ'] }, noKey] })
    setMacForTest(false)
    expect(keymap.hint('undo')).toBe('Ctrl+Y')
    expect(keymap.hint(noKey.id)).toBe('')
    expect(keymap.hint('нет такой')).toBe('')
  })

  it('apply заменяет вид целиком: свои клавиши после правки окна', () => {
    keymap.apply(view)
    const undo = view.commands.find((e) => e.id === 'undo')!
    keymap.apply({ ...view, commands: view.commands.map((e) => (e === undo ? { ...e, keys: ['Mod+KeyU'], custom: true } : e)), changed: 1 })
    expect(keymap.keys('undo')).toEqual(['Mod+KeyU'])
    expect(keymap.index.get('Mod+KeyZ')?.some((e) => e.id === 'undo') ?? false).toBe(false)
    expect(keymap.index.get('Mod+KeyU')?.[0].id).toBe('undo')
  })

  it('hint(id, n) — клавиша по номеру; tip — подпись со скобками, без клавиши — как есть', () => {
    keymap.apply(view)
    setMacForTest(false)
    expect(keymap.hint('spaceDown', 1)).toBe('Shift+Пробел')
    expect(keymap.hint('undo', 5)).toBe('')
    expect(keymap.tip('Отменить', 'undo')).toBe('Отменить (Ctrl+Z)')
    expect(keymap.tip('Предыдущая раскладка', 'spaceDown', 1)).toBe('Предыдущая раскладка (Shift+Пробел)')
    expect(keymap.tip('Подпись', 'нет такой')).toBe('Подпись')
  })

  it('text подставляет метки {key:id}: сочетание, шаблон, «без клавиши»', () => {
    keymap.apply(view)
    setMacForTest(false)
    expect(keymap.text('Отмена {key:undo}')).toBe('Отмена Ctrl+Z')
    expect(keymap.text('Отмена{key:undo| (%)}.')).toBe('Отмена (Ctrl+Z).')
    expect(keymap.text('Разметка{key:toggleMarkup|; %}.')).toBe('Разметка.')
    expect(keymap.text('Клавиша {key:toggleMarkup}')).toBe('Клавиша без клавиши')
  })

  it('apply(null/undefined) вид не затирает (Go без поля до G2)', () => {
    keymap.apply(view)
    keymap.apply(null)
    keymap.apply(undefined)
    expect(keymap.view.commands.length).toBe(view.commands.length)
  })
})
