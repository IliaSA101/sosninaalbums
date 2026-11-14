// Тесты go-pin.mjs (спека 171, design §8): закрепления версии Go и порог macOS. Запуск: node --test scripts/go-pin.test.mjs.
// Репозиторий копируется во временную папку (только нужные файлы), правки идут там; настоящее дерево не меняется. Сеть не нужна.
import assert from 'node:assert/strict'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { spawnSync } from 'node:child_process'
import { test } from 'node:test'
import { fileURLToPath } from 'node:url'
import { MAC_FLOOR, PLISTS, PinError, SMOKE, checkPins, cmpOS, collectPins, goVersionOf, latestFrom, macFloor, maxOS, parseGo, plistMin, writePins } from './go-pin.mjs'

const REPO = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const SCRIPT = path.join(REPO, 'scripts', 'go-pin.mjs')
const TOOLCHAIN = /^toolchain go(\d+\.\d+\.\d+)$/m.exec(fs.readFileSync(path.join(REPO, 'go.mod'), 'utf8'))[1]
const MINOR = parseGo(TOOLCHAIN).minor

// Производные места, которых в публичном дереве (main, winqa/*, CI) нет: docs/qa/ не публикуется.
const OPTIONAL = ['docs/qa/e2e-regress.md']
const HAS_E2E = fs.existsSync(path.join(REPO, OPTIONAL[0]))

/** Копия закреплений репозитория во временной папке: те же файлы, что читает go-pin. */
function mirror() {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'gopin-'))
  const files = ['go.mod', 'api/albums/go.mod', 'THIRD_PARTY_NOTICES.txt', 'docs/qa/e2e-regress.md', SMOKE, ...PLISTS]
  for (const f of fs.readdirSync(path.join(REPO, '.github', 'workflows'))) files.push(`.github/workflows/${f}`)
  for (const f of files) {
    // производное место может отсутствовать (docs/qa/ нет в публичном дереве): go-pin его пропускает
    if (OPTIONAL.includes(f) && !fs.existsSync(path.join(REPO, f))) continue
    fs.mkdirSync(path.dirname(path.join(dir, f)), { recursive: true })
    fs.copyFileSync(path.join(REPO, f), path.join(dir, f))
  }
  return dir
}
const rd = (dir, f) => fs.readFileSync(path.join(dir, f), 'utf8')
const wr = (dir, f, t) => fs.writeFileSync(path.join(dir, f), t)
const edit = (dir, f, from, to) => {
  const t = rd(dir, f)
  assert.ok(t.includes(from), `${f}: нет «${from}»`)
  wr(dir, f, t.replace(from, to))
}
const problems = (dir) => checkPins(dir).problems

test('репозиторий согласован: --check зелёный, 20 мест: 2 go.mod, 15 go-version и 3 производных', () => {
  assert.deepEqual(checkPins(REPO).problems, [])
  const pins = collectPins(REPO)
  const by = (re) => pins.filter((p) => re.test(p.file)).length
  assert.equal(by(/^go\.mod$|^api\/albums\/go\.mod$/), 2)
  assert.equal(by(/^\.github\/workflows\//), 15, 'строк go-version в workflow')
  assert.ok(by(/THIRD_PARTY_NOTICES|e2e-regress/) >= (HAS_E2E ? 2 : 1), 'производные места (e2e-regress.md — только в полном дереве)')
  assert.ok(pins.every((p) => p.version === TOOLCHAIN))
  const r = spawnSync(process.execPath, [SCRIPT, '--check'], { encoding: 'utf8' })
  assert.equal(r.status, 0, r.stderr)
  assert.match(r.stdout, /go-pin: go\d+\.\d+\.\d+ во всех закреплениях/)
  assert.equal(spawnSync(process.execPath, [SCRIPT], { encoding: 'utf8' }).status, 0, 'без флагов — то же, что --check')
})

test('расхождение: одна строка workflow, api/albums/go.mod, версия без патча и «stable» — каждая в списке с файлом и строкой', () => {
  const dir = mirror()
  edit(dir, '.github/workflows/ci.yml', `go-version: "${TOOLCHAIN}"`, 'go-version: "1.26.1"')
  edit(dir, 'api/albums/go.mod', `toolchain go${TOOLCHAIN}`, 'toolchain go1.26.2')
  const p = problems(dir)
  assert.equal(p.length, 2, p.join('\n'))
  assert.match(p[0], /^api\/albums\/go\.mod:\d+: 1\.26\.2, в go\.mod /)
  assert.match(p[1], /^\.github\/workflows\/ci\.yml:\d+: 1\.26\.1, в go\.mod /)
  // версия без патча или имя вместо неё не должны проскочить мимо проверки
  edit(dir, '.github/workflows/win-qa.yml', `go-version: "${TOOLCHAIN}"`, 'go-version: "1.26"')
  edit(dir, '.github/workflows/release.yml', `go-version: "${TOOLCHAIN}"`, 'go-version: stable')
  const q = problems(dir)
  assert.equal(q.length, 4, q.join('\n'))
  assert.ok(q.some((x) => /win-qa\.yml: версия Go не найдена/.test(x)) && q.some((x) => /release\.yml: версия Go не найдена/.test(x)))
  // производное место расходится — тоже
  const d = mirror()
  edit(d, 'THIRD_PARTY_NOTICES.txt', `Стандартная библиотека Go go${TOOLCHAIN}`, 'Стандартная библиотека Go go1.25.0')
  assert.match(problems(d)[0], /^THIRD_PARTY_NOTICES\.txt:\d+: 1\.25\.0/)
  // нет строки toolchain в go.mod — одна понятная ошибка, а не падение
  const e = mirror()
  wr(e, 'go.mod', 'module x\n\ngo 1.26.0\n')
  assert.deepEqual(problems(e), ['go.mod: нет строки «toolchain goX.Y.Z»'])
})

test('плист или MAC_MIN_OS ниже порога Go — отказ; на пороге и выше — проходит', () => {
  const floor = MAC_FLOOR[MINOR]
  for (const plist of PLISTS) {
    const dir = mirror()
    edit(dir, plist, `<string>${plistMin(rd(dir, plist))}</string>`, '<string>10.13.0</string>')
    const p = problems(dir)
    assert.equal(p.length, 1, p.join('\n'))
    assert.match(p[0], new RegExp(`^${plist.replace(/[.]/g, '\\.')}: LSMinimumSystemVersion 10\\.13\\.0 ниже порога macOS ${floor.replace('.', '\\.')}`))
    edit(dir, plist, '<string>10.13.0</string>', `<string>${floor}</string>`) // ровно порог
    assert.deepEqual(problems(dir), [])
    edit(dir, plist, `<string>${floor}</string>`, '<string>99.0</string>') // выше — тоже можно
    assert.deepEqual(problems(dir), [])
  }
  const dir = mirror()
  edit(dir, SMOKE, /MAC_MIN_OS:-([\d.]+)/.exec(rd(dir, SMOKE))[0], 'MAC_MIN_OS:-11.0')
  assert.match(problems(dir)[0], /smoke\.sh: MAC_MIN_OS по умолчанию 11\.0 ниже порога macOS/)
  // плист без ключа, с мусором и без файла
  const x = mirror()
  edit(x, PLISTS[0], /<key>LSMinimumSystemVersion<\/key>/.exec(rd(x, PLISTS[0]))[0], '<key>Other</key>')
  assert.match(problems(x)[0], /LSMinimumSystemVersion не найден/)
  edit(x, PLISTS[1], `<string>${plistMin(rd(x, PLISTS[1]))}</string>`, '<string>abc</string>')
  assert.match(problems(x)[1], /«abc» — не версия системы/)
  fs.rmSync(path.join(x, SMOKE))
  assert.match(problems(x).at(-1), /smoke\.sh: нет строки MIN_OS/)
})

test('--write меняет все места, затем --check зелёный; повтор — ничего не меняет', () => {
  const dir = mirror()
  const next = `${MINOR}.${parseGo(TOOLCHAIN).patch + 1}`
  const w = writePins(dir, `go${next}`)
  assert.equal(w.version, next)
  assert.deepEqual(problems(dir), [])
  const pins = collectPins(dir)
  assert.equal(pins.length, collectPins(REPO).length)
  assert.ok(pins.every((p) => p.version === next))
  // ни одного старого вхождения в закреплённых файлах
  for (const f of w.changed) assert.ok(!rd(dir, f).includes(TOOLCHAIN), `${f}: осталась ${TOOLCHAIN}`)
  assert.ok(w.changed.includes('go.mod') && w.changed.includes('api/albums/go.mod') && w.changed.includes('.github/workflows/ci.yml'))
  assert.equal(w.changed.filter((f) => f.startsWith('.github/workflows/')).length, 3)
  assert.ok(rd(dir, 'go.mod').includes(`toolchain go${next}`) && rd(dir, 'go.mod').includes('go 1.26.0'), 'директива go не тронута')
  assert.deepEqual(writePins(dir, next).changed, [], 'без префикса go и повтором — без правок')
  // остальной текст файлов не задет: разница только в версиях
  for (const f of w.changed) assert.equal(rd(dir, f).split(next).join(TOOLCHAIN), rd(REPO, f), `${f}: изменено не только число`)
})

test('--write на новый минор: плисты и MAC_MIN_OS ниже порога — отказ в --check (шаг 2); минор вне таблицы и мусор — отказ до правок', () => {
  const dir = mirror()
  writePins(dir, 'go1.27.4')
  const p = problems(dir)
  assert.equal(p.length, 3, p.join('\n'))
  assert.ok(p.every((x) => /13\.0/.test(x)), p.join('\n'))
  for (const plist of PLISTS) edit(dir, plist, `<string>${plistMin(rd(dir, plist))}</string>`, '<string>13.0</string>')
  edit(dir, SMOKE, /MAC_MIN_OS:-([\d.]+)/.exec(rd(dir, SMOKE))[0], 'MAC_MIN_OS:-13.0')
  assert.deepEqual(problems(dir), [])
  const before = rd(dir, 'go.mod')
  assert.throws(() => writePins(dir, 'go1.99.0'), (e) => e instanceof PinError && /нет порога macOS для go1\.99.*MAC_FLOOR/.test(e.message))
  for (const bad of ['go1.26', '1.26', 'x', '', undefined, 'go1.26.x']) assert.throws(() => writePins(dir, bad), PinError, String(bad))
  assert.equal(rd(dir, 'go.mod'), before, 'при отказе ничего не записано')
  // go.mod с минором вне таблицы — отказ в --check со ссылкой на таблицу
  const x = mirror()
  edit(x, 'go.mod', `toolchain go${TOOLCHAIN}`, 'toolchain go1.99.1')
  assert.ok(problems(x).some((m) => /нет порога macOS для go1\.99/.test(m)))
})

test('CLI: --write без значения и лишние аргументы — код 2; --write не трогает дерево при отказе', () => {
  const run = (...a) => spawnSync(process.execPath, [SCRIPT, ...a], { encoding: 'utf8', cwd: REPO })
  const before = fs.readFileSync(path.join(REPO, 'go.mod'), 'utf8')
  for (const a of [['--write'], ['--write', 'go1.99.0'], ['--write', 'мусор'], ['--bogus']]) {
    const r = run(...a)
    assert.equal(r.status, 2, a.join(' '))
    assert.match(r.stderr, /go-pin: /)
  }
  assert.equal(fs.readFileSync(path.join(REPO, 'go.mod'), 'utf8'), before)
})

test('latest из JSON go.dev: последний стабильный патч своего минора и последний минор; нестабильные и мусор пропускаются', () => {
  const list = [
    { version: 'go1.27.4', stable: true },
    { version: 'go1.27rc1', stable: false },
    { version: 'go1.28rc1', stable: false },
    { version: 'go1.26.11', stable: true },
    { version: 'go1.26.9', stable: true },
    { version: 'go1.26.12', stable: false }, // не стабильный — не берём
    { version: 'go1.25.20', stable: true },
    { version: 'go1.27', stable: true },
    { version: 7 },
    null,
  ]
  assert.deepEqual(latestFrom(list, '1.26.11'), { patch: 'go1.26.11', minor: 'go1.27.4' })
  assert.deepEqual(latestFrom(list, '1.25.3'), { patch: 'go1.25.20', minor: 'go1.27.4' })
  assert.deepEqual(latestFrom(list, '1.24.1'), { patch: null, minor: 'go1.27.4' }, 'у минора нет выпусков — null')
  // сравнение чисел, не строк: 1.26.10 новее 1.26.9, 1.100 новее 1.27
  assert.equal(latestFrom([{ version: 'go1.26.9', stable: true }, { version: 'go1.26.10', stable: true }], '1.26.1').patch, 'go1.26.10')
  assert.equal(latestFrom([{ version: 'go1.27.1', stable: true }, { version: 'go1.100.0', stable: true }], '1.27.1').minor, 'go1.100.0')
  assert.throws(() => latestFrom([], '1.26.11'), PinError)
  assert.throws(() => latestFrom([{ version: 'go1.27rc1', stable: false }], '1.26.11'), PinError)
  assert.throws(() => latestFrom({}, '1.26.11'), PinError)
  assert.throws(() => latestFrom(list, 'abc'), PinError)
})

test('порог macOS по минору, сравнение версий системы и разбор go.mod и плиста', () => {
  assert.equal(macFloor('1.26.11'), '12.0')
  assert.equal(macFloor('go1.27.4'), '13.0')
  assert.equal(macFloor('1.24'), '11.0')
  assert.throws(() => macFloor('1.28.0'), (e) => e instanceof PinError && /go1\.28.*MAC_FLOOR/.test(e.message))
  assert.throws(() => macFloor('1.22.5'), PinError) // ниже таблицы тоже отказ: впишите порог
  assert.throws(() => macFloor('abc'), PinError)
  assert.ok(cmpOS('13.0', '12.9.9') > 0 && cmpOS('13', '13.0.0') === 0 && cmpOS('10.13.0', '12.0') < 0 && cmpOS('10.0.22000', '10.0.9') > 0)
  assert.equal(maxOS('10.13.0', '12.0'), '12.0')
  assert.equal(maxOS('14.0', '12.0'), '14.0')
  assert.equal(goVersionOf('go 1.26.0\ntoolchain go1.26.11\n'), '1.26.11')
  assert.equal(goVersionOf('module x\ngo 1.25.3\n'), '1.25.3')
  assert.equal(goVersionOf('module x\n'), null)
  assert.equal(plistMin('<key>LSMinimumSystemVersion</key>\n  <string> 12.0 </string>'), '12.0')
  assert.equal(plistMin('<key>A</key><string>1</string>'), null)
  assert.deepEqual(parseGo('go1.26.11'), { minor: '1.26', patch: 11 })
  assert.equal(parseGo('1.26rc1'), null)
})
