// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

// Состояние окна крупного просмотра (128): заморозка списка, выделение и прокрутка на закрытии, шаги, ключ Z.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({}))

const { photoui } = await import('./photoui.svelte')
const { store } = await import('../../base/core/store.svelte')
const { runStep } = await import('../../shell/guide/guidestate.svelte')

const photo = (id: string) => ({ id, name: `${id}.jpg`, w: 3, h: 2 })
const hooks = store.hooks
const hide = (...ids: string[]) => store.plug({ ...hooks, visible: (ps) => ps.filter((p) => !ids.includes(p.id)) })

beforeEach(() => {
  store.photos = ['a', 'b', 'c', 'd', 'e'].map(photo) as never
  store.selected.clear()
  store.anchor = null
  photoui.preview = null
  photoui.reveal = null
})
afterEach(() => {
  store.plug(hooks)
  photoui.preview = null
  photoui.reveal = null
  store.toast = null
})

describe('открытие', () => {
  it('из ленты список замораживается: скрытый позже фильтром кадр остаётся в листании', () => {
    photoui.openLibrary('b')
    hide('c')
    expect(store.visible.map((p) => p.id)).not.toContain('c')
    expect(photoui.preview).toEqual({ ids: ['a', 'b', 'c', 'd', 'e'], at: 1, from: 'library' })
    photoui.step(1)
    expect(photoui.previewId).toBe('c')
  })

  it('фото, которого нет среди видимых, открывается одно', () => {
    hide('b')
    photoui.openLibrary('b')
    expect(photoui.preview).toEqual({ ids: ['b'], at: 0, from: 'library' })
  })

  it('шаг пропускает фото, ушедшее из альбома', () => {
    photoui.openLibrary('a')
    store.photos = ['a', 'c', 'd'].map(photo) as never
    photoui.step(1)
    expect(photoui.previewId).toBe('c')
  })

  it('шаг при закрытом окне ничего не делает', () => {
    photoui.step(1)
    expect(photoui.preview).toBeNull()
  })
})

describe('закрытие из ленты', () => {
  it('показанное вне выделения выделяется одно, якорь — оно; лента просится к нему', () => {
    store.selected.add('a')
    photoui.openLibrary('d')
    photoui.closePreview()
    expect([...store.selected]).toEqual(['d'])
    expect(store.anchor).toBe('d')
    expect(photoui.reveal).toMatchObject({ id: 'd' })
    expect(photoui.preview).toBeNull()
  })

  it('показанное в выделении — выделение из четырёх не трогается', () => {
    for (const id of ['a', 'b', 'c', 'd']) store.selected.add(id)
    store.anchor = 'a'
    photoui.openLibrary('c')
    photoui.step(1)
    photoui.step(-1)
    photoui.closePreview()
    expect([...store.selected].sort()).toEqual(['a', 'b', 'c', 'd'])
    expect(store.anchor).toBe('a')
  })

  it('кадр скрыт фильтром: выделение не трогается, лента идёт к ближайшему видимому', () => {
    store.selected.add('a')
    photoui.openLibrary('c')
    photoui.step(1) // d
    hide('c', 'd')
    photoui.closePreview()
    expect([...store.selected]).toEqual(['a'])
    expect(photoui.reveal).toMatchObject({ id: 'e' }) // равное расстояние (b и e) → следующее
  })

  it('повторная просьба о том же фото — новый номер (эффект ленты сработает снова)', () => {
    photoui.openLibrary('b')
    photoui.closePreview()
    const n = photoui.reveal!.n
    photoui.openLibrary('b')
    photoui.closePreview()
    expect(photoui.reveal).toEqual({ id: 'b', n: n + 1 })
  })

  it('закрыто — ничего: повторный вызов безвреден', () => {
    photoui.closePreview()
    expect(photoui.reveal).toBeNull()
  })
})

describe('закрытие с холста', () => {
  it('выделение ленты и прокрутка не трогаются', () => {
    store.selected.add('a')
    photoui.openCanvas('d')
    photoui.closePreview()
    expect([...store.selected]).toEqual(['a'])
    expect(photoui.reveal).toBeNull()
    expect(photoui.preview).toBeNull()
  })
})

describe('справочник', () => {
  it('шаг «закрыть меню и окна» закрывает просмотр через closePreview: стек окон освобождается эффектом окна', () => {
    photoui.openLibrary('b')
    runStep({ do: 'closeMenus' })
    expect(photoui.preview).toBeNull()
    expect(photoui.reveal).toMatchObject({ id: 'b' }) // лента вернулась к показанному, как при Esc
  })
})
