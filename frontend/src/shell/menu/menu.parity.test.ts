import { writeFileSync } from 'node:fs'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import snapshot from '../../../../testdata/menu-enabled.json'
import { dropCatalog, useCatalog } from '../../base/keys/keymap.fixture'
import { FEATURES, can, type Custom, type Level } from '../../base/prefs/levels'
import { features } from '../../base/prefs/levelcustom'
import { buildMenu, type MenuCtx, type MenuItem } from './menu'

// Снимок меню (124 U0, 0.26-124-26): id → { disabled, keys, icon } каждого пункта всех меню на явных контекстах.
// Снят ДО переноса пунктов в «Разворот»: после U1 тест только сверяет, что id, клавиши, значки и условия серого
// прежние. Меню пункта здесь не проверяется — это menu.spread.test.ts. Пункт, скрытый уровнем, в снимке отсутствует.
// Правится только вместе со спекой; перезапись — SA_WRITE_SNAPSHOT=1 npx vitest run src/shell/menu/menu.parity.test.ts.
beforeEach(() => useCatalog())
afterEach(() => dropCatalog())

// Всё доступно и включено: от этого контекста флаги гасятся по одному.
const on: MenuCtx = {
  spreads: 3,
  hasSpread: true,
  canUndo: true,
  canRedo: true,
  zone: 'canvas',
  libraryPhotos: 10,
  anySelection: true,
  deletable: true,
  flippable: true,
  allGray: false,
  sticker: false,
  varyable: true,
  vigable: true,
  decorable: true,
  vigSetup: true,
  copyable: true,
  pasteable: true,
  layerable: true,
  missing: 2,
  big: false,
  details: false,
  safe: false,
  markup: false,
  rulers: true,
  snap: true,
  binShown: true,
  alignCount: 3,
}

const noFeatures: Custom = { base: 'simple', on: [], off: FEATURES.map((f) => f.id) }
const atLevel = (level: Level, o: Partial<MenuCtx> = {}): MenuCtx => ({
  ...on,
  level,
  can: level === 'custom' ? (f) => features('custom', noFeatures).has(f) : (f) => can(f, level),
  ...o,
})
const full = (o: Partial<MenuCtx>): MenuCtx => atLevel('full', o)

// Явные контексты (имя → контекст): 4 уровня × всё включено, затем «Полный» с одним выключенным флагом.
const CONTEXTS: Record<string, MenuCtx> = {
  'простой, всё включено': atLevel('simple'),
  'стандартный, всё включено': atLevel('standard'),
  'полный, всё включено': atLevel('full'),
  'свой без функций, всё включено': atLevel('custom'),
  'простой в ленте': atLevel('simple', { zone: 'library' }),
  'стандартный, ничего не выделено': atLevel('standard', { anySelection: false, deletable: false, flippable: false, copyable: false }),
  'полный, нет разворота': full({ hasSpread: false, decorable: false, vigable: false, varyable: false, vigSetup: false, copyable: false }),
  'полный, нет отмены': full({ canUndo: false }),
  'полный, нет повтора': full({ canRedo: false }),
  'полный, нечего удалять': full({ deletable: false }),
  'полный, нет выделения': full({ anySelection: false }),
  'полный, не фото': full({ flippable: false }),
  'полный, ч/б': full({ allGray: true }),
  'полный, стикер': full({ flippable: false, sticker: true }),
  'полный, нечего разнообразить': full({ varyable: false }),
  'полный, нельзя виньетку': full({ vigable: false }),
  'полный, нет оформления (обложка)': full({ decorable: false }),
  'полный, не виньетка': full({ vigSetup: false }),
  'полный, нечего копировать': full({ copyable: false }),
  'полный, буфер пуст': full({ pasteable: false }),
  'полный, слои недоступны': full({ layerable: false }),
  'полный, нет выравнивания': full({ alignCount: 0 }),
  'полный, лента': full({ zone: 'library' }),
  'полный, лента без фото': full({ zone: 'library', libraryPhotos: 0 }),
  'полный, нет пропавших и разворотов': full({ missing: 0, spreads: 0 }),
  'полный, без подборки, сбор идёт': full({ binShown: false, collecting: true }),
  'полный, всё выключено': full({
    hasSpread: false,
    canUndo: false,
    canRedo: false,
    anySelection: false,
    deletable: false,
    flippable: false,
    varyable: false,
    vigable: false,
    decorable: false,
    vigSetup: false,
    copyable: false,
    pasteable: false,
    layerable: false,
    alignCount: 0,
    libraryPhotos: 0,
  }),
}

interface Row {
  disabled: boolean
  keys?: string
  icon?: string
}

// id → строка по всем пунктам всех меню и подменю; повтор id — ошибка (иначе снимок скроет потерю).
function rows(c: MenuCtx): Record<string, Row> {
  const out: Record<string, Row> = {}
  const walk = (items: MenuItem[]) => {
    for (const it of items) {
      if (out[it.id]) throw new Error(`id пункта встретился дважды: ${it.id}`)
      out[it.id] = { disabled: !!it.disabled, keys: it.keys, icon: it.icon }
      if (it.sub) walk(it.sub)
    }
  }
  for (const m of buildMenu(c)) walk(m.items)
  return JSON.parse(JSON.stringify(out)) // undefined в снимке не пишем
}

const now = (): Record<string, Record<string, Row>> => Object.fromEntries(Object.entries(CONTEXTS).map(([name, c]) => [name, rows(c)]))

// Один пункт — одна строка: снимок читается и сверяется построчно в диффе.
const text = (all: Record<string, Record<string, Row>>): string =>
  '{\n' +
  Object.entries(all)
    .map(
      ([name, r]) =>
        ` ${JSON.stringify(name)}: {\n` +
        Object.entries(r)
          .map(([id, row]) => `  ${JSON.stringify(id)}: ${JSON.stringify(row)}`)
          .join(',\n') +
        '\n }',
    )
    .join(',\n') +
  '\n}\n'

describe('снимок меню: id → { disabled, keys, icon } (124 U0)', () => {
  it('на запись: SA_WRITE_SNAPSHOT=1 пишет testdata/menu-enabled.json', () => {
    if (!process.env.SA_WRITE_SNAPSHOT) return
    const url = new URL('../../../../testdata/menu-enabled.json', import.meta.url)
    writeFileSync(url, text(now()))
  })

  it('контекстов 20–30', () => {
    expect(Object.keys(CONTEXTS).length).toBeGreaterThanOrEqual(20)
    expect(Object.keys(CONTEXTS).length).toBeLessThanOrEqual(30)
  })

  const snap = snapshot as unknown as Record<string, Record<string, Row>>
  for (const [name, c] of Object.entries(CONTEXTS)) {
    it(name, () => {
      expect(snap[name], 'контекста нет в снимке').toBeDefined()
      expect(rows(c)).toEqual(snap[name])
    })
  }

  it('в снимке нет лишних контекстов', () => {
    expect(Object.keys(snap).sort()).toEqual(Object.keys(CONTEXTS).sort())
  })
})
