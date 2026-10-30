import { describe, expect, it } from 'vitest'
import type { AgentPerm, AgentPlace } from './agents'
import {
  allowMissing,
  askMissing,
  askOnly,
  commonMode,
  consentFor,
  groupAskMissing,
  groupCan,
  liftGroups,
  isMode,
  noteShown,
  placeRows,
  placesShown,
  rowKey,
  rowsOf,
  segItems,
  targets,
  type PermChange,
} from './agentperms'

const mkPlace = (dir: string, o: Partial<AgentPlace> = {}): AgentPlace => ({
  key: dir,
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
  const pl = (...d: string[]) => d.map((dir) => mkPlace(dir))
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

describe('placeRows: строки списка мест (153)', () => {
  it('сохранённое: путь и серая метка; ссылки «Привязать» до ответа AgentPlacesLive нет', () => {
    const saved = [mkPlace('D:\\Фото', { label: 'Фото-архив' }), mkPlace('E:\\Клиент', { bound: false })]
    expect(placeRows(saved, null, false)).toEqual([
      { key: 'D:\\Фото', dir: 'D:\\Фото', right: 'Фото-архив', dim: false, hints: [], bind: '' },
      { key: 'E:\\Клиент', dir: 'E:\\Клиент', right: '', dim: false, hints: ['не привязано к диску'], bind: '' },
    ])
  })

  it('живое поверх сохранённого по ключу: путь после смены буквы, «Привязать к диску «метка»», без метки — «к этому диску»', () => {
    const saved = [mkPlace('E:\\Клиент', { key: 'k1', bound: true }), mkPlace('F:\\Съёмки', { key: 'k2', bound: false })]
    const live = [
      mkPlace('G:\\Клиент', { key: 'k1', label: 'Фото-архив' }),
      mkPlace('F:\\Съёмки', { key: 'k2', bound: false, state: 'here', here: 'Карта' }),
    ]
    const r = placeRows(saved, live, false)
    expect(r[0]).toMatchObject({ dir: 'G:\\Клиент', right: 'Фото-архив', hints: [], bind: '' })
    expect(r[1]).toMatchObject({ hints: ['не привязано к диску'], bind: 'Привязать к диску «Карта»' })
    live[1].here = ''
    expect(placeRows(saved, live, false)[1].bind).toBe('Привязать к этому диску')
  })

  it('state пуст (диск по пути не определён): ссылки привязки нет, строка не бледная', () => {
    const saved = [mkPlace('F:\\Съёмки', { key: 'k2', bound: false })]
    const r = placeRows(saved, [mkPlace('F:\\Съёмки', { key: 'k2', bound: false, here: 'Карта' })], false)[0]
    expect(r).toMatchObject({ dim: false, bind: '', hints: ['не привязано к диску'] })
  })

  it('диска нет и папки нет: бледная строка и слова справа; ссылки «Привязать» у такого места нет', () => {
    const saved = [mkPlace('E:\\A', { key: 'a', bound: false }), mkPlace('E:\\B', { key: 'b', bound: false })]
    const live = [
      mkPlace('E:\\A', { key: 'a', bound: false, state: 'nodisk', label: 'Фото-архив' }),
      mkPlace('E:\\B', { key: 'b', bound: false, state: 'nofolder' }),
    ]
    const r = placeRows(saved, live, false)
    expect(r[0]).toMatchObject({ right: 'Диска «Фото-архив» нет', dim: true, bind: '' })
    expect(r[1]).toMatchObject({ right: 'Папка не найдена', dim: true, bind: '' })
    expect(
      placeRows([mkPlace('E:\\A', { key: 'a', state: 'nodisk' })], [mkPlace('E:\\A', { key: 'a', state: 'nodisk' })], false)[0].right,
    ).toBe('Диска нет')
  })

  it('узкое место: подсказка с именем папки над ним; на корне диска — «Диск E: целиком»; на Linux без буквы — «Диск целиком»', () => {
    const r = placeRows([mkPlace('/home/u/Изображения/2026', { narrow: true }), mkPlace('D:\\Фото\\2026', { narrow: true })], null, false)
    expect(r[0].hints).toEqual(['Новый год спросит снова. Изображения целиком — «Папка со съёмками…»'])
    expect(r[1].hints).toEqual(['Новый год спросит снова. Фото целиком — «Папка со съёмками…»'])
    const root = placeRows(
      [mkPlace('E:\\2026', { narrow: true, narrowRoot: true }), mkPlace('/2026', { narrow: true, narrowRoot: true })],
      null,
      false,
    )
    expect(root[0].hints).toEqual(['Диск E: целиком — «Папка со съёмками…»'])
    expect(root[1].hints).toEqual(['Диск целиком — «Папка со съёмками…»'])
  })

  it('macOS: привязки нет — ни «не привязано», ни ссылки', () => {
    const saved = [mkPlace('/Volumes/A/Фото', { key: 'a', bound: false })]
    expect(placeRows(saved, [mkPlace('/Volumes/A/Фото', { key: 'a', bound: false, state: 'here', here: 'A' })], true)[0]).toMatchObject({
      hints: [],
      bind: '',
    })
  })

  it('место, которого нет в живом ответе (добавлено после него), показывается по сохранённому', () => {
    const r = placeRows([mkPlace('D:\\Новое', { key: 'n' })], [mkPlace('D:\\Старое', { key: 'o' })], false)
    expect(r.map((x) => x.dir)).toEqual(['D:\\Новое'])
  })
})

describe('liftGroups: заметка о подъёме мест', () => {
  it('на каждое «куда» — все «откуда» без повторов; пусто — нет заметки', () => {
    expect(liftGroups([])).toEqual([])
    expect(
      liftGroups([
        { from: 'D:\\Фото\\2026', to: 'D:\\Фото' },
        { from: 'D:\\Фото\\2025', to: 'D:\\Фото' },
        { from: 'D:\\Фото\\2025', to: 'D:\\Фото' },
        { from: 'E:\\2026', to: 'E:\\Архив' },
      ]),
    ).toEqual([
      { to: 'D:\\Фото', from: ['D:\\Фото\\2026', 'D:\\Фото\\2025'] },
      { to: 'E:\\Архив', from: ['E:\\2026'] },
    ])
  })
})

// Класс Q (152): выгрузка и открытие — can = [ask, deny], «Всегда» нет.
describe('askOnly: строки «только с вопросом»', () => {
  const Q = ['ask', 'deny']
  const qrows = [P('export_album', 'other', 'ask', Q), P('open_album', 'other', 'deny', Q)]

  it('askOnly — «Спрашивать» есть, «Всегда» нет; чтение, запуск и правки — нет', () => {
    expect(qrows.map(askOnly)).toEqual([true, true])
    expect(askOnly(P('get_album', 'read', 'allow', READ))).toBe(false)
    expect(askOnly(P('start_program', 'other', 'allow', READ))).toBe(false)
    expect(askOnly(P('autobuild', 'edit', 'ask'))).toBe(false)
  })

  it('сегмент: «Всегда» серое, «Спрашивать» живое (не askMissing), «Запретить» живое', () => {
    const items = segItems(qrows[0].can, false)
    expect(items.map((i) => [i.id, !!i.off])).toEqual([
      ['allow', true],
      ['ask', false],
      ['deny', false],
    ])
    expect(askMissing(qrows[0])).toBe(false)
  })

  it('allowMissing и сегмент: с why «Всегда» живое (объясняет), без why — серое; чтению и правкам не касается', () => {
    const why = 'Спрашивают всегда.'
    const q = { ...qrows[0], why }
    expect(allowMissing(q)).toBe(true)
    expect(allowMissing(qrows[0])).toBe(false)
    expect(allowMissing(P('autobuild', 'edit', 'ask'))).toBe(false)
    expect(segItems(q.can, false, true).map((i) => [i.id, !!i.off])).toEqual([
      ['allow', false],
      ['ask', false],
      ['deny', false],
    ])
    expect(segItems(q.can, true, true).every((i) => i.off)).toBe(true) // блокировка правок сильнее
  })

  it('групповой щелчок «Правки» Q не трогает: строки в группе other', () => {
    const all = [...rows(), ...qrows]
    expect(targets(all, group('edit', 'allow')).map((r) => r.tool)).toEqual(['create_album', 'add_folder', 'autobuild'])
    expect(targets(all, group('edit', 'deny')).map((r) => r.tool)).not.toContain('export_album')
    expect(targets(all, tool('export_album', 'allow'))).toEqual([]) // «Всегда» Q-строке недопустимо
    expect(targets(all, tool('export_album', 'deny')).map((r) => r.tool)).toEqual(['export_album'])
  })

  it('consentFor: «Спрашивать»/«Запретить» у Q согласий не требует', () => {
    const all = [...rows(), ...qrows]
    expect(consentFor(all, false, tool('export_album', 'ask'))).toEqual({ edit: false, folder: false })
    expect(consentFor(all, false, tool('open_album', 'deny'))).toEqual({ edit: false, folder: false })
  })
})
