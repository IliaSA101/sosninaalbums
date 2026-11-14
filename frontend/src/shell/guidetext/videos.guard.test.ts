import { describe, expect, it } from 'vitest'
import { ARTICLES } from './articles'
import { articleHash, fnv1a32 } from './videohash'
import { GUIDE_VIDEOS } from './videos'

// Страж роликов (спека 140, design §6): запись GUIDE_VIDEOS — только для существующей статьи, с хешем её текста
// на момент съёмки. Текст статьи изменился — ролик «переснять или обновить хеш с причиной в коммите».

describe('GUIDE_VIDEOS', () => {
  const ids = Object.keys(GUIDE_VIDEOS)

  it('каждая запись — существующая статья', () => {
    const known = new Set(ARTICLES.map((a) => a.id))
    expect(ids.filter((id) => !known.has(id))).toEqual([])
  })

  it('длина — целые секунды 1…600 (как в каталоге), хеш — 8 знаков hex', () => {
    for (const id of ids) {
      const v = GUIDE_VIDEOS[id]
      expect(Number.isInteger(v.seconds) && v.seconds >= 1 && v.seconds <= 600, `${id}: seconds ${v.seconds}`).toBe(true)
      expect(v.hash, id).toMatch(/^[0-9a-f]{8}$/)
    }
  })

  it('хеш совпадает с текстом статьи: иначе переснять ролик или обновить хеш с причиной', () => {
    const stale = ids.filter((id) => articleHash(ARTICLES.find((a) => a.id === id)!) !== GUIDE_VIDEOS[id].hash)
    expect(stale, `текст статей изменился после съёмки: ${stale.join(', ')} — переснять или обновить хеш`).toEqual([])
  })

  it('articleHash: FNV-1a 32 по title\\nwhat\\nwhere\\nhow, пустое поле — пустая строка', () => {
    expect(fnv1a32('')).toBe(0x811c9dc5)
    expect(fnv1a32('a')).toBe(0xe40c292c)
    expect(articleHash({ title: 'a' })).toBe(fnv1a32('a\n\n\n').toString(16).padStart(8, '0'))
    expect(articleHash({ title: 'a', what: 'b', where: 'c', how: 'd' })).not.toBe(
      articleHash({ title: 'a', what: 'b', where: 'd', how: 'c' }),
    )
    expect(articleHash({ title: 'Ё' })).toMatch(/^[0-9a-f]{8}$/) // не ASCII — по байтам UTF-8
  })
})
