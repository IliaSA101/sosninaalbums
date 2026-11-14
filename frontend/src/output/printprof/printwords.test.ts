import { describe, expect, it } from 'vitest'
import type { PrintCheckRow, PrintEditCheck, PrintInfo } from '../../base/core/types'
import {
  FIELD_KEYS,
  baseGroups,
  fieldErr,
  formErr,
  isHitLine,
  isOwnId,
  listLabel,
  mmFmt,
  ownLabel,
  pageCell,
  planLabel,
  removeQuestion,
  rowBad,
  shareHint,
  sideCell,
  spineNote,
} from './printwords'

const info = (id: string, name: string, source: PrintInfo['source']): PrintInfo => ({
  id,
  name,
  source,
  version: 1,
  hasExport: true,
  formats: [],
})
const infos = [
  info('fabrika', 'Фабрика Фотокниги', 'builtin'),
  info('fsfera', 'Фотосфера', 'builtin'),
  info('lib-1', 'Из набора', 'library'),
  info('own-1', 'Альфа', 'own'),
  info('own-2', 'Бета (своя)', 'own'),
]
const check = (o: Partial<PrintEditCheck>): PrintEditCheck => ({ ok: false, errs: {}, formats: [], spine: 0, lines: [], names: '', ...o })

describe('«(своя)» в списках', () => {
  it('ownLabel дописывает пометку один раз', () => {
    expect(ownLabel('Альфа')).toBe('Альфа (своя)')
    expect(ownLabel('Альфа (своя)')).toBe('Альфа (своя)')
    expect(ownLabel('Альфа (своя 2)')).toBe('Альфа (своя 2)')
  })
  it('listLabel: только у своей; встроенная и неизвестное имя — как есть', () => {
    expect(listLabel('Альфа', infos)).toBe('Альфа (своя)')
    expect(listLabel('Бета (своя)', infos)).toBe('Бета (своя)')
    expect(listLabel('Фабрика Фотокниги', infos)).toBe('Фабрика Фотокниги')
    expect(listLabel('Из набора', infos)).toBe('Из набора')
    expect(listLabel('', infos)).toBe('')
  })
  it('planLabel: «Для «Альфа»» → с пометкой после; встроенная без неё', () => {
    expect(planLabel({ kind: 'own-1', title: 'Для «Альфа»' }, infos)).toBe('Для «Альфа» (своя)')
    expect(planLabel({ kind: 'own-2', title: 'Для «Бета (своя)»' }, infos)).toBe('Для «Бета (своя)»')
    expect(planLabel({ kind: 'fabrika', title: 'Для Фабрики Фотокниги' }, infos)).toBe('Для Фабрики Фотокниги')
  })
  it('пустой реестр (ещё не прочитан) — названия без пометки', () => {
    expect(listLabel('Альфа', [])).toBe('Альфа')
    expect(planLabel({ kind: 'own-1', title: 'Для «Альфа»' }, [])).toBe('Для «Альфа»')
  })
})

describe('основа и своя', () => {
  it('baseGroups: встроенные · из наборов · свои, пустые группы не нужны', () => {
    expect(baseGroups(infos).map((g) => [g.label, g.items.length])).toEqual([
      ['Встроенные', 2],
      ['Из наборов', 1],
      ['Свои', 2],
    ])
    expect(baseGroups(infos.filter((i) => i.source === 'builtin')).map((g) => g.label)).toEqual(['Встроенные'])
    expect(baseGroups([])).toEqual([])
  })
  it('isOwnId: только source own', () => {
    expect(isOwnId(infos, 'own-1')).toBe(true)
    expect(isOwnId(infos, 'fabrika')).toBe(false)
    expect(isOwnId(infos, '')).toBe(false)
    expect(isOwnId(infos, 'нет')).toBe(false)
  })
})

describe('числа и подписи строки формата', () => {
  const r: PrintCheckRow = { pageW: 303.95, pageH: 202.95, sideW: 310, sideH: 206, err: '', field: '', hit: false }
  it('mmFmt: сотые и запятая, целое без хвоста', () => {
    expect(mmFmt(303.95)).toBe('303,95')
    expect(mmFmt(304)).toBe('304')
    expect(mmFmt(202.9500000001)).toBe('202,95')
  })
  it('pageCell / sideCell', () => {
    expect(pageCell(r)).toBe('= 303,95 × 202,95 мм')
    expect(pageCell(undefined)).toBe('= —')
    expect(pageCell({ ...r, pageW: 0 })).toBe('= —')
    expect(sideCell(r, 7866, 2835)).toBe('= 310 × 206 мм')
    expect(sideCell({ ...r, sideW: 0 }, 0, 0)).toBe('без обложки')
    expect(sideCell({ ...r, sideW: 0 }, 5409, 0)).toBe('= —')
    expect(sideCell(undefined, 0, 0)).toBe('без обложки')
  })
  it('spineNote: нет основы — пусто', () => {
    expect(spineNote(12)).toBe(', корешок 12 мм')
    expect(spineNote(0)).toBe('')
  })
  it('красное поле строки: по признаку field от Go, текст ошибки не разбирается', () => {
    const row = (field: PrintCheckRow['field'], err = 'любой текст'): PrintCheckRow => ({ ...r, err, field })
    expect(rowBad(row('name', 'Укажите число'), 'name')).toBe(true)
    expect(rowBad(row('name'), 'px')).toBe(false)
    expect(rowBad(row('name'), 'cover')).toBe(false)
    // слово «подпись» в тексте ничего не решает
    expect(rowBad(row('px', 'Подпись повторена'), 'name')).toBe(false)
    expect(rowBad(row('px'), 'px')).toBe(true)
    expect(rowBad(row('px'), 'cover')).toBe(false)
    // обложка подсвечивается только полями обложки
    expect(rowBad(row('cover'), 'cover')).toBe(true)
    expect(rowBad(row('cover'), 'px')).toBe(false)
    expect(rowBad(row('cover'), 'name')).toBe(false)
    expect(rowBad(row(''), 'name')).toBe(false)
    expect(rowBad(row(''), 'px')).toBe(false)
    expect(rowBad(row(''), 'cover')).toBe(false)
    expect(rowBad(undefined, 'name')).toBe(false)
  })
  it('isHitLine: зелёная только lines[0] и только при hit хотя бы у одной строки', () => {
    const chk = (...hits: boolean[]): PrintEditCheck => ({
      ok: true,
      errs: {},
      formats: hits.map((hit) => ({ ...r, hit })),
      spine: 0,
      lines: ['Сейчас альбом даёт: разворот 7181 × 2398 px при 300 dpi', 'другая строка'],
      names: '',
    })
    expect(isHitLine(chk(false, true), 0)).toBe(true)
    expect(isHitLine(chk(true), 0)).toBe(true)
    expect(isHitLine(chk(false, true), 1)).toBe(false)
    expect(isHitLine(chk(false, false), 0)).toBe(false)
    expect(isHitLine(chk(), 0)).toBe(false)
    expect(isHitLine(null, 0)).toBe(false)
    // текст строки не разбирается: «— это формат» без hit зелёной не делает
    const textOnly = { ...chk(false), lines: ['Сейчас альбом даёт: … — это формат «30×20»'] }
    expect(isHitLine(textOnly, 0)).toBe(false)
  })
})

describe('ошибки Go под полями', () => {
  it('fieldErr: пусто, если расчёта ещё нет', () => {
    expect(fieldErr(null, 'name')).toBe('')
    expect(fieldErr(check({ errs: { name: 'Введите название' } }), 'name')).toBe('Введите название')
    expect(fieldErr(check({ errs: { name: 'x' } }), 'dpi')).toBe('')
  })
  it('formErr: form и неизвестные ключи — общей строкой; поля окна — нет', () => {
    expect(formErr(null)).toBe('')
    expect(formErr(check({ errs: { name: 'А', dpi: 'Б' } }))).toBe('')
    expect(formErr(check({ errs: { form: 'В одном слове буквы разных алфавитов' } }))).toBe('В одном слове буквы разных алфавитов')
    expect(formErr(check({ errs: { form: 'Нет видимых знаков', 'products.0': 'Адрес на другом сайте', url: 'У' } }))).toBe(
      'Нет видимых знаков. Адрес на другом сайте',
    )
    expect(FIELD_KEYS).toContain('url')
  })
  it('вопрос об удалении называет id из карточки обложки', () => {
    expect(removeQuestion('Альфа', 'own-3f9a1c2e')).toBe(
      'Удалить типографию «Альфа»? Альбомы, где она выбрана, не изменятся: в карточке обложки будет написано «Типография не найдена: own-3f9a1c2e». Отменить удаление нельзя.',
    )
    expect(shareHint('Альфа')).toBe('Записать «Альфа.saprint»')
  })
})
