// Тесты тем набора при выпуске (спека 170, design §6): правила объявлений и ссылок, сверка с прежним каталогом,
// самопроверка зеркалом разбора программы. Общие случаи парности с Go — pack-check.test.mjs и packprep.TestThemeDeclParity.
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { test } from 'node:test'
import { fileURLToPath } from 'node:url'
import { MAX_PACK_THEMES, checkCatalogThemes, checkMirror, srcThemes, themeRow, themeSkeleton } from './library-pack-themes.mjs'
import { validThemeId, validThemeName } from './pack-check/manifest.mjs'

const NEWYEAR = { id: 'newyear', name: 'Новый год', base: 'holiday', order: 10 }
const KIDS = { id: 'kidsparty', name: 'Детский праздник', base: 'kids' }
const item = (file, theme, topic) => ({ file, kind: 'sticker', theme, name: file, ...(topic === undefined ? {} : { topic }) })

/** Исходник с двумя темами: по предмету в каждой и один без темы. */
const goodSrc = () => ({
  themes: [{ ...NEWYEAR }, { ...KIDS }],
  items: [item('st/a.png', 'holiday', 'newyear'), item('st/b.png', 'kids', 'kidsparty'), item('st/c.png', 'school')],
})

const refused = (src, re) => assert.throws(() => srcThemes(src), re)

test('srcThemes: годное — объявления как записаны, без подставленных умолчаний; без themes — пусто', () => {
  assert.deepEqual(srcThemes(goodSrc()), [NEWYEAR, KIDS])
  assert.deepEqual(srcThemes({ items: [item('st/a.png', 'school')] }), [])
  assert.deepEqual(srcThemes({ themes: null, items: [] }), [])
  assert.deepEqual(srcThemes({ themes: [], items: [item('st/a.png', 'school')] }), [])
})

test('srcThemes: тема без предметов, topic без объявления, theme не база, id темы в theme — отказ', () => {
  const s = goodSrc()
  s.themes.push({ id: 'extra', name: 'Лишняя', base: 'basic' })
  refused(s, /тема "extra" объявлена, но ни один предмет/)

  const orphan = goodSrc()
  orphan.items.push(item('st/d.png', 'holiday', 'ghost'))
  refused(orphan, /topic "ghost" без объявления/)

  const noDecl = { items: [item('st/a.png', 'holiday', 'newyear')] }
  refused(noDecl, /topic "newyear" без объявления/)

  const wrongBase = goodSrc()
  wrongBase.items[0].theme = 'kids'
  refused(wrongBase, /theme "kids" при topic "newyear": в theme идёт база темы \(holiday\)/)

  const noTheme = goodSrc()
  delete noTheme.items[1].theme // по умолчанию basic, а база — kids
  refused(noTheme, /theme "basic" при topic "kidsparty"/)

  const idInTheme = goodSrc()
  idInTheme.items[2].theme = 'newyear'
  refused(idInTheme, /theme "newyear" — id темы набора/)

  refused({ themes: { id: 'x' }, items: [] }, /нужен список/)
  const nonString = goodSrc()
  nonString.items[0].topic = 7
  refused(nonString, /topic "7" без объявления/)
})

test('srcThemes: правила записи — id, имя, база, order, повторы, предел 16, чужие поля', () => {
  const one = (d, used = d.id) => ({ themes: [d], items: [item('st/a.png', d.base ?? 'basic', used)] })
  const bad = [
    [{ id: 'holiday', name: 'Лето', base: 'basic' }, /id "holiday"/],
    [{ id: 'constructor', name: 'Лето', base: 'basic' }, /id "constructor"/],
    [{ id: 'year2', name: 'Лето', base: 'basic' }, /id "year2"/],
    [{ id: 'abcdefghijklmnopq', name: 'Лето', base: 'basic' }, /id "abcdefghijklmnopq"/],
    [{ id: 'summer', name: 'я'.repeat(21), base: 'basic' }, /имя/],
    [{ id: 'summer', name: 'Пpаздник', base: 'basic' }, /имя/], // латинская p среди кириллицы
    [{ id: 'summer', name: 'Лето​', base: 'basic' }, /имя/],
    [{ id: 'summer', name: 'праздник', base: 'basic' }, /совпадает с другой темой или встроенной/],
    [{ id: 'summer', name: 'Мои', base: 'basic' }, /совпадает/],
    [{ id: 'summer', name: 'Лето' }, /base "undefined"/], // база явная: пустая у программы — basic, у инструмента — отказ
    [{ id: 'summer', name: 'Лето', base: 'newyear' }, /base "newyear"/],
    [{ id: 'summer', name: 'Лето', base: 'basic', order: 0 }, /order "0"/],
    [{ id: 'summer', name: 'Лето', base: 'basic', order: 100 }, /order "100"/],
    [{ id: 'summer', name: 'Лето', base: 'basic', order: 1.5 }, /order/],
    [{ id: 'summer', name: 'Лето', base: 'basic', order: '5' }, /order/],
    [{ id: 'summer', name: 'Лето', base: 'basic', color: 'red' }, /неизвестное поле "color"/],
  ]
  for (const [d, re] of bad) refused(one(d), re)
  assert.deepEqual(srcThemes(one({ id: 'summer', name: 'Лето', base: 'basic', order: 99 })), [{ id: 'summer', name: 'Лето', base: 'basic', order: 99 }])
  assert.deepEqual(srcThemes(one({ id: 'summer', name: 'Лето', base: 'basic', order: 1 })), [{ id: 'summer', name: 'Лето', base: 'basic', order: 1 }])

  const dupId = goodSrc()
  dupId.themes[1].id = 'newyear'
  refused(dupId, /id "newyear" повторяется/)
  const twin = goodSrc()
  twin.themes[1].name = 'НОВЫЙ ГОД'
  refused(twin, /имя "НОВЫЙ ГОД" совпадает/)
  const notObject = goodSrc()
  notObject.themes.push('x', null, [])
  refused(notObject, /themes\[2\]: не объект/)

  // 16 — предел включительно, 17 — отказ
  const many = (n) => ({
    themes: Array.from({ length: n }, (_, i) => ({ id: 'a'.repeat(i + 1), name: `Тема ${String.fromCharCode(1072 + i)}`, base: 'basic' })),
    items: Array.from({ length: n }, (_, i) => item(`st/${i}.png`, 'basic', 'a'.repeat(i + 1))),
  })
  assert.equal(srcThemes(many(16)).length, 16)
  refused({ ...many(17), themes: [...many(16).themes, { id: 'zz', name: 'Зз', base: 'basic' }] }, /не больше 16 объявлений, а их 17/)
})

// Гомоглифы (аудит 170, п. 2): слово целиком латиницей или с цифрой проходит белый список имени, но на глаз — другая тема.
test('имена по скелету: «Bce» = «Все», «Mope» = «Море», «3има» = «Зима»; служебная строка или другая тема — отказ', () => {
  assert.equal(themeSkeleton('Bce'), themeSkeleton('Все'))
  assert.equal(themeSkeleton('Mope'), themeSkeleton('Море'))
  assert.equal(themeSkeleton('3има'), themeSkeleton('Зима'))
  assert.equal(themeSkeleton('Лето 0'), themeSkeleton('Лето О'))
  assert.notEqual(themeSkeleton('Mope'), themeSkeleton('Лето')) // разные слова разными и остаются
  const two = (a, b) => ({
    themes: [
      { id: 'one', name: a, base: 'basic' },
      { id: 'two', name: b, base: 'basic' },
    ],
    items: [item('st/a.png', 'basic', 'one'), item('st/b.png', 'basic', 'two')],
  })
  const one = (name) => ({ themes: [{ id: 'one', name, base: 'basic' }], items: [item('st/a.png', 'basic', 'one')] })
  refused(one('Bce'), /"Bce" совпадает с другой темой или встроенной/) // служебная «Все»
  refused(two('Море', 'Mope'), /"Mope" совпадает/)
  refused(two('Mope', 'Море'), /"Море" совпадает/)
  refused(two('Зима', '3има'), /"3има" совпадает/)
  assert.equal(srcThemes(one('Mope')).length, 1) // сама по себе латинская «Mope» допустима
  assert.equal(srcThemes(one('3има')).length, 1)
  // против прежнего каталога
  const packs = [{ id: 'sea', themes: [{ id: 'sea', name: 'Море', base: 'basic' }] }]
  assert.throws(() => checkCatalogThemes([{ id: 'beach', name: 'Mope', base: 'basic' }], packs, 'z'), /"Mope" уже у темы "sea" набора sea/)
  assert.throws(() => checkCatalogThemes([{ id: 'beach', name: 'MOPE', base: 'basic' }], packs, 'z'), /уже у темы "sea"/)
})

test('srcThemes: все нарушения — одним исключением, один прогон — все причины', () => {
  const s = goodSrc()
  s.themes[0].base = 'x'
  s.themes[1].name = ''
  s.items[2].topic = 'ghost'
  assert.throws(
    () => srcThemes(s),
    (e) => /base "x"/.test(e.message) && /имя ""/.test(e.message) && /topic "ghost"/.test(e.message) && e.message.split('\n').length >= 4,
  )
})

test('srcThemes и зеркало программы: id и имена на одних правилах', () => {
  for (const id of ['newyear', 'a', 'abcdefghijklmnop']) assert.equal(validThemeId(id), true)
  for (const id of ['', 'Newyear', 'holiday', 'mine', 'abcdefghijklmnopq']) assert.equal(validThemeId(id), false)
  for (const n of ['Новый год', 'История любви', 'Лето-2026', 'Baby']) assert.equal(validThemeName(n), true, n)
})

test('checkCatalogThemes: один id — одно имя и база; разные id — разные имена; своя строка заменяется', () => {
  const row = (id, themes) => ({ id, themes })
  const packs = [row('holiday-birthday', [{ id: 'birthday', name: 'День рождения', base: 'holiday' }]), row('plain', undefined), row('odd', 'строка')]
  const themes = [{ id: 'newyear', name: 'Новый год', base: 'holiday' }]
  checkCatalogThemes(themes, packs, 'holiday-newyear') // чужие темы другие — тихо
  checkCatalogThemes([], packs, 'x') // нет объявлений — нечего сверять

  // тот же id и то же имя и база — второй набор той же темы допустим
  checkCatalogThemes([{ id: 'birthday', name: 'День рождения', base: 'holiday', order: 5 }], packs, 'holiday-cake')
  assert.throws(() => checkCatalogThemes([{ id: 'birthday', name: 'Именины', base: 'holiday' }], packs, 'holiday-cake'), /имя "Именины", а в наборе holiday-birthday — "День рождения"/)
  assert.throws(() => checkCatalogThemes([{ id: 'birthday', name: 'День рождения', base: 'kids' }], packs, 'holiday-cake'), /база "kids", а в наборе holiday-birthday — "holiday"/)
  // другой id с тем же именем, в том числе в другом регистре
  assert.throws(() => checkCatalogThemes([{ id: 'bday', name: 'день РОЖДЕНИЯ', base: 'holiday' }], packs, 'holiday-cake'), /уже у темы "birthday" набора holiday-birthday: разные id — разные имена/)
  // свою прежнюю строку набор перезаписывает: те же id с другим именем при выпуске новой версии допустимы
  checkCatalogThemes([{ id: 'birthday', name: 'Именины', base: 'holiday' }], packs, 'holiday-birthday')
  // две причины сразу
  assert.throws(
    () => checkCatalogThemes([{ id: 'birthday', name: 'Именины', base: 'kids' }], packs, 'z'),
    (e) => /имя/.test(e.message) && /база/.test(e.message),
  )
})

test('checkCatalogThemes: больше 64 тем в каталоге — отказ; ровно 64 — можно', () => {
  const ids = (from, n) => Array.from({ length: n }, (_, i) => `t${String.fromCharCode(97 + ((from + i) % 26))}${String.fromCharCode(97 + Math.floor((from + i) / 26))}`)
  const decl = (id) => ({ id, name: `Т ${id}`, base: 'basic' })
  const packs = [{ id: 'big', themes: ids(0, MAX_PACK_THEMES - 1).map(decl) }]
  checkCatalogThemes([decl('zzzz')], packs, 'new')
  assert.throws(() => checkCatalogThemes([decl('zzzz'), decl('yyyy')], packs, 'new'), /тем наборов в каталоге 65/)
})

test('themeRow: id, имя, база — без order', () => {
  assert.deepEqual(themeRow([NEWYEAR, KIDS]), [
    { id: 'newyear', name: 'Новый год', base: 'holiday' },
    { id: 'kidsparty', name: 'Детский праздник', base: 'kids' },
  ])
  assert.deepEqual(themeRow([]), [])
})

test('checkMirror: зеркало программы принимает собранный манифест; расхождение — ошибка', () => {
  const sum = 'a'.repeat(64)
  const m = (themes, topic) => ({
    format: 1, id: 'test-set', version: 1, minApp: '0.29.0', title: 'Т', about: '', author: '', license: '', source: '',
    themes,
    items: [{ file: 'st/a.png', kind: 'sticker', theme: 'holiday', ...(topic ? { topic } : {}), name: 'А', sha256: sum, size: 10 }],
  })
  checkMirror(m(undefined)) // без тем — нечего сверять
  checkMirror(m([NEWYEAR], 'newyear'))
  checkMirror(m([{ id: 'newyear', name: 'Новый год', base: 'holiday' }], 'newyear')) // order по умолчанию — 50
  // манифест, который программа бы разобрала иначе: имя вне белого списка — пропуск записи
  assert.throws(() => checkMirror(m([{ ...NEWYEAR, name: 'Пpаздник' }], 'newyear')), /программа примет не всё \(пропущено 1: имя\)/)
  // topic, который программа сбросила бы в базу
  assert.throws(() => checkMirror(m([NEWYEAR], 'ghost')), /topic потерян у 1 предметов/)
})

test('pack-manifest-cases.json: годные объявления из общего вектора проходят srcThemes, пропущенные программой — нет', () => {
  const v = JSON.parse(fs.readFileSync(fileURLToPath(new URL('../testdata/pack-manifest-cases.json', import.meta.url)), 'utf8'))
  let ok = 0
  let refusedN = 0
  for (const c of v.cases) {
    const raw = c.manifest?.themes
    if (!Array.isArray(raw) || raw.length === 0 || !raw.every((d) => d !== null && typeof d === 'object' && !Array.isArray(d) && typeof d.id === 'string' && typeof d.name === 'string' && (d.base === undefined || typeof d.base === 'string') && ['undefined', 'number'].includes(typeof d.order))) continue
    // каждая запись используется: в инструмент темы приходят вместе с предметом-ссылкой
    const src = { themes: raw, items: raw.map((d, i) => item(`st/${i}.png`, d.base || 'basic', d.id)) }
    const accepted = c.want.ok && c.want.themeSkipN === 0 && c.want.themes.length === raw.length && raw.every((d) => d.base)
    let passed = true
    try {
      srcThemes(src)
    } catch {
      passed = false
    }
    // инструмент строже: годное для программы без явной базы он отвергает, но принятое им программа принимает
    if (passed) {
      assert.ok(c.want.ok && c.want.themeSkipN === 0 && c.want.themes.length === raw.length, `${c.name}: инструмент принял, а программа пропустила`)
      ok++
    } else if (!accepted) refusedN++
  }
  assert.ok(ok >= 5 && refusedN >= 10, `вектор почти не использован: принято ${ok}, отвергнуто ${refusedN}`)
})
