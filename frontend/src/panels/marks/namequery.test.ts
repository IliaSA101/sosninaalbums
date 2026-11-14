import { describe, expect, it } from 'vitest'
import { buildIndex, counters, hiddenLabel, hiddenToks, matchIds, missLabel, parseQuery, shortQuery } from './namequery'

// Имена камер и программ; id = имя, чтобы читать ответ глазами.
const NAMES = [
  'IMG_0798.JPG',
  'IMG_1798.JPG',
  'IMG_0801.JPG',
  'IMG_0805.JPG',
  '_MG_0798.CR2',
  'DSC_0798.NEF',
  'DSC07798.ARW',
  'DSCF0798.RAF',
  'P1027798.JPG',
  '2Q4A1234.CR3',
  '2024-10-12_0012.jpg',
  '2024-10-12_0798.jpg',
  'Ivanovy2024-12.jpg',
  'Ivanovy2024-13.jpg',
  'Svadba 2024 - 12.jpg',
  'IMG_1234.jpg',
  'IMG_1234-2.jpg',
  'Свадьба_015.jpg',
  'Свадьба Ивановых 016.jpg',
  'Ёлка_017.jpg',
  'Мама.jpg',
]
const idx = buildIndex(NAMES.map((name) => ({ id: name, name })))
const find = (q: string) => {
  const r = matchIds(idx, parseQuery(q))
  return { found: NAMES.filter((n) => r.ids.has(n)), miss: r.miss }
}
const found = (q: string) => find(q).found

describe('counters: номер кадра в имени', () => {
  it.each([
    ['img_0798', ['0798']],
    ['_mg_0798', ['0798']],
    ['dsc_0798', ['0798']],
    ['dsc07798', ['07798']],
    ['dscf0798', ['0798']],
    ['p1027798', ['1027798']],
    ['2q4a1234', ['1234']],
    ['2024-10-12_0012', ['0012']],
    ['ivanovy-12', ['12']],
    ['ivanovy2024-12', ['2024', '12']],
    ['img_1234-2', ['1234', '2']],
    ['мама', []],
  ])('%s', (base, want) => expect(counters(base)).toEqual(want))
})

describe('parseQuery', () => {
  it('пусто и пробелы — нет токенов', () => {
    expect(parseQuery('')).toEqual([])
    expect(parseQuery('  \n ')).toEqual([])
    expect(parseQuery(' , ; ')).toEqual([])
  })

  it('регистр и ё/е, расширение отбрасывается', () => {
    expect(parseQuery('ЁЛКА').map((t) => t.raw)).toEqual(['елка'])
    expect(parseQuery('IMG_0798.CR2').map((t) => t.raw)).toEqual(['img_0798'])
    expect(parseQuery('2024.5').map((t) => t.raw)).toEqual(['2024.5'])
    expect(parseQuery('.jpg')).toEqual([])
  })

  it('список: запятая, точка с запятой, перевод строки', () => {
    expect(parseQuery('798, 801;805\n810').map((t) => t.raw)).toEqual(['798', '801', '805', '810'])
  })

  it('список по пробелу — если в каждой части есть цифра; фраза остаётся фразой', () => {
    expect(parseQuery('798 801 IMG_0805').map((t) => t.raw)).toEqual(['798', '801', 'img_0805'])
    expect(parseQuery('Свадьба 2024').map((t) => t.raw)).toEqual(['свадьба 2024'])
    expect(parseQuery('Свадьба Ивановых').map((t) => t.raw)).toEqual(['свадьба ивановых'])
    expect(parseQuery('IMG 0798').map((t) => t.raw)).toEqual(['img 0798'])
  })

  it('число — числовой токен; кириллица получает латинскую раскладку', () => {
    expect(parseQuery('0798')).toEqual([{ raw: '0798', lat: '', num: true }])
    expect(parseQuery('шьп')).toEqual([{ raw: 'шьп', lat: 'img', num: false }])
    expect(parseQuery('img')[0].lat).toBe('')
  })
})

describe('matchIds: номера', () => {
  it('без ведущих нулей: 798 — IMG_0798, не IMG_1798', () => {
    expect(found('798')).toEqual(['IMG_0798.JPG', '_MG_0798.CR2', 'DSC_0798.NEF', 'DSCF0798.RAF', '2024-10-12_0798.jpg'])
  })

  it('с нулями и без — одно и то же', () => {
    expect(found('0798')).toEqual(found('798'))
  })

  it('от 4 цифр — ещё и конец номера: 7798 — P1027798 и DSC07798', () => {
    expect(found('7798')).toEqual(['DSC07798.ARW', 'P1027798.JPG'])
  })

  it('3 цифры — только равенство: конец 1027798 по «798» не ищется', () => {
    expect(found('798')).not.toContain('P1027798.JPG')
  })

  it('2Q4A: номер после буквенного префикса', () => {
    expect(found('1234')).toEqual(['2Q4A1234.CR3', 'IMG_1234.jpg', 'IMG_1234-2.jpg'])
  })

  it('дата-номер: «12» — день в конце имени, не год', () => {
    expect(found('12')).toEqual(['2024-10-12_0012.jpg', 'Ivanovy2024-12.jpg', 'Svadba 2024 - 12.jpg'])
  })

  it('«Ivanovy2024-12»: два счётчика — 2024 и 12', () => {
    expect(found('2024')).toEqual(['Ivanovy2024-12.jpg', 'Ivanovy2024-13.jpg', 'Svadba 2024 - 12.jpg'])
  })

  it('копия «-2» находится по «2», но не по «02» чужих имён', () => {
    expect(found('2')).toEqual(['IMG_1234-2.jpg'])
  })
})

describe('matchIds: текст и список', () => {
  it('подстрока, регистр и ё/е не важны', () => {
    expect(found('img_08')).toEqual(['IMG_0801.JPG', 'IMG_0805.JPG'])
    expect(found('ЕЛКА')).toEqual(['Ёлка_017.jpg'])
    expect(found('ёлка')).toEqual(['Ёлка_017.jpg'])
    expect(found('свадьба')).toEqual(['Свадьба_015.jpg', 'Свадьба Ивановых 016.jpg'])
  })

  it('русская раскладка: «шьп» ищется и как набрано, и как латиница', () => {
    expect(found('шьп_08')).toEqual(['IMG_0801.JPG', 'IMG_0805.JPG'])
    expect(found('шьп')).toContain('IMG_0798.JPG')
    expect(found('мама')).toEqual(['Мама.jpg'])
  })

  it('фраза целиком, а не по словам', () => {
    expect(found('Свадьба Ивановых')).toEqual(['Свадьба Ивановых 016.jpg'])
  })

  it('«Свадьба 2024» — не год как номер: ничего', () => {
    expect(find('Свадьба 2024')).toEqual({ found: [], miss: ['свадьба 2024'] })
  })

  it('расширение в запросе не мешает: «IMG_0798.CR2» ищет IMG_0798', () => {
    expect(found('IMG_0798.CR2')).toEqual(['IMG_0798.JPG'])
  })

  it('список: любой из токенов', () => {
    expect(found('801, 805')).toEqual(['IMG_0801.JPG', 'IMG_0805.JPG'])
    expect(found('798 801 IMG_0805')).toEqual(expect.arrayContaining(['IMG_0798.JPG', 'IMG_0801.JPG', 'IMG_0805.JPG']))
  })
})

describe('matchIds: «IMG 0798», «#798» — по числу как номеру (0.26-124-21)', () => {
  it.each(['IMG 0798', 'img0798', '#798', '№798', 'фото 798'])('%s', (q) => {
    expect(found(q)).toContain('IMG_0798.JPG')
    expect(found(q)).not.toContain('IMG_1798.JPG')
  })

  it('«DSC_0798» при кадре IMG_0798 — подсказка: находит кадры с этим номером', () => {
    expect(found('DSC_0798.jpg')).toContain('DSC_0798.NEF')
  })

  it('подстрока главнее: второго прохода нет, если подстрока нашла', () => {
    expect(found('img_0798')).toEqual(['IMG_0798.JPG'])
  })
})

describe('matchIds: «нет»', () => {
  it('ненайденные токены попадают в miss', () => {
    expect(find('798, 7811, 7815')).toMatchObject({ miss: ['7811', '7815'] })
    expect(find('zzz').miss).toEqual(['zzz'])
  })

  it('пустой запрос — ничего не ищет', () => {
    expect(matchIds(idx, [])).toEqual({ ids: new Set(), miss: [], hits: [] })
  })

  it('5+ цифр — только конец счётчика: 17798 не находит DSC07798 и P1027798', () => {
    expect(found('17798')).toEqual([])
    expect(found('27798')).toEqual(['P1027798.JPG'])
    expect(found('07798')).toEqual(['DSC07798.ARW'])
  })

  it('пустая библиотека', () => {
    expect(matchIds(buildIndex([]), parseQuery('798'))).toEqual({ ids: new Set(), miss: ['798'], hits: [] })
  })
})

describe('missLabel и shortQuery', () => {
  it('до трёх — списком, больше — числом; список в подсказке', () => {
    expect(missLabel([])).toEqual({ text: '', title: '' })
    expect(missLabel(['7811', '7815'])).toEqual({ text: '· нет: 7811, 7815', title: 'Не найдено: 7811, 7815' })
    expect(missLabel(['1', '2', '3']).text).toBe('· нет: 1, 2, 3')
    expect(missLabel(['1', '2', '3', '4', '5'])).toEqual({ text: '· нет 5', title: 'Не найдено: 1, 2, 3, 4, 5' })
  })

  it('hiddenLabel: «· скрыто: 805», больше трёх — числом; hiddenToks — токены без кадров в ленте', () => {
    expect(hiddenLabel([])).toEqual({ text: '', title: '' })
    expect(hiddenLabel(['805']).text).toBe('· скрыто: 805')
    expect(hiddenLabel(['1', '2', '3', '4']).text).toBe('· скрыто 4')
    const hits = [
      { raw: '805', ids: ['a'] },
      { raw: '798', ids: ['b', 'c'] },
    ]
    expect(hiddenToks(hits, new Set(['c']))).toEqual(['805'])
    expect(hiddenToks(hits, new Set())).toEqual(['805', '798'])
  })

  it('длинный запрос в тексте усечён', () => {
    expect(shortQuery('  a  ')).toBe('a')
    expect(shortQuery('x'.repeat(100))).toBe('x'.repeat(60) + '…')
  })
})

describe('время', () => {
  const slow = Number(process.env.SA_TEST_SLOW)
  const k = Number.isFinite(slow) && slow >= 1 ? slow : 1
  // Время процессора, а не стенные часы: на занятой машине поток ждёт очереди, и стенное время меряет чужую нагрузку.
  const cpuMs = () => {
    const u = process.cpuUsage()
    return (u.user + u.system) / 1000
  }
  // 20 000 имён четырёх видов (design §8.3). Суффикс делает имена новыми для кэша разбора: каждая сборка холодная.
  const names = (tag: string) =>
    Array.from({ length: 20000 }, (_, i) => ({
      id: 'p' + i,
      name: ['IMG_', 'DSC_', '2024-10-12_', 'Ivanovy2024-'][i % 4] + String(i % 10000).padStart(4, '0') + tag + '.JPG',
    }))

  it('индекс ≤ 60 мс, 40 номеров ≤ 10 мс', () => {
    // Лучший из пяти: один замер может поймать сборку мусора или прогрев JIT.
    let built = Infinity
    let ix = buildIndex([])
    for (const tag of ['', 'a', 'b', 'c', 'd']) {
      const list = names(tag)
      const t0 = cpuMs()
      ix = buildIndex(list)
      built = Math.min(built, cpuMs() - t0)
    }
    const toks = parseQuery(Array.from({ length: 40 }, (_, i) => String(100 + i * 37)).join(', '))
    let looked = Infinity
    let size = 0
    for (let r = 0; r < 5; r++) {
      const t1 = cpuMs()
      size = matchIds(ix, toks).ids.size
      looked = Math.min(looked, cpuMs() - t1)
    }
    expect(size).toBeGreaterThan(0)
    expect(built).toBeLessThanOrEqual(60 * k)
    expect(looked).toBeLessThanOrEqual(10 * k)
  })
})
