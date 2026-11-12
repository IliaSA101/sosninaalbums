import { describe, expect, it } from 'vitest'
import { allLink, blobType, busyLine, getAction, hasVideo, percent, rowState, secondsLabel, type VideoItem } from './videorow'

// Строка «Ролик» (спека 140, design §4): состояния и тексты — без Svelte и сети.
const MB = 1048576
const v = (id: string, o: Partial<VideoItem> = {}): VideoItem => ({ id, version: 1, seconds: 48, size: MB / 2, ready: false, ...o })
const none = new Set<string>()

describe('rowState', () => {
  it('скачан или нет файла — всегда «Смотреть» (нет файла: окно загрузит его само)', () => {
    expect(rowState('cover', v('cover'), [], none)).toBe('watch')
    expect(rowState('cover', undefined, [], none)).toBe('watch') // встроенный список без каталога
    expect(rowState('cover', v('cover', { ready: true }), [], none)).toBe('watch')
  })
  it('идёт скачивание этого ролика вне окна (одного или в «все ролики») — busy; чужое — нет', () => {
    expect(rowState('cover', v('cover'), ['cover'], none)).toBe('busy')
    expect(rowState('cover', v('cover'), ['manual', 'cover'], none)).toBe('busy')
    expect(rowState('cover', v('cover'), ['manual'], none)).toBe('watch')
  })
  it('ролик грузится в окне ролика — строка под окном остаётся «Смотреть»; чужое окно её не трогает', () => {
    expect(rowState('cover', v('cover'), ['cover'], none, 'cover')).toBe('watch')
    expect(rowState('cover', v('cover'), ['cover'], none, 'manual')).toBe('busy')
  })
  it('окно не сыграло скачанный ролик — broken; после ошибки загрузки (не скачан) — снова «Смотреть»', () => {
    expect(rowState('cover', v('cover', { ready: true }), [], new Set(['cover']))).toBe('broken')
    expect(rowState('cover', v('cover'), [], new Set(['cover']))).toBe('watch')
  })
  it('ролик пересняли (ready=false у нового) — «Смотреть» снова загрузит новый', () => {
    expect(rowState('cover', v('cover', { version: 2 }), [], none)).toBe('watch')
  })
})

describe('согласие на сеть', () => {
  it('без согласия загрузка спрашивает, с согласием идёт', () => {
    expect(getAction(false)).toBe('ask')
    expect(getAction(true)).toBe('download')
  })
})

describe('hasVideo', () => {
  it('статья с роликом — по встроенному списку или по каталогу; иначе строки нет', () => {
    const items = new Map([['manual', v('manual')]])
    expect(hasVideo('cover', { cover: {} }, items)).toBe(true)
    expect(hasVideo('manual', {}, items)).toBe(true)
    expect(hasVideo('export', { cover: {} }, items)).toBe(false)
    expect(hasVideo('export', {}, new Map())).toBe(false)
  })
})

describe('тексты', () => {
  it('секунды: «48 с», «1 мин», «1 мин 5 с»', () => {
    expect(secondsLabel(48)).toBe('48 с')
    expect(secondsLabel(60)).toBe('1 мин')
    expect(secondsLabel(65)).toBe('1 мин 5 с')
  })
  it('ход: «0,3 из 0,5 МБ»; без размера или чужой ролик — «Скачиваем…»', () => {
    const p = { id: 'cover', done: Math.round(0.3 * MB), total: MB / 2 }
    expect(busyLine(p, 'cover')).toBe('Скачиваем… 0,3 из 0,5 МБ')
    expect(busyLine(p, 'manual')).toBe('Скачиваем…')
    expect(busyLine({ id: 'cover', done: 5, total: 0 }, 'cover')).toBe('Скачиваем…')
    expect(busyLine(null, 'cover')).toBe('Скачиваем…')
    expect(busyLine({ id: 'cover', done: 2 * MB, total: MB }, 'cover')).toBe('Скачиваем… 1,0 из 1,0 МБ') // done не больше total
  })
  it('доля полосы: 0…100, неопределённая без размера', () => {
    expect(percent({ id: 'a', done: 50, total: 200 }, 'a')).toBe(25)
    expect(percent({ id: 'a', done: 500, total: 200 }, 'a')).toBe(100)
    expect(percent({ id: 'a', done: 5, total: 0 }, 'a')).toBeNull()
    expect(percent({ id: 'a', done: 5, total: 10 }, 'b')).toBeNull()
    expect(percent(null, 'a')).toBeNull()
  })
})

describe('allLink', () => {
  it('«все ролики (3, 1,5 МБ)» — не скачанные; готовые не считаются', () => {
    const items = [v('a'), v('b'), v('c'), v('d', { ready: true })]
    expect(allLink(items)).toEqual({ ids: ['a', 'b', 'c'], label: 'все ролики (3, 1,5 МБ)' })
  })
  it('не скачан один или ни одного — ссылки нет', () => {
    expect(allLink([v('a'), v('b', { ready: true })])).toBeNull()
    expect(allLink([v('a')])).toBeNull()
    expect(allLink([])).toBeNull()
  })
  it('размер неизвестен хоть у одного — без размера', () => {
    expect(allLink([v('a'), v('b', { size: 0 })])?.label).toBe('все ролики (2)')
  })
})

describe('blobType', () => {
  it('только webm и mp4; параметры и регистр не мешают, прочее — пусто', () => {
    expect(blobType('video/webm')).toBe('video/webm')
    expect(blobType('Video/MP4; codecs=avc1')).toBe('video/mp4')
    expect(blobType('text/html')).toBe('')
    expect(blobType('')).toBe('')
    expect(blobType(null)).toBe('')
  })
})
