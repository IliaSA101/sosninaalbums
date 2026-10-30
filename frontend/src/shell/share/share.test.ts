// @vitest-environment happy-dom
import { beforeEach, describe, expect, it } from 'vitest'
import {
  DAYS_ITEMS,
  DEFAULT_DAYS,
  blockText,
  hasLink,
  loadDays,
  normDays,
  runLine,
  runPercent,
  saveDays,
  screenOf,
  summaryLine,
} from './share'
import type { ShareProgress, ShareView } from './shareapi'
import { stubView, type Scenario } from './sharestub'
import { BUSY_OTHER, dataHint } from './sharetext'

// Чистая логика окна «Отправить по ссылке» (146): экран по ShareView, строки хода, сводка, срок.
const SCREEN: Record<Scenario, string> = {
  unplugged: 'connect',
  plugged: 'form',
  running: 'run',
  published: 'link',
  changed: 'link',
  incomplete: 'incomplete',
  expired: 'expired',
  gone: 'expired',
  other: 'other',
  unsaved: 'blocked',
  busyOther: 'busyOther',
}

describe('экран по состоянию Go (design §3.4)', () => {
  for (const [kind, screen] of Object.entries(SCREEN))
    it(`${kind} → ${screen}`, () => {
      expect(screenOf(stubView(kind as Scenario))).toBe(screen)
    })

  it('нет токена при имени сервера — подключение; ссылка без подключения только показывается', () => {
    const v: ShareView = stubView('published')
    v.service.token = false
    expect(screenOf(v)).toBe('connect')
    expect(hasLink(v)).toBe(true)
    v.service.host = ''
    expect(screenOf(v)).toBe('connect')
  })

  it('ход важнее причины, причина важнее подключения', () => {
    const v = stubView('unplugged')
    v.album.block = 'нельзя'
    expect(screenOf(v)).toBe('blocked')
    v.busy = true
    expect(screenOf(v)).toBe('run')
    v.busyOther = true
    expect(screenOf(v)).toBe('busyOther')
  })

  it('причина Go при готовой ссылке: «Сохранить…» первым, иначе экран ссылки (T5-7)', () => {
    const v = stubView('published')
    v.album.block = 'Лист виньетки нельзя отправить по ссылке'
    expect(screenOf(v)).toBe('link')
    v.album.save = true
    expect(screenOf(v)).toBe('blocked')
    const c = stubView('changed')
    c.album.block = 'нельзя'
    expect(screenOf(c)).toBe('link')
    const o = stubView('other')
    o.album.block = 'нельзя'
    expect(screenOf(o)).toBe('other')
    const i = stubView('incomplete') // ссылки нет — причина первой
    i.album.block = 'нельзя'
    expect(screenOf(i)).toBe('blocked')
    const u = stubView('published') // без подключения причина не прячет форму подключения
    u.album.block = 'нельзя'
    u.service.token = false
    expect(screenOf(u)).toBe('connect')
  })

  it('hasLink: опубликованная и чужого сервиса — да; пустая, просроченная и недоделанная — нет', () => {
    expect(hasLink(stubView('plugged'))).toBe(false)
    expect(hasLink(stubView('changed'))).toBe(true)
    expect(hasLink(stubView('other'))).toBe(true)
    expect(hasLink(stubView('expired'))).toBe(false)
    expect(hasLink(stubView('incomplete'))).toBe(false)
  })

  it('blockText: у чужой выгрузки своя строка, иначе причина Go', () => {
    expect(blockText(stubView('busyOther'))).toBe(BUSY_OTHER)
    expect(blockText(stubView('unsaved'))).toContain('Сначала сохраните альбом')
    expect(blockText(stubView('plugged'))).toBe('')
  })
})

describe('ход выгрузки', () => {
  const p = (o: Partial<ShareProgress>): ShareProgress => ({ phase: 'pages', done: 0, total: 0, bytes: 0, totalBytes: 0, ...o })

  it('строки трёх фаз', () => {
    expect(runLine(null)).toBe('Готовим…')
    expect(runLine(p({ done: 12, total: 62 }))).toBe('Готовим развороты: 12 из 62')
    expect(runLine(p({ phase: 'upload', done: 18, total: 40, bytes: 7e6, totalBytes: 16e6 }))).toBe('Загружено 18 из 40 · 7 из 16 МБ')
    expect(runLine(p({ phase: 'publish' }))).toBe('Публикуем…')
  })

  it('до первого счёта страниц и без байтов — без выдуманных чисел', () => {
    expect(runLine(p({}))).toBe('Готовим…')
    expect(runLine(p({ phase: 'upload', done: 3, total: 9 }))).toBe('Загружено 3 из 9')
  })

  it('доля: страницы — по штукам, загрузка — по байтам, публикация и пустой счёт — неопределённая', () => {
    expect(runPercent(null)).toBeNull()
    expect(runPercent(p({ done: 12, total: 48 }))).toBe(25)
    expect(runPercent(p({ phase: 'upload', done: 1, total: 40, bytes: 8e6, totalBytes: 16e6 }))).toBe(50)
    expect(runPercent(p({ phase: 'upload', done: 10, total: 40 }))).toBe(25)
    expect(runPercent(p({ phase: 'publish', done: 1, total: 1 }))).toBeNull()
    expect(runPercent(p({ done: 5, total: 0 }))).toBeNull()
    expect(runPercent(p({ done: 99, total: 10 }))).toBe(100)
  })
})

describe('сводка', () => {
  it('число страниц склоняется, размер округляется', () => {
    expect(summaryLine(62, 24_000_000)).toBe('62 страницы до 1600 px, ≈ 24 МБ')
    expect(summaryLine(1, 800_000)).toBe('1 страница до 1600 px, ≈ 0,8 МБ')
    expect(summaryLine(5, 3_000_000)).toBe('5 страниц до 1600 px, ≈ 3 МБ')
    expect(summaryLine(11, 24_400_000)).toBe('11 страниц до 1600 px, ≈ 24 МБ')
  })
  it('размер неизвестен — только страницы', () => {
    expect(summaryLine(2, 0)).toBe('2 страницы до 1600 px')
  })
})

describe('срок', () => {
  beforeEach(() => localStorage.clear())

  it('нормализация: чужое — по умолчанию, предел сервиса срезает', () => {
    expect(normDays(30)).toBe(30)
    expect(normDays(45)).toBe(DEFAULT_DAYS)
    expect(normDays('x')).toBe(DEFAULT_DAYS)
    expect(normDays(365, 90)).toBe(90)
    expect(normDays(365, 60)).toBe(30)
    expect(normDays(90, 0)).toBe(90)
  })
  it('последний выбор помнится; порченое значение — по умолчанию', () => {
    expect(loadDays()).toBe(DEFAULT_DAYS)
    saveDays(365)
    expect(loadDays()).toBe(365)
    expect(loadDays(90)).toBe(90)
    localStorage.setItem('shareDays', '{')
    expect(loadDays()).toBe(DEFAULT_DAYS)
  })
  it('пункты: Месяц · 3 месяца · Год; сверх предела сервиса — серые', () => {
    expect(DAYS_ITEMS(0).map((i) => i.label)).toEqual(['Месяц', '3 месяца', 'Год'])
    expect(DAYS_ITEMS(0).some((i) => i.off)).toBe(false)
    expect(DAYS_ITEMS(90).map((i) => !!i.off)).toEqual([false, false, true])
  })
})

describe('строка про данные (152-ФЗ)', () => {
  it('до подключения — «этот сервер», потом имя хоста', () => {
    expect(dataHint('')).toBe(
      'Страницы альбома и название уходят на этот сервер. Их увидит любой, у кого есть ссылка (и пароль, если задан). Согласие клиента на показ — по вашему договору.',
    )
    expect(dataHint('albums.sosnina-films.ru')).toContain('на сервер albums.sosnina-films.ru. Их увидит')
  })
})
