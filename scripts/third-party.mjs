// Генератор THIRD_PARTY_NOTICES.txt — единый файл лицензий: реальный состав exe + полные тексты.
// Go-модули: go-licenses (GOPATH/bin) с тегами сборки Wails; лицензия стандартной библиотеки — из GOROOT;
// npm-пакеты — список NPM ниже (рантайм Svelte и его зависимости; тексты — из frontend/node_modules);
// шрифты — internal/text/fonts. Только Node. Запуск из корня: node scripts/third-party.mjs
// (нужны go, go-licenses и frontend/node_modules; GOTMPDIR задан в окружении).
// При смене состава сборки обновить NPM и проверить diff.
import fs from 'fs'
import os from 'os'
import path from 'path'
import { execFileSync } from 'child_process'
import { fileURLToPath } from 'url'

const R = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..').replace(/\\/g, '/')
const W = R
const TAGS = 'desktop,production'
const goLicenses = path.join(process.env.GOPATH || path.join(os.homedir(), 'go'), 'bin', 'go-licenses')
const run = (cmd, args) =>
  execFileSync(cmd, args, { cwd: R, encoding: 'utf8', maxBuffer: 1 << 26, env: { ...process.env, GOFLAGS: `-tags=${TAGS}` } })
const GOROOT = run('go', ['env', 'GOROOT']).trim()
const tmp = fs.mkdtempSync(path.join(process.env.GOTMPDIR || os.tmpdir(), 'notices-'))
const glReport = run(goLicenses, ['report', './...'])
run(goLicenses, ['save', './...', '--save_path', path.join(tmp, 'save'), '--force'])
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
const mods = {}
for (const m of fs.readFileSync(W + '/go.mod', 'utf8').matchAll(/^\s*(?:require\s+)?(\S+) (v\S+)/gm)) mods[m[1]] = m[2]
const MIT_BODY = `Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.`
const groups = new Map() // text -> {lic, items}
function add(name, ver, lic, text) {
  const t = norm(text)
  if (!groups.has(t)) groups.set(t, { lic: new Set(), items: [] })
  const g = groups.get(t)
  g.lic.add(lic)
  g.items.push(`${name} ${ver}`)
}
const section = { go: [], npm: [] }
for (const line of glReport.split('\n').filter(Boolean)) {
  const [pkg, , lic] = line.trim().split(',')
  // Свой модуль, включая чужой Go-код внутри frontend/node_modules (flatted кладёт пакет golang/): в exe не входит.
  if (pkg === 'github.com/IliaSA101/sosninaalbums' || pkg.startsWith('github.com/IliaSA101/sosninaalbums/')) continue
  const modName = Object.keys(mods)
    .filter((k) => pkg === k || pkg.startsWith(k + '/'))
    .sort((a, b) => b.length - a.length)[0]
  const dir = tmp + '/save/' + pkg
  const f = fs.readdirSync(dir).find((x) => /^LICEN[CS]E/i.test(x))
  const text = fs.readFileSync(dir + '/' + f, 'utf8')
  const label = `${modName} ${mods[modName]} (${lic})`
  if (!section.go.includes(label)) {
    section.go.push(label)
    add(modName, mods[modName], lic, text)
  } else if (!groups.get(norm(text))) add(modName, mods[modName], lic, text)
}
// Стандартная библиотека
section.go.push('Стандартная библиотека Go go1.26.9 (BSD-3-Clause)')
add('Стандартная библиотека Go', 'go1.26.9', 'BSD-3-Clause', fs.readFileSync(GOROOT + '/LICENSE', 'utf8'))
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
  ['Great Vibes', 'OFL-greatvibes.txt', 'The Great Vibes Pro Project Authors', 'нет', 'урезан набор знаков'],
  ['Marck Script', 'OFL-marck.txt', 'Denis Masharov и Marck Fogel', 'ДА: "Marck Script"', 'без изменений'],
  ['Pacifico', 'OFL-pacifico.txt', 'The Pacifico Project Authors', 'нет', 'урезан набор знаков'],
  ['Amatic SC', 'OFL-amaticsc.txt', 'The Amatic SC Project Authors', 'нет', 'урезан набор знаков'],
  ['Comfortaa', 'OFL-comfortaa.txt', 'The Comfortaa Project Authors', 'ДА: "Comfortaa"', 'без изменений'],
  ['Neucha', 'OFL-neucha.txt', 'Jovanny Lemonad', 'нет', 'урезан набор знаков'],
  ['Nunito', 'OFL-nunito.txt', 'The Nunito Project Authors', 'нет', 'урезан набор знаков'],
  ['Cormorant Garamond', 'OFL-cormorant.txt', 'The Cormorant Project Authors', 'нет', 'урезан набор знаков'],
  ['Forum', 'OFL-forum.txt', 'Denis Masharov', 'ДА: "Forum"', 'без изменений'],
  ['Lora', 'OFL-lora.txt', 'The Lora Project Authors', 'ДА: "Lora"', 'без изменений'],
  ['Playfair Display', 'OFL-playfair.txt', 'The Playfair Display Project Authors', 'ДА: "Playfair Display"', 'без изменений'],
  ['PT Serif', 'OFL-ptserif.txt', 'ParaType Ltd.', 'ДА: "PT Sans", "PT Serif", "ParaType"', 'без изменений'],
  ['Yeseva One', 'OFL-yeseva.txt', 'The Yeseva One Project Authors', 'ДА: "Yeseva"', 'без изменений'],
  ['Montserrat', 'OFL-montserrat.txt', 'The Montserrat Project Authors', 'нет', 'урезан набор знаков'],
  ['Oswald', 'OFL-oswald.txt', 'The Oswald Project Authors', 'нет', 'урезан набор знаков'],
  ['Raleway', 'OFL-raleway.txt', 'The Raleway Project Authors', 'ДА: "Raleway"', 'без изменений'],
]
const fontFiles = fs.readdirSync(fontsDir).filter((x) => x.endsWith('.ttf')).length
const reserved = fonts.filter((x) => x[3] !== 'нет').map((x) => x[0])
const out = []
out.push(`THIRD_PARTY_NOTICES
===================

Sosnina Albums — программа вёрстки свадебных фотоальбомов. Исходный код программы
распространяется по лицензии MIT (файл LICENSE). В программу и её исполняемый файл
входят сторонние компоненты — Go-модули, npm-пакеты фронтенда и шрифты; их авторы,
версии и лицензии перечислены ниже, полные тексты лицензий приведены в конце файла.
Список составлен go-licenses и по package-lock.json для версии 0.8.0.

Содержание
  1. Go-модули в исполняемом файле
  2. npm-пакеты в сборке интерфейса
  3. Шрифты (SIL Open Font License 1.1)
  4. Прочее (Microsoft WebView2)
  5. Тексты лицензий

1. Go-модули в исполняемом файле (${section.go.length})
-------------------------------------------
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
out.push(`4. Прочее
---------
Microsoft WebView2 (Evergreen Bootstrapper и WebView2Loader.dll) встроены через
github.com/wailsapp/wails/v2 и github.com/wailsapp/go-webview2; распространяются на условиях
Microsoft: https://developer.microsoft.com/microsoft-edge/webview2/
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
fs.rmSync(tmp, { recursive: true, force: true })
console.log(
  section.go.length,
  section.npm.length,
  groups.size,
  [...groups.values()].map((g) => [...g.lic].join('/') + ':' + g.items.length).join(' | '),
)
