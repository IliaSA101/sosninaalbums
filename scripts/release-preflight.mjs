#!/usr/bin/env node
// Подготовка выпуска до метки: `node scripts/release-preflight.mjs X.Y.Z [--rebuild] [--against <id прогона Release dry_run>] [--key <pem>]`.
// Уроки 0.25.0: makensis не оказалось на машине подписи, а установщик не воспроизвёлся (mtime BMP попал в байты) — обе находки
// пришли после метки, когда её уже не перевесить. Здесь то же проверяется заранее и выводом «OK/FAIL что — как исправить».
//
// Быстрые проверки: makensis = NSIS_CI и раннер pack в release.yml закреплён (иначе версия NSIS в CI уплывёт), вход в gh,
// файл ключа подписи (только существование, размер и права — содержимое не читается и не печатается), Node и Go как в CI,
// wails как в go.mod, рабочая копия чистая, метки vX.Y.Z ещё нет, release-check --version зелёный, ассет примера набора
// (спека 142) опубликован в библиотеке и совпадает длиной с zip в репозитории.
// --rebuild (2–6 мин): установщик собирается в чистой копии коммита HEAD (release-rebuild с ref) и должен воспроизводиться:
// после сдвига mtime всех входов на год назад установщик из тех же входов обязан совпасть байт в байт.
// --against <id>: установщик из артефакта packages прогона CI сверяется с локальной пересборкой на коммите прогона (headSha)
// побайтно — ровно сверка release-sign, но до метки. Подразумевает --rebuild. Код выхода ≠ 0 при любом FAIL.
import { spawnSync } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { BMP_FILES, buildEnv, defaultRun, rebuildSetup } from './release-rebuild.mjs'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
/** Версия NSIS на раннере ubuntu-24.04 (apt nsis 3.09-4ubuntu1): с ней CI собирает установщик, release-sign пересобирает здесь. */
export const NSIS_CI = 'v3.09-4'
/** Раннер задания pack в release.yml: закреплён, чтобы NSIS_CI не уплыл с переездом ubuntu-latest. */
export const PACK_RUNNER = 'ubuntu-24.04'
export const KEY_REL = path.join('docs', 'private', 'release-key.pem')
const VER_RE = /^\d{1,4}\.\d{1,4}\.\d{1,4}$/
const YEAR = 365 * 86400

// ---- разбор файлов проекта ----

/** runs-on задания job в тексте workflow (отступ заданий — два пробела); null — задания или строки нет. */
export function jobRunner(yaml, job) {
  const lines = yaml.replace(/\r\n/g, '\n').split('\n')
  const at = lines.findIndex((l) => l === `  ${job}:`)
  if (at < 0) return null
  for (let i = at + 1; i < lines.length && !/^ {0,2}\S/.test(lines[i]); i++) {
    const m = lines[i].match(/^\s+runs-on:\s*(\S+)/)
    if (m) return m[1].replace(/^["']|["']$/g, '')
  }
  return null
}

export const nodeWanted = (ciYaml) => ciYaml.match(/node-version:\s*["']?(\d+\.\d+\.\d+)["']?/)?.[1] ?? null
export const goToolchain = (goMod) => goMod.match(/^toolchain go(\d+\.\d+\.\d+)\s*$/m)?.[1] ?? null
export const wailsWanted = (goMod) => goMod.match(/github\.com\/wailsapp\/wails\/v2 (v\d+\.\d+\.\d+)/)?.[1] ?? null

/** Входы установщика, чей mtime мог бы попасть в байты (File в installer.nsi): exe, значок, картинки мастера. */
export const installerInputs = (version) => [
  `build/bin/SosninaAlbums-${version}.exe`,
  'build/windows/icon.ico',
  ...BMP_FILES,
  ...[100, 125, 150, 175].map((d) => `scripts/win/wizard-${d}.bmp`),
]

const ok = (what, hint = '') => ({ ok: true, what, hint })
const fail = (what, hint) => ({ ok: false, what, hint })

/** Префикс ассетов библиотеки: то же, что fetch.AssetPrefix в Go (спека 142, 0.30-142-42). */
export const LIBRARY_ASSETS = 'https://github.com/IliaSA101/sosninaalbums-library/releases/download/'

/** Адрес ассета примера по имени zip: wedding-navy-watercolor-demo-1.zip → <префикс>wedding-navy-watercolor-demo-v1/<файл>; null — имя не того вида. */
export function demoAssetUrl(zipName) {
  const m = zipName.match(/^(.+-demo)-([1-9]\d*)\.zip$/)
  return m ? `${LIBRARY_ASSETS}${m[1]}-v${m[2]}/${zipName}` : null
}

/**
 * Ассет примера (спека 142, design §9) опубликован до метки: 200 и длина как у zip в репозитории. Компактная редакция
 * докачивает пример из библиотеки — без ассета «Скачать» в готовом выпуске ведёт в пустоту. demoZip() -> {name, size}|null.
 */
export function checkDemoAsset(deps) {
  const zip = deps.demoZip()
  if (!zip) return fail('ассет примера', 'в internal/demopack нет ровно одного zip примера')
  const url = demoAssetUrl(zip.name)
  if (!url) return fail(`ассет примера ${zip.name}`, 'имя zip не вида <набор>-demo-<N>.zip')
  const r = deps.exec('curl', ['-sIL', '--max-time', '30', url])
  if (r.code !== 0) return fail(`ассет примера ${url}`, `curl не отработал (код ${r.code}): нет сети или ассет не опубликован`)
  // После перенаправлений (GitHub → CDN) значим последний ответ
  const blocks = r.out.replace(/\r\n/g, '\n').split(/\n(?=HTTP\/)/)
  const last = blocks[blocks.length - 1]
  const status = last.match(/^HTTP\/\S+ (\d+)/)?.[1]
  const len = Number(last.match(/^content-length:\s*(\d+)/im)?.[1] ?? NaN)
  if (status !== '200') return fail(`ассет примера ${url}`, `ответ ${status ?? 'не разобран'}: опубликуйте релиз ${url.split('/').slice(-2)[0]} в sosninaalbums-library до метки`)
  if (len !== zip.size) return fail(`ассет примера ${url}`, `длина в сети ${len}, у zip в репозитории ${zip.size}: ассет другой версии`)
  return ok(`ассет примера опубликован (${zip.name}, ${len} байт)`)
}

/** Первый разошедшийся байт двух буферов; -1 — равны. */
export function firstDiff(a, b) {
  const n = Math.min(a.length, b.length)
  for (let i = 0; i < n; i++) if (a[i] !== b[i]) return i
  return a.length === b.length ? -1 : n
}

// ---- быстрые проверки ----

/** Файл ключа: существует, обычный, не пустой, вне Windows без прав группы и остальных. stat — fs.statSync (бросает при отсутствии). */
export function checkKey(file, stat, platform) {
  let st
  try {
    st = stat(file)
  } catch {
    return fail(`ключ подписи ${file}`, 'нет файла: достаньте ключ из копии вне машины (--new-key создаёт новую пару, но её открытый ключ придётся вшить в программу)')
  }
  if (!st.isFile() || st.size <= 0) return fail(`ключ подписи ${file}`, 'не файл или пустой')
  if (platform !== 'win32' && (st.mode & 0o077) !== 0) return fail(`ключ подписи ${file}`, `права ${(st.mode & 0o777).toString(8)}: нужен chmod 600`)
  return ok(`ключ подписи на месте (${st.size} байт, содержимое не читалось)`)
}

/** Быстрые проверки (секунды). deps: exec(cmd,args)->{code,out}, read(rel)->текст|null, stat(абс. путь), platform, nodeVersion. */
export function quickChecks(version, deps, keyFile) {
  const res = []
  const mk = deps.exec('makensis', ['-VERSION'])
  const have = mk.out.trim()
  if (mk.code !== 0) res.push(fail('makensis', 'не найден: sudo apt install nsis'))
  else if (have !== NSIS_CI) res.push(fail(`makensis ${have}`, `в CI ${NSIS_CI}: установщик не сойдётся с CI (sudo apt install nsis=… или обновить NSIS_CI вместе с раннером)`))
  else res.push(ok(`makensis ${have} = CI`))

  const rel = deps.read('.github/workflows/release.yml')
  const runner = rel === null ? null : jobRunner(rel, 'pack')
  if (runner === PACK_RUNNER) res.push(ok(`pack в release.yml на ${runner}`))
  else res.push(fail(`pack в release.yml на ${runner ?? 'неизвестном раннере'}`, `ждали ${PACK_RUNNER}: на ubuntu-latest версия NSIS меняется сама, NSIS_CI устареет`))

  const gh = deps.exec('gh', ['auth', 'status'])
  res.push(gh.code === 0 ? ok('gh: вход выполнен') : fail('gh: не вошли в GitHub', 'gh auth login'))
  res.push(checkKey(keyFile, deps.stat, deps.platform))

  const ci = deps.read('.github/workflows/ci.yml')
  const wantNode = ci === null ? null : nodeWanted(ci)
  const haveNode = deps.nodeVersion.replace(/^v/, '')
  if (wantNode === null) res.push(fail('Node', 'в ci.yml не найден node-version'))
  else if (haveNode !== wantNode) res.push(fail(`Node ${haveNode}`, `в CI ${wantNode} (ci.yml node-version)`))
  else res.push(ok(`Node ${haveNode} = CI`))

  const mod = deps.read('go.mod')
  const wantGo = mod === null ? null : goToolchain(mod)
  const goOut = deps.exec('go', ['version'])
  const haveGo = goOut.out.match(/go(\d+\.\d+\.\d+)/)?.[1]
  if (!wantGo) res.push(fail('Go', 'в go.mod нет строки toolchain'))
  else if (goOut.code !== 0 || haveGo !== wantGo) res.push(fail(`Go ${haveGo ?? 'не найден'}`, `go.mod просит go${wantGo}`))
  else res.push(ok(`Go ${haveGo} = go.mod`))

  const wantWails = mod === null ? null : wailsWanted(mod)
  const w = deps.exec('wails', ['version'])
  const haveWails = w.out.match(/v\d+\.\d+\.\d+/)?.[0]
  if (!wantWails) res.push(fail('wails', 'в go.mod нет github.com/wailsapp/wails/v2'))
  else if (w.code !== 0) res.push(fail('wails не найден', `go install github.com/wailsapp/wails/v2/cmd/wails@${wantWails}`))
  else if (haveWails !== wantWails) res.push(fail(`wails ${haveWails}`, `go.mod просит ${wantWails}: go install github.com/wailsapp/wails/v2/cmd/wails@${wantWails}`))
  else res.push(ok(`wails ${haveWails} = go.mod`))

  const st = deps.exec('git', ['status', '--porcelain'])
  res.push(st.code === 0 && !st.out.trim() ? ok('рабочая копия чистая') : fail('рабочая копия не чиста', st.code === 0 ? 'закоммитьте или уберите правки: пересборка идёт из коммита HEAD' : 'git status не отработал'))
  const tag = deps.exec('git', ['tag', '-l', `v${version}`])
  res.push(tag.out.trim() ? fail(`метка v${version} уже есть`, 'метку не перевешиваем: следующая версия или git tag -d до публикации') : ok(`метки v${version} ещё нет`))
  const rc = deps.exec('node', ['scripts/release-check.mjs', '--version', version])
  res.push(rc.code === 0 ? ok(`release-check ${version}`) : fail(`release-check ${version}`, rc.out.trim().split('\n').slice(0, 6).join(' | ')))
  res.push(checkDemoAsset(deps))
  return res
}

// ---- воспроизводимость и сверка с CI ----

/**
 * Установщик из тех же входов, но с mtime на год раньше, обязан совпасть с собранным. Выполняется в копии коммита (src), где
 * лежат результаты сборки. io: run(cmd,args,{cwd,env}), touch(file,sec), mtime(file)->sec|null, read(file)->Buffer, now() мс.
 * Возвращает {ok, culprits, at}: culprits — входы, чей mtime меняет байты (проверяются по одному только при расхождении).
 */
export function checkReproducible({ src, version, setup, io, env }) {
  const inputs = installerInputs(version).filter((f) => io.mtime(path.join(src, f)) !== null)
  const out = path.join(src, 'build', 'repro')
  const exe = path.join(src, installerInputs(version)[0])
  const built = path.join(out, path.basename(setup))
  const base = io.read(setup)
  const old = Math.floor(io.now() / 1000) - YEAR
  const rebuild = (files) => {
    const saved = files.map((f) => [f, io.mtime(path.join(src, f))])
    try {
      for (const f of files) io.touch(path.join(src, f), old)
      io.run('bash', ['scripts/win/installer.sh', '--exe', exe, '--version', version, '--out', out], { cwd: src, env })
      return io.read(built)
    } finally {
      for (const [f, sec] of saved) io.touch(path.join(src, f), sec)
    }
  }
  const all = rebuild(inputs)
  const at = firstDiff(base, all)
  if (at < 0) return { ok: true, culprits: [], at }
  const culprits = inputs.filter((f) => firstDiff(base, rebuild([f])) >= 0)
  return { ok: false, culprits, at }
}

/** Побайтная сверка установщика CI с локальным: результат одной проверки. */
export function compareWithCi(name, ci, local, runId) {
  const at = firstDiff(ci, local)
  if (at < 0) return ok(`установщик ${name} совпал с CI (прогон ${runId}) побайтно, ${ci.length} байт`)
  return fail(
    `установщик ${name} не совпал с CI (прогон ${runId})`,
    `CI ${ci.length} байт, здесь ${local.length}; первое расхождение — байт ${at}. Сверьте тулчейны с release.yml (Go, Wails, Node, CGO_ENABLED=0, makensis) и mtime входов; метку ставить рано`,
  )
}

/**
 * Пересборка (--rebuild / --against). deps: exec, rebuild({ref,version,verify})->путь, readBuf(file)->Buffer, download(id)->путь установщика|null,
 * io (для checkReproducible), env. Возвращает список результатов.
 */
export function rebuildChecks(version, opts, deps) {
  const res = []
  let ref = deps.exec('git', ['rev-parse', 'HEAD']).out.trim()
  let ci = null
  if (opts.against) {
    const v = deps.exec('gh', ['run', 'view', opts.against, '--json', 'headSha,conclusion'])
    let info
    try {
      info = JSON.parse(v.out)
    } catch {
      return [fail(`gh run view ${opts.against}`, 'не получены данные прогона (id верный? вход в gh?)')]
    }
    if (info.conclusion !== 'success') return [fail(`прогон ${opts.against}: ${info.conclusion || 'не завершён'}`, 'сверять можно только с успешным Release dry_run')]
    if (deps.exec('git', ['cat-file', '-e', `${info.headSha}^{commit}`]).code !== 0) return [fail(`коммит прогона ${info.headSha.slice(0, 7)} не найден здесь`, 'git fetch или отправьте ветку winqa из этой копии')]
    ref = info.headSha
    ci = deps.download(opts.against, version)
    if (!ci) return [fail(`артефакт packages прогона ${opts.against}`, `не скачан или нет установщика SosninaAlbums-${version}-windows-x64-setup.exe (артефакты хранятся 7 дней)`)]
  }
  let repro = null
  let rebuilt
  try {
    rebuilt = deps.rebuild({
      ref,
      version,
      verify: ({ src, setup }) => {
        repro = checkReproducible({ src, version, setup, io: deps.io, env: deps.env })
      },
    })
  } catch (e) {
    return [fail(`пересборка ${ref.slice(0, 7)}`, e.message)]
  }
  res.push(ok(`установщик пересобран из коммита ${ref.slice(0, 7)}`))
  if (repro?.ok) res.push(ok('установщик воспроизводится: mtime входов на год раньше — те же байты'))
  else if (repro) res.push(fail('установщик не воспроизводится при другом mtime входов', `расходится с байта ${repro.at}; виноваты: ${repro.culprits.join(', ') || 'совокупность входов'} (в .nsi — SetDateSave off; mtime BMP/exe попадает в заголовок)`))
  if (ci) res.push(compareWithCi(path.basename(rebuilt), deps.readBuf(ci), deps.readBuf(rebuilt), opts.against))
  return res
}

/** Текст вывода и код выхода по результатам. */
export function report(results) {
  const lines = results.map((r) => (r.ok ? `OK   ${r.what}` : `FAIL ${r.what}${r.hint ? ` — ${r.hint}` : ''}`))
  const bad = results.filter((r) => !r.ok).length
  lines.push(bad ? `ИТОГО: FAIL (${bad} из ${results.length})` : `ИТОГО: OK (${results.length})`)
  return { text: lines.join('\n'), code: bad ? 1 : 0 }
}

// ---- реальные зависимости ----

function realExec(cmd, args) {
  const r = spawnSync(cmd, args, { cwd: root, encoding: 'utf8', maxBuffer: 1 << 26 })
  if (r.error) return { code: 127, out: r.error.message }
  return { code: r.status ?? 1, out: `${r.stdout || ''}${r.stderr || ''}` }
}

const findFile = (dir, name) => {
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, e.name)
    if (e.isDirectory()) {
      const f = findFile(p, name)
      if (f) return f
    } else if (e.name === name) return p
  }
  return null
}

function realDeps(work) {
  const env = buildEnv(process.env)
  // сборка exe занимает все ядра — через очередь heavy.mjs; остальные команды (npm ci, installer.sh) — напрямую
  const run = (cmd, args, o) => (cmd === 'bash' && args[0] === 'scripts/build.sh' ? defaultRun('node', [path.join(root, 'scripts', 'heavy.mjs'), cmd, ...args], o) : defaultRun(cmd, args, o))
  return {
    exec: realExec,
    read: (rel) => (fs.existsSync(path.join(root, rel)) ? fs.readFileSync(path.join(root, rel), 'utf8') : null),
    stat: fs.statSync,
    demoZip: () => {
      const dir = path.join(root, 'internal', 'demopack')
      const zips = fs.existsSync(dir) ? fs.readdirSync(dir).filter((n) => n.endsWith('.zip')) : []
      return zips.length === 1 ? { name: zips[0], size: fs.statSync(path.join(dir, zips[0])).size } : null
    },
    platform: process.platform,
    nodeVersion: process.version,
    env,
    io: {
      run,
      touch: (f, sec) => fs.utimesSync(f, sec, sec),
      mtime: (f) => (fs.existsSync(f) ? fs.statSync(f).mtimeMs / 1000 : null),
      read: (f) => fs.readFileSync(f),
      now: () => Date.now(),
    },
    readBuf: (f) => fs.readFileSync(f),
    rebuild: ({ ref, version, verify }) => rebuildSetup({ tag: `v${version}`, version, work, repo: root, ref, env: process.env, run, verify }),
    download: (id, version) => {
      const dir = path.join(work, `ci-${id}`)
      fs.rmSync(dir, { recursive: true, force: true })
      const r = realExec('gh', ['run', 'download', id, '-n', 'packages', '-D', dir])
      return r.code === 0 && fs.existsSync(dir) ? findFile(dir, `SosninaAlbums-${version}-windows-x64-setup.exe`) : null
    },
  }
}

function main(argv) {
  const version = argv.find((a) => VER_RE.test(a))
  const val = (f) => (argv.includes(f) ? argv[argv.indexOf(f) + 1] : undefined)
  const against = val('--against')
  if (!version || (argv.includes('--against') && (!against || against.startsWith('--')))) {
    console.error('использование: node scripts/release-preflight.mjs X.Y.Z [--rebuild] [--against <id прогона>] [--key <pem>]')
    process.exit(2)
  }
  const work = path.join(root, '.gotmp', 'release-preflight')
  fs.rmSync(work, { recursive: true, force: true })
  fs.mkdirSync(work, { recursive: true })
  const deps = realDeps(work)
  const keyFile = path.resolve(val('--key') ?? path.join(root, KEY_REL))
  const results = quickChecks(version, deps, keyFile)
  if (argv.includes('--rebuild') || against) {
    results.push(...rebuildChecks(version, { against }, deps))
  }
  const r = report(results)
  console.log(r.text)
  process.exit(r.code)
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main(process.argv.slice(2))
