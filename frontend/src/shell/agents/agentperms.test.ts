import { describe, expect, it } from 'vitest'
import type { AgentPerm } from './agents'
import {
  askMissing,
  commonMode,
  consentFor,
  groupAskMissing,
  groupCan,
  isMode,
  noteShown,
  placesShown,
  rowKey,
  rowsOf,
  segItems,
  targets,
  type PermChange,
} from './agentperms'

// Чистая логика окна «Разрешения» (150, design §7): группы, общее значение, какие согласия нужны.
const P = (tool: string, group: string, mode: string, can = ['allow', 'ask', 'deny']): AgentPerm => ({
  tool,
  title: tool,
  group,
  mode,
  can,
  why: '',
  note: '',
})
const READ = ['allow', 'deny']
const rows = (m: Record<string, string> = {}): AgentPerm[] => [
  P('get_album', 'read', 'allow', READ),
  P('list_photos', 'read', 'allow', READ),
  P('create_album', 'edit', m.create_album ?? 'deny'),
  P('add_folder', 'edit', m.add_folder ?? 'deny'),
  P('autobuild', 'edit', m.autobuild ?? 'deny'),
  P('start_program', 'other', 'allow', READ),
  P('show_spread', 'other', 'allow'),
]
const tool = (t: string, mode: PermChange['mode']): PermChange => ({ kind: 'tool', tool: t, mode })
const group = (g: 'read' | 'edit', mode: PermChange['mode']): PermChange => ({ kind: 'group', group: g, mode })

describe('группы и общее значение', () => {
  it('rowsOf: строки группы в порядке Go; «other» — отдельно', () => {
    expect(rowsOf(rows(), 'edit').map((r) => r.tool)).toEqual(['create_album', 'add_folder', 'autobuild'])
    expect(rowsOf(rows(), 'other').map((r) => r.tool)).toEqual(['start_program', 'show_spread'])
    expect(rowsOf([], 'edit')).toEqual([])
  })

  it('commonMode: одно значение у всех — оно; разные, пусто и незнакомое — undefined', () => {
    expect(commonMode(rowsOf(rows(), 'edit'))).toBe('deny')
    expect(commonMode(rowsOf(rows({ add_folder: 'ask' }), 'edit'))).toBeUndefined()
    expect(commonMode([])).toBeUndefined()
    expect(commonMode([P('x', 'edit', 'странное')])).toBeUndefined()
  })

  it('isMode принимает только три значения', () => {
    expect(['allow', 'ask', 'deny'].every(isMode)).toBe(true)
    expect(isMode('')).toBe(false)
    expect(isMode('ALLOW')).toBe(false)
  })

  it('rowKey — ключ подсветки строки инструмента', () => {
    expect(rowKey('add_folder')).toBe('tool:add_folder')
  })
})

describe('«Спрашивать» там, где его нет', () => {
  it('askMissing: нет в can; groupAskMissing — у группы только когда его нет у всех строк', () => {
    expect(askMissing(P('get_album', 'read', 'allow', READ))).toBe(true)
    expect(askMissing(P('autobuild', 'edit', 'deny'))).toBe(false)
    expect(groupAskMissing(rowsOf(rows(), 'read'))).toBe(true)
    expect(groupAskMissing(rowsOf(rows(), 'edit'))).toBe(false)
    expect(groupAskMissing([P('a', 'x', 'allow', READ), P('b', 'x', 'allow')])).toBe(false)
    expect(groupAskMissing([])).toBe(false)
  })

  it('segItems: «Спрашивать» не серый, даже если Go его не принимает; чужое значение вне can серое; off гасит всё', () => {
    const read = segItems(READ, false)
    expect(read.map((i) => [i.label, !!i.off])).toEqual([
      ['Всегда', false],
      ['Спрашивать', false],
      ['Запретить', false],
    ])
    const only = segItems(['ask', 'deny'], false)
    expect(only.find((i) => i.id === 'allow')?.off).toBe(true)
    expect(segItems(['allow', 'ask', 'deny'], true).every((i) => i.off)).toBe(true)
  })

  it('groupCan: объединение can строк группы', () => {
    expect(groupCan(rowsOf(rows(), 'read')).sort()).toEqual(['allow', 'deny'])
    expect(groupCan(rowsOf(rows(), 'edit')).sort()).toEqual(['allow', 'ask', 'deny'])
    expect(groupCan([])).toEqual([])
  })
})

describe('targets: что меняет щелчок', () => {
  it('строка — она сама; группа — её строки, которым значение допустимо', () => {
    expect(targets(rows(), tool('autobuild', 'ask')).map((r) => r.tool)).toEqual(['autobuild'])
    expect(targets(rows(), tool('get_album', 'ask'))).toEqual([]) // чтению «Спрашивать» недоступно
    expect(targets(rows(), group('edit', 'allow')).map((r) => r.tool)).toEqual(['create_album', 'add_folder', 'autobuild'])
    expect(targets(rows(), group('read', 'ask'))).toEqual([])
    expect(targets(rows(), tool('нет-такого', 'allow'))).toEqual([])
  })

  it('групповой щелчок «Правки» не берёт «Запустить программу» и «Показать разворот»', () => {
    expect(targets(rows(), group('edit', 'deny')).map((r) => r.tool)).not.toContain('start_program')
    expect(targets(rows(), group('edit', 'deny')).map((r) => r.tool)).not.toContain('show_spread')
  })
})

describe('consentFor: какие согласия нужны', () => {
  it('правки целиком «Запретить» → любой выход из него просит согласие 149', () => {
    expect(consentFor(rows(), false, tool('autobuild', 'ask'))).toEqual({ edit: true, folder: false })
    expect(consentFor(rows(), false, tool('autobuild', 'allow'))).toEqual({ edit: true, folder: false })
    expect(consentFor(rows(), false, group('edit', 'ask'))).toEqual({ edit: true, folder: false })
  })

  it('«Всегда» у папки или альбома просит согласие с местами; при выключенных правках — оба сразу', () => {
    expect(consentFor(rows(), false, tool('add_folder', 'allow'))).toEqual({ edit: true, folder: true })
    expect(consentFor(rows(), false, group('edit', 'allow'))).toEqual({ edit: true, folder: true })
    expect(consentFor(rows(), true, tool('add_folder', 'allow'))).toEqual({ edit: false, folder: true })
    expect(consentFor(rows(), true, tool('create_album', 'allow'))).toEqual({ edit: false, folder: true })
    expect(consentFor(rows({ add_folder: 'ask' }), true, tool('add_folder', 'allow'))).toEqual({ edit: false, folder: true })
  })

  it('правки уже включены: «Спрашивать» у папки и «Всегда» у автосборки — без вопросов', () => {
    expect(consentFor(rows(), true, tool('add_folder', 'ask'))).toEqual({ edit: false, folder: false })
    expect(consentFor(rows(), true, tool('autobuild', 'allow'))).toEqual({ edit: false, folder: false })
  })

  it('снятие и повторное значение — без вопросов', () => {
    expect(consentFor(rows({ add_folder: 'allow' }), true, tool('add_folder', 'deny'))).toEqual({ edit: false, folder: false })
    expect(consentFor(rows({ add_folder: 'allow' }), true, tool('add_folder', 'allow'))).toEqual({ edit: false, folder: false })
    expect(consentFor(rows(), false, tool('autobuild', 'deny'))).toEqual({ edit: false, folder: false }) // «Запретить» у «Запретить»
    expect(consentFor(rows({ add_folder: 'allow', create_album: 'allow', autobuild: 'allow' }), true, group('edit', 'deny'))).toEqual({
      edit: false,
      folder: false,
    })
  })

  it('чтение согласий не просит ни в каком значении; группа «Всегда» при уже «Всегда» папках — тоже', () => {
    expect(consentFor(rows(), false, group('read', 'deny'))).toEqual({ edit: false, folder: false })
    expect(consentFor(rows(), false, group('read', 'allow'))).toEqual({ edit: false, folder: false })
    expect(consentFor(rows({ add_folder: 'allow', create_album: 'allow', autobuild: 'allow' }), true, group('edit', 'allow'))).toEqual({
      edit: false,
      folder: false,
    })
  })

  it('«Запустить программу» и «Показать разворот» согласий не просят и не считаются правкой', () => {
    expect(consentFor(rows(), false, tool('show_spread', 'ask'))).toEqual({ edit: false, folder: false })
    expect(consentFor(rows(), false, tool('start_program', 'deny'))).toEqual({ edit: false, folder: false })
  })

  it('часть группы «Всегда», часть «Спрашивать» → «Всегда» у всей группы просит согласие с местами только за недостающие', () => {
    expect(consentFor(rows({ add_folder: 'allow', create_album: 'ask', autobuild: 'ask' }), true, group('edit', 'allow'))).toEqual({
      edit: false,
      folder: true,
    })
    expect(consentFor(rows({ add_folder: 'allow', create_album: 'allow', autobuild: 'ask' }), true, group('edit', 'allow'))).toEqual({
      edit: false,
      folder: false,
    })
  })
})

describe('placesShown: места в строке', () => {
  const pl = (...d: string[]) => d.map((dir) => ({ dir, chosen: false }))
  it('до двух путей и сколько осталось', () => {
    expect(placesShown([])).toEqual({ shown: [], more: 0 })
    expect(placesShown(pl('D:\\a'))).toEqual({ shown: ['D:\\a'], more: 0 })
    expect(placesShown(pl('D:\\a', 'E:\\b'))).toEqual({ shown: ['D:\\a', 'E:\\b'], more: 0 })
    expect(placesShown(pl('D:\\a', 'E:\\b', 'F:\\c', 'G:\\d'))).toEqual({ shown: ['D:\\a', 'E:\\b'], more: 2 })
  })
})

describe('noteShown: подпись у папки и альбома', () => {
  const n = (tool: string, mode: string, note = 'Подпись'): AgentPerm => ({ ...P(tool, 'edit', mode), note })
  it('у папки и альбома — только при «Всегда»; у прочих строк — при любом значении; пустая подпись не показывается', () => {
    expect(noteShown(n('add_folder', 'allow'))).toBe(true)
    expect(noteShown(n('create_album', 'allow'))).toBe(true)
    expect(noteShown(n('add_folder', 'ask'))).toBe(false)
    expect(noteShown(n('create_album', 'deny'))).toBe(false)
    expect(noteShown(n('get_photo_preview', 'ask'))).toBe(true)
    expect(noteShown(n('add_folder', 'allow', ''))).toBe(false)
  })
})
