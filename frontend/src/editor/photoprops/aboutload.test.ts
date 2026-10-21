import { describe, expect, it } from 'vitest'
import { AboutLoader, libPhotoOf, splitUse, usesShown, type About } from './aboutload.svelte'

const about = (id: string, extra: Partial<About> = {}): About =>
  ({ id, file: '', shot: [], note: '', used: 'В альбоме не использовано', uses: [], ...extra }) as About

// Ответ, который отдаётся вручную: проверяем порядок прихода.
function deferred<T>() {
  let resolve!: (v: T) => void
  const promise = new Promise<T>((a) => {
    resolve = a
  })
  return { promise, resolve }
}

describe('libPhotoOf: раздел «Фото из ленты»', () => {
  it('одно выделенное в ленте и открытая панель — id', () => {
    expect(libPhotoOf(['a'], true)).toBe('a')
  })
  it('панель закрыта — раздела нет', () => {
    expect(libPhotoOf(['a'], false)).toBe('')
  })
  it('ни одного и несколько — раздела нет', () => {
    expect(libPhotoOf([], true)).toBe('')
    expect(libPhotoOf(['a', 'b'], true)).toBe('')
  })
  it('то же фото уже в карточке — раздела нет; другое — есть', () => {
    expect(libPhotoOf(['a'], true, 'a')).toBe('')
    expect(libPhotoOf(['a'], true, 'b')).toBe('a')
  })
})

describe('usesShown и splitUse', () => {
  const mk = (n: number) => Array.from({ length: n }, (_, i) => ({ sheet: i + 1, label: `Разворот ${i + 1} — фон` }))
  it('не больше шести ссылок, остальные — «ещё N»', () => {
    const r = usesShown(mk(9))
    expect(r.shown).toHaveLength(6)
    expect(r.more).toBe(3)
  })
  it('ровно шесть и меньше — без «ещё»', () => {
    expect(usesShown(mk(6)).more).toBe(0)
    expect(usesShown(mk(1)).shown).toHaveLength(1)
    expect(usesShown([])).toEqual({ shown: [], more: 0 })
  })
  it('подпись делится на ссылку и вид; без тире вся подпись — ссылка', () => {
    expect(splitUse({ sheet: 2, label: 'Обложка — фон' })).toEqual({ sheet: 2, link: 'Обложка', kind: 'фон' })
    expect(splitUse({ sheet: 2, label: 'Лист 2' })).toEqual({ sheet: 2, link: 'Лист 2', kind: '' })
  })
})

describe('AboutLoader', () => {
  it('поздний ответ прежнего фото отбрасывается', async () => {
    const a = deferred<About>()
    const b = deferred<About>()
    const q = new Map([
      ['a', a],
      ['b', b],
    ])
    const l = new AboutLoader((id) => q.get(id)!.promise)
    const pa = l.load('a')
    const pb = l.load('b')
    b.resolve(about('b'))
    await pb
    a.resolve(about('a'))
    await pa
    expect(l.data?.id).toBe('b')
  })
  it('смена фото очищает данные, перезапрос того же держит прежние до ответа', async () => {
    const d = deferred<About>()
    const l = new AboutLoader(async (id) => (id === 'a' ? about('a') : d.promise))
    await l.load('a')
    expect(l.data?.id).toBe('a')
    const again = l.load('a')
    expect(l.data?.id).toBe('a')
    await again
    const other = l.load('b')
    expect(l.data).toBeNull()
    d.resolve(about('b'))
    await other
    expect(l.data?.id).toBe('b')
  })
  it('перезапрос на правку: ответ с новыми вхождениями заменяет старый', async () => {
    let n = 0
    const l = new AboutLoader(async (id) => about(id, { used: `n${++n}` }))
    await l.load('a')
    await l.load('a')
    expect(l.data?.used).toBe('n2')
  })
  it('ошибка — данных нет, без исключения; пустой id — запроса нет', async () => {
    let calls = 0
    const l = new AboutLoader(async () => {
      calls++
      throw new Error('boom')
    })
    await l.load('a')
    expect(l.data).toBeNull()
    await l.load('')
    expect(calls).toBe(1)
  })
  it('sync: запрос на смену фото и на смену ревизии, на тот же ключ — нет', async () => {
    const calls: string[] = []
    const l = new AboutLoader(async (id) => {
      calls.push(id)
      return about(id)
    })
    await l.sync('a', 1)
    await l.sync('a', 1)
    expect(calls).toEqual(['a'])
    await l.sync('a', 2) // правка альбома
    await l.sync('b', 2) // другое фото
    await l.sync('a', 2) // назад к прежнему фото
    expect(calls).toEqual(['a', 'a', 'b', 'a'])
  })
  it('sync: пустой id запроса не делает, данные сброшены', async () => {
    let calls = 0
    const l = new AboutLoader(async (id) => {
      calls++
      return about(id)
    })
    await l.sync('a', 1)
    await l.sync('', 1)
    expect(calls).toBe(1)
    expect(l.data).toBeNull()
  })
})
