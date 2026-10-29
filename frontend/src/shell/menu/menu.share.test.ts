// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { dropCatalog, useCatalog } from '../../base/keys/keymap.fixture'
import { ALL_LEVELS, can } from '../../base/prefs/levels'
import { articleById } from '../guidetext/articles'
import { menuPath } from '../guide/show'
import { buildMenu, keyId, type MenuCtx, type MenuItem } from './menu'

// «Файл ▸ Отправить по ссылке…» (146, Т5): пункт сразу после «Экспорт…», серый без разворотов и у листа-виньетки (031),
// команда открывает окно; статья F1 `share-link` ведёт на этот пункт, а «Экспорт» ссылается на неё.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({}))

const { COMMANDS } = await import('./commands')
const { share } = await import('../share/share.svelte')
const { StubShare, stubView } = await import('../share/sharestub')
const { store } = await import('../../base/core/store.svelte')

const stub = () => share.bridge as InstanceType<typeof StubShare>
// Проект с n разворотами; sheet — лист-виньетка (settings.sheet).
function withSpreads(n: number, sheet = false) {
  const spreads = Array.from({ length: n }, (_, i) => ({ id: `s${i}`, photos: [] }))
  store.view = { ...store.view, spreads, settings: { ...store.view.settings, sheet } } as unknown as typeof store.view
}

beforeEach(() => {
  useCatalog()
  share.view = null
  share.bridge = new StubShare('plugged')
  share.close()
})
afterEach(() => {
  dropCatalog()
  share.close()
})

const base: MenuCtx = {
  spreads: 3,
  hasSpread: true,
  canUndo: false,
  canRedo: false,
  zone: 'canvas',
  libraryPhotos: 0,
  anySelection: false,
  deletable: false,
  flippable: false,
  varyable: false,
  layerable: false,
  missing: 0,
  big: false,
  details: false,
  safe: false,
  rulers: false,
  snap: false,
}
const file = (c: Partial<MenuCtx>): MenuItem[] => buildMenu({ ...base, ...c }).find((m) => m.id === 'file')!.items
const item = (c: Partial<MenuCtx>): MenuItem => file(c).find((i) => i.id === 'share')!

describe('«Файл ▸ Отправить по ссылке…» (146)', () => {
  it('сразу после «Экспорт…», до «Открыть папку экспорта»; подпись с многоточием; без значка и клавиши', () => {
    const ids = file({}).map((i) => i.id)
    expect(ids.indexOf('share')).toBe(ids.indexOf('export') + 1)
    expect(ids.indexOf('share')).toBeLessThan(ids.indexOf('showExport'))
    expect(item({})).toMatchObject({ label: 'Отправить по ссылке…', cmd: 'share' })
    expect(item({}).icon).toBeUndefined()
    expect(keyId('share')).toBeUndefined()
  })

  it('есть развороты — доступен; на всех уровнях', () => {
    for (const level of ALL_LEVELS) {
      const it = file({ level, can: (f) => can(f, level) }).find((i) => i.id === 'share')
      expect(it, level).toBeDefined()
      expect(it!.disabled, level).toBeFalsy()
    }
  })

  it('без разворотов серый; у листа-виньетки серый; идущий экспорт его без листов не открывает', () => {
    expect(item({ spreads: 0 }).disabled).toBe(true)
    expect(item({ sheetDoc: true }).disabled).toBe(true)
    expect(item({ spreads: 0, exporting: true }).disabled).toBe(true)
  })

  it('идёт своя выгрузка — пункт не серый (открывает ход), даже без листов', () => {
    expect(item({ spreads: 0, sharing: true }).disabled).toBeFalsy()
    expect(item({ sheetDoc: true, sharing: true }).disabled).toBeFalsy()
    expect(item({ spreads: 0, sharing: false }).disabled).toBe(true)
  })

  it('команда открывает окно и читает состояние Go; повторный вызов окно не дублирует', async () => {
    withSpreads(2)
    expect(share.open).toBe(false)
    await COMMANDS.share()
    expect(share.open).toBe(true)
    expect(share.view?.service.host).toBe('albums.sosnina-films.ru')
    await COMMANDS.share()
    expect(share.open).toBe(true)
  })

  // Как у «Экспорта»: пункт серый, а команда не из меню (стартовое окно, без альбома) не открывает окно, а говорит почему.
  it('без разворотов команда окно не открывает — тост, как у «Экспорта»; виньетка — свой тост; идущая выгрузка открывает ход', async () => {
    const notify = vi.spyOn(store, 'notify').mockImplementation(() => {})
    withSpreads(0)
    await COMMANDS.share()
    expect(share.open).toBe(false)
    expect(notify).toHaveBeenCalledWith('Отправлять нечего: в альбоме нет разворотов')
    expect(stub().calls).not.toContain('ShareState')

    withSpreads(2, true)
    await COMMANDS.share()
    expect(share.open).toBe(false)
    expect(notify).toHaveBeenCalledWith('Лист-виньетка не выгружается по ссылке')

    withSpreads(0)
    share.view = stubView('running')
    await COMMANDS.share() // своя выгрузка идёт — окно хода открывается и без листов
    expect(share.open).toBe(true)
    notify.mockRestore()
  })

  it('статья F1: меню ведёт на пункт, абзац про данные клиента на месте, «Экспорт» ссылается на неё', () => {
    const a = articleById('share-link')!
    expect(a.menu).toEqual(['share'])
    expect(a.spot).toBe('menu:share')
    expect(menuPath('share')).toBe('Файл ▸ Отправить по ссылке…')
    expect(a.how).toContain('Что уходит на сервер: страницы альбома (лица, подписи с именами на разворотах) и название')
    expect(a.how).toContain('Не уходят: пути и имена файлов фото, сведения о съёмке (EXIF, GPS), таблица имён.')
    expect(articleById('export')!.see).toContain('share-link')
  })
})
