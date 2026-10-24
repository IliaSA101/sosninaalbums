// Генератор THIRD_PARTY_NOTICES.txt — единый файл лицензий: реальный состав сборок + полные тексты.
// Go-модули: `go list -deps` отдельно под windows, darwin и linux (теги сборки Wails; у Linux +webkit2_41),
// у модуля пометка ОС, если он не во всех трёх; текст — ближайший к пакету файл LICENSE в кэше модулей
// (как у go-licenses, но без него: go-licenses работает только под ОС машины); лицензия стандартной
// библиотеки — из GOROOT; npm-пакеты — список NPM ниже (рантайм Svelte и его зависимости; тексты — из
// frontend/node_modules); шрифты — internal/text/fonts. Только Node. Запуск из корня:
// node scripts/third-party.mjs (нужны go и frontend/node_modules; GOTMPDIR задан в окружении).
// При смене состава сборки обновить NPM и проверить diff.
import fs from 'fs'
import path from 'path'
import { execFileSync } from 'child_process'
import { fileURLToPath } from 'url'
import { withGoCaches } from './goenv.mjs'

const R = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..').replace(/\\/g, '/')
const W = R
// Сборки программы: теги и cgo как в scripts/build.* и release.yml (у Linux — WebKitGTK 4.1).
const TARGETS = [
  { os: 'windows', name: 'Windows', tags: 'desktop,production', cgo: '0' },
  { os: 'darwin', name: 'macOS', tags: 'desktop,production', cgo: '1' },
  { os: 'linux', name: 'Linux', tags: 'desktop,production,webkit2_41', cgo: '1' },
]
const run = (cmd, args, extra = {}) =>
  execFileSync(cmd, args, { cwd: R, encoding: 'utf8', maxBuffer: 1 << 26, env: withGoCaches({ ...process.env, ...extra }, R) })
const GOROOT = run('go', ['env', 'GOROOT']).trim()
const GOVERSION = run('go', ['env', 'GOVERSION']).trim()
const NPM = [
  ['@jridgewell/gen-mapping', 'MIT'],
  ['@jridgewell/remapping', 'MIT'],
  ['@jridgewell/resolve-uri', 'MIT'],
  ['@jridgewell/sourcemap-codec', 'MIT'],
  ['@jridgewell/trace-mapping', 'MIT'],
  ['@sveltejs/acorn-typescript', 'MIT'],
  ['@types/estree', 'MIT'],
  ['acorn', 'MIT'],
  ['aria-query', 'Apache-2.0'],
  ['axobject-query', 'Apache-2.0'],
  ['clsx', 'MIT'],
  ['devalue', 'MIT'],
  ['esm-env', 'MIT'],
  ['esrap', 'MIT'],
  ['is-reference', 'MIT'],
  ['locate-character', 'MIT'],
  ['magic-string', 'MIT'],
  ['svelte', 'MIT'],
  ['zimmerframe', 'MIT'],
]
const norm = (t) =>
  t
    .replace(/\r\n/g, '\n')
    .replace(/[ \t]+$/gm, '')
    .trim() + '\n'
const MIT_BODY = `Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.`
const groups = new Map() // text -> {lic, items}
function add(name, ver, lic, text) {
  const t = norm(text)
  if (!groups.has(t)) groups.set(t, { lic: new Set(), items: [] })
  const g = groups.get(t)
  g.lic.add(lic)
  if (!g.items.includes(`${name} ${ver}`)) g.items.push(`${name} ${ver}`)
}
const section = { go: [], npm: [] }
// Вид лицензии по тексту; незнакомый — ошибка (новый модуль проверить глазами и дополнить).
// Файл с несколькими лицензиями (MIT + Apache) помечается всеми; BSD-4 (пункт о рекламе) — ошибка.
function classify(text, where) {
  if (/advertising materials/i.test(text)) throw new Error('BSD-4 (пункт о рекламе) несовместим с GPL, проверить: ' + where)
  const found = []
  if (/Apache License/.test(text) && /Version 2\.0/.test(text)) found.push('Apache-2.0')
  if (/^ISC License/m.test(text)) found.push('ISC')
  if (/Permission is hereby granted, free of charge/.test(text)) found.push('MIT')
  if (/Redistributions of source code must retain/.test(text)) found.push(/Neither the name/.test(text) ? 'BSD-3-Clause' : 'BSD-2-Clause')
  if (!found.length) throw new Error('неизвестная лицензия: ' + where)
  return found.join(' + ')
}
// Ближайший к пакету файл лицензии внутри модуля (как у go-licenses).
function licenseFile(dir, root) {
  for (let d = dir; ; d = path.dirname(d)) {
    const f = fs.readdirSync(d).find((x) => /^LICEN[CS]E/i.test(x))
    if (f) return path.join(d, f)
    if (d === root || path.dirname(d) === d) return null
  }
}
const gomods = new Map() // «модуль версия файл» -> {name, ver, lic, text, oses}
for (const t of TARGETS) {
  const env = { GOOS: t.os, GOARCH: 'amd64', CGO_ENABLED: t.cgo, GOFLAGS: `-tags=${t.tags}` }
  const raw = run('go', ['list', '-deps', '-json=Module,Dir,ImportPath', './...'], env)
  for (const p of JSON.parse(`[${raw.replace(/}\s*{/g, '},{')}]`)) {
    const m = p.Module
    // Свой модуль и стандартная библиотека (у неё нет Module) в списки модулей не входят.
    if (!m || m.Path === 'github.com/IliaSA101/sosninaalbums') continue
    const file = licenseFile(p.Dir, m.Dir)
    if (!file) throw new Error('нет файла лицензии: ' + p.ImportPath)
    const key = `${m.Path} ${m.Version} ${file}`
    if (!gomods.has(key)) {
      const text = fs.readFileSync(file, 'utf8')
      gomods.set(key, { name: m.Path, ver: m.Version, lic: classify(text, file), text, oses: new Set() })
    }
    gomods.get(key).oses.add(t.name)
  }
}
// ОС по всем файлам одной лицензии модуля; пометка, если модуль не во всех сборках.
const labelOses = new Map()
for (const g of gomods.values()) {
  const label = `${g.name} ${g.ver} (${g.lic})`
  if (!labelOses.has(label)) labelOses.set(label, new Set())
  for (const o of g.oses) labelOses.get(label).add(o)
}
for (const g of [...gomods.values()].sort((a, b) => a.name.localeCompare(b.name) || a.lic.localeCompare(b.lic))) {
  const label = `${g.name} ${g.ver} (${g.lic})`
  const oses = labelOses.get(label)
  const line = label + (oses.size === TARGETS.length ? '' : ` — только ${TARGETS.filter((t) => oses.has(t.name)).map((t) => t.name).join(', ')}`)
  if (!section.go.includes(line)) section.go.push(line)
  add(g.name, g.ver, g.lic, g.text)
}
// Стандартная библиотека
section.go.push(`Стандартная библиотека Go ${GOVERSION} (BSD-3-Clause)`)
add('Стандартная библиотека Go', GOVERSION, 'BSD-3-Clause', fs.readFileSync(GOROOT + '/LICENSE', 'utf8'))
// npm
for (const [name, lic] of NPM) {
  const dir = R + '/frontend/node_modules/' + name
  const pj = JSON.parse(fs.readFileSync(dir + '/package.json', 'utf8'))
  const f = fs.readdirSync(dir).find((x) => /^(LICEN[CS]E|COPYING)/i.test(x))
  section.npm.push(`${name} ${pj.version} (${lic})`)
  if (!f) {
    // В пакете нет файла лицензии — стандартный текст MIT по полю license и автору из package.json
    if (lic !== 'MIT') throw new Error('no license ' + name)
    const au = typeof pj.author === 'string' ? pj.author : pj.author.name
    add(
      name,
      pj.version,
      lic,
      `MIT License (файл лицензии в пакете отсутствует; указано в package.json: license MIT, author ${au})

Copyright (c) ${au}

` + MIT_BODY,
    )
    continue
  }
  add(name, pj.version, lic, fs.readFileSync(dir + '/' + f, 'utf8'))
}
const fontsDir = W + '/internal/text/fonts/'
// Гарнитуры: [имя, файл лицензии, автор, зарезервированное имя, изменения]. Число гарнитур и файлов
// в заголовке раздела 3 считается ниже по этому массиву и по *.ttf в папке.
const fonts = [
  ['Bad Script', 'OFL-badscript.txt', 'The Bad Script Project Authors', 'нет', 'урезан набор знаков'],
  ['Caveat', 'OFL-caveat.txt', 'The Caveat Project Authors', 'нет', 'урезан набор знаков'],
  ['Comforter', 'OFL-comforter.txt', 'The Comforter Project Authors', 'нет', 'урезан набор знаков'],
  ['Great Vibes', 'OFL-greatvibes.txt', 'The Great Vibes Pro Project Authors', 'нет', 'урезан набор знаков, кернинг кириллицы подключён к latn'],
  ['Lobster', 'OFL-lobster.txt', 'The Lobster Project Authors', 'ДА: "Lobster"', 'без изменений'],
  ['Marck Script', 'OFL-marck.txt', 'Denis Masharov и Marck Fogel', 'ДА: "Marck Script"', 'без изменений'],
  ['Pacifico', 'OFL-pacifico.txt', 'The Pacifico Project Authors', 'нет', 'урезан набор знаков'],
  ['Amatic SC', 'OFL-amaticsc.txt', 'The Amatic SC Project Authors', 'нет', 'урезан набор знаков'],
  ['Comfortaa', 'OFL-comfortaa.txt', 'The Comfortaa Project Authors', 'ДА: "Comfortaa"', 'без изменений'],
  ['Marmelad', 'OFL-marmelad.txt', 'The Marmelad Project Authors', 'нет', 'урезан набор знаков'],
  ['Neucha', 'OFL-neucha.txt', 'Jovanny Lemonad', 'нет', 'урезан набор знаков'],
  ['Nunito', 'OFL-nunito.txt', 'The Nunito Project Authors', 'нет', 'урезан набор знаков'],
  ['Pangolin', 'OFL-pangolin.txt', 'The Pangolin Project Authors', 'нет', 'урезан набор знаков'],
  ['Rubik', 'OFL-rubik.txt', 'The Rubik Project Authors', 'нет', 'экземпляры вариативного шрифта (wght 400/700), урезан набор знаков, кернинг кириллицы подключён к latn'],
  ['Alice', 'OFL-alice.txt', 'The Alice Project Authors', 'нет', 'урезан набор знаков'],
  ['Cormorant Garamond', 'OFL-cormorant.txt', 'The Cormorant Project Authors', 'нет', 'урезан набор знаков'],
  ['Forum', 'OFL-forum.txt', 'Denis Masharov', 'ДА: "Forum"', 'без изменений'],
  ['Kurale', 'OFL-kurale.txt', 'The Kurale Project Authors', 'нет', 'урезан набор знаков'],
  ['Lora', 'OFL-lora.txt', 'The Lora Project Authors', 'ДА: "Lora"', 'без изменений'],
  ['Old Standard TT', 'OFL-oldstandard.txt', 'The Old Standard Project Authors', 'нет', 'урезан набор знаков'],
  ['Playfair Display', 'OFL-playfair.txt', 'The Playfair Display Project Authors', 'ДА: "Playfair Display"', 'без изменений'],
  ['Prata', 'OFL-prata.txt', 'The Prata Project Authors', 'нет', 'урезан набор знаков'],
  ['PT Serif', 'OFL-ptserif.txt', 'ParaType Ltd.', 'ДА: "PT Sans", "PT Serif", "ParaType"', 'без изменений'],
  ['Yeseva One', 'OFL-yeseva.txt', 'The Yeseva One Project Authors', 'ДА: "Yeseva"', 'без изменений'],
  ['Golos Text', 'OFL-golos.txt', 'The Golos Text Project Authors', 'нет', 'экземпляры вариативного шрифта (wght 400/700), урезан набор знаков'],
  ['Montserrat', 'OFL-montserrat.txt', 'The Montserrat Project Authors', 'нет', 'урезан набор знаков'],
  ['Oswald', 'OFL-oswald.txt', 'The Oswald Project Authors', 'нет', 'урезан набор знаков'],
  ['Raleway', 'OFL-raleway.txt', 'The Raleway Project Authors', 'ДА: "Raleway"', 'без изменений'],
  ['Roboto Condensed', 'OFL-robotocondensed.txt', 'The Roboto Project Authors', 'нет', 'экземпляры вариативного шрифта (wght 400/700), урезан набор знаков, кернинг кириллицы подключён к latn'],
  ['Tenor Sans', 'OFL-tenorsans.txt', 'Denis Masharov', 'ДА: "Tenor", "Tenor Sans"', 'без изменений'],
  ['Poiret One', 'OFL-poiretone.txt', 'The Poiret One Project Authors', 'нет', 'урезан набор знаков'],
  ['Ruslan Display', 'OFL-ruslan.txt', 'Oleg Snarsky, Denis Masharov, Vladimir Rabdu', 'ДА: "Ruslan", "Ruslan Display"', 'без изменений'],
  ['Underdog', 'OFL-underdog.txt', 'Sergey Steblina, Jovanny Lemonad', 'ДА: "Underdog"', 'без изменений'],
]
const fontFiles = fs.readdirSync(fontsDir).filter((x) => x.endsWith('.ttf')).length
const reserved = fonts.filter((x) => x[3] !== 'нет').map((x) => x[0])
const out = []
out.push(`THIRD_PARTY_NOTICES
===================

Sosnina Albums — программа вёрстки свадебных фотоальбомов. Исходный код программы
распространяется по лицензии GNU General Public License версии 3 (только этой версии,
GPL-3.0-only) с дополнительным условием по п. 7(b) — файлы LICENSE и NOTICE.md; версии
до 0.22.x включительно выпущены по лицензии MIT. В программу и её исполняемый файл
входят сторонние компоненты — Go-модули, npm-пакеты фронтенда и шрифты; их авторы,
версии и лицензии перечислены ниже, полные тексты лицензий приведены в конце файла.
Все они допускают распространение вместе с программой под GPL-3.0 (Apache-2.0
совместима с GPL версии 3). Список составлен по go list для сборок Windows, macOS
и Linux и по package-lock.json.

Содержание
  1. Go-модули в исполняемом файле
  2. npm-пакеты в сборке интерфейса
  3. Шрифты (SIL Open Font License 1.1)
  4. Прочее (системные библиотеки)
  5. Тексты лицензий

1. Go-модули в исполняемом файле (${section.go.length})
-------------------------------------------
Модуль без пометки входит во все три сборки (Windows, macOS, Linux).
${section.go.join('\n')}

2. npm-пакеты в сборке интерфейса (${section.npm.length})
---------------------------------------------
В frontend/package.json нет раздела dependencies: интерфейс собирает Vite, в программу
попадает рантайм Svelte и его зависимости. Инструменты разработки (Vite, ESLint,
Prettier, Vitest и др.) в программу не входят.
${section.npm.join('\n')}

3. Шрифты (${fonts.length} гарнитур, ${fontFiles} файлов; внутри программы: internal/text/fonts)
${'-'.repeat(40)}
Все — SIL Open Font License, Version 1.1 (текст — в разделе 5). Шрифты вшиты в программу
и без изменений лицензии распространяются вместе с ней.
`)
for (const [n, , a, rfn, ch] of fonts) out.push(`  ${n} — автор: ${a}\n    Reserved Font Name: ${rfn}; изменения: ${ch}`)
out.push(`
Гарнитуры без Reserved Font Name урезаны по набору знаков (subset). Гарнитуры с зарезервированным
именем (${reserved.join(', ')}) не изменялись: изменённый файл под тем же именем
распространять нельзя.

Заголовки авторских прав шрифтов:
`)
for (const [n, f] of fonts.map((x) => [x[0], x[1]])) {
  const t = fs.readFileSync(fontsDir + f, 'utf8').replace(/\r\n/g, '\n')
  out.push(
    `  [${n}]\n` +
      t
        .split('\n\n')[0]
        .split('\n')
        .map((l) => '    ' + l)
        .join('\n') +
      '\n',
  )
  const body = norm(t.slice(t.indexOf('-----')))
  // тело OFL общее: сверяем сравнением после нормализации
  add(n, '(шрифт)', 'OFL-1.1', body)
}
out.push(`4. Прочее (системные библиотеки)
---------------------------------
Не входят в поставку, подключаются из системы:
  Windows — Microsoft WebView2 (Evergreen Bootstrapper и WebView2Loader.dll встроены через
    github.com/wailsapp/wails/v2 и github.com/wailsapp/go-webview2); распространяется на условиях
    Microsoft: https://developer.microsoft.com/microsoft-edge/webview2/
  macOS — WKWebView (часть системы).
  Linux — GTK 3, WebKitGTK 4.1 и libsoup 3 (LGPL-2.1 и новее, BSD); динамическое подключение,
    ставятся из репозитория дистрибутива как зависимости пакета .deb.
`)
out.push(`5. Тексты лицензий
------------------`)
let i = 0
for (const [t, g] of groups) {
  i++
  out.push(`\n=== Лицензия ${i}: ${[...g.lic].join(' / ')} ===\nПрименяется к:\n${g.items.map((x) => '  - ' + x).join('\n')}\n\n${t}`)
}
fs.writeFileSync(
  W + '/THIRD_PARTY_NOTICES.txt',
  out
    .join('\n')
    .replace(/\r\n/g, '\n')
    .replace(/\n{4,}/g, '\n\n\n'),
)
console.log(
  section.go.length,
  section.npm.length,
  groups.size,
  [...groups.values()].map((g) => [...g.lic].join('/') + ':' + g.items.length).join(' | '),
)
