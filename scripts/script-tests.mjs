// Тесты скриптов одним списком — тем же, что гоняет CI: `node scripts/script-tests.mjs [--public]`.
// node --test скриптов, release-check, python unittest (win, mac, guide), test_lib.sh, test_deb.sh.
// --public: сначала публичное дерево (без scripts/publish-exclude.txt, public-tree.mjs) в .gotmp/public-tree и тесты на нём,
// как их видит CI main: 0.25.0 — test_qakit ждал docs/qa, которого в публичном дереве нет.
// Выход: код первого упавшего шага (0 — все зелёные; пропуск без инструмента — WARN, не провал, кроме NSIS: CI его требует).
import { spawnSync } from 'node:child_process'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { exportPublic } from './public-tree.mjs'

export const NO_DESKTOP = ['DISPLAY', 'WAYLAND_DISPLAY', 'XDG_CURRENT_DESKTOP', 'DESKTOP_SESSION']

/**
 * Какие тесты скриптов затронуты правкой (для precheck): changed — пути от корня через «/», exists(путь) — есть ли файл.
 * node — файлы scripts/*.test.mjs (правка scripts/x.mjs берёт scripts/x.test.mjs, если он есть; правка workflow — все тесты),
 * win — тесты упаковки scripts/win (правка scripts/win/** или workflow), deb — test_deb.sh (правка scripts/linux/**).
 * Причина: правка одного workflow ломала test_jobs_and_needs, а precheck молчал (0.25.0, 1fda7bbe и ed6c5f40).
 */
export function affectedScriptTests(changed, exists) {
  const node = new Set()
  let all = false
  let win = false
  let deb = false
  for (const f of changed) {
    if (f.startsWith('.github/workflows/')) {
      all = true
      win = true
    }
    if (f.startsWith('scripts/win/')) win = true
    if (f.startsWith('scripts/linux/')) deb = true
    const m = f.match(/^scripts\/([\w-]+)(\.test)?\.mjs$/)
    if (!m) continue
    const test = `scripts/${m[1]}.test.mjs`
    if (exists(test)) node.add(test)
  }
  return { node: all ? ['scripts/*.test.mjs'] : [...node].sort(), win, deb }
}

/**
 * Список шагов. ctx: {platform, python (команда или ''), hasDocx, hasMakensis, hasDpkg}.
 * Шаг: {name, cmd, args, env?, skip?: текст WARN, fail?: текст провала без запуска, ci: [файл workflow, строка из его run:]}.
 * ci — страж расхождения: тест сверяет, что строка есть в workflow (скрипт-tests.test.mjs).
 */
export function buildSteps(ctx) {
  const py = ctx.python
  const pyStep = (name, dir, ci, extra = {}) => ({
    name,
    cmd: py || 'python3',
    args: ['-I', '-m', 'unittest', 'discover', '-s', dir],
    ci: ['ci.yml', ci],
    ...(py ? {} : { skip: `нет python3 и python в PATH: ${name} не запущен` }),
    ...extra,
  })
  const steps = [
    { name: 'node --test скриптов', cmd: 'node', args: ['--test', 'scripts/*.test.mjs'], ci: ['ci.yml', 'node --test "scripts/*.test.mjs"'] },
    { name: 'release-check', cmd: 'node', args: ['scripts/release-check.mjs'], ci: ['ci.yml', 'node scripts/release-check.mjs'] },
    // CI=true: без makensis тесты упаковки падают, а не пропускаются — как в CI
    pyStep('python scripts/win', 'scripts/win', 'python -I -m unittest discover -s scripts/win', {
      env: { CI: 'true' },
      ...(py && !ctx.hasMakensis ? { fail: 'нет makensis (sudo apt install nsis): в CI он обязателен, тесты упаковки без него не идут' } : {}),
    }),
    pyStep('python scripts/mac', 'scripts/mac', 'python -I -m unittest discover -s scripts/mac'),
    { name: 'scripts/mac/test_lib.sh', cmd: 'bash', args: ['scripts/mac/test_lib.sh'], ci: ['ci.yml', 'bash scripts/mac/test_lib.sh'] },
  ]
  // guide: нужен python-docx (в CI ставится по хешам), без него тесты не имеют смысла
  const guide = pyStep('python scripts/guide', 'scripts/guide', 'python -I -m unittest discover -s scripts/guide')
  if (py && !ctx.hasDocx) guide.skip = 'нет python-docx (pip install -r scripts/guide/requirements.txt): scripts/guide не запущен'
  steps.push(guide)
  // test_deb.sh в CI идёт на раннере без рабочего стола: xdg-mime при DISPLAY смотрит не туда (0.25.0)
  const deb = { name: 'scripts/linux/test_deb.sh', cmd: 'bash', args: ['scripts/linux/test_deb.sh'], unset: NO_DESKTOP, ci: ['release.yml', 'bash scripts/linux/test_deb.sh'] }
  if (ctx.platform === 'win32') deb.skip = 'test_deb.sh: на Windows нет dpkg-deb, пропуск'
  else if (!ctx.hasDpkg) deb.skip = 'test_deb.sh: нет dpkg-deb, пропуск'
  steps.push(deb)
  return steps
}

const ok = (cmd, args) => spawnSync(cmd, args, { encoding: 'utf8', stdio: 'ignore' }).status === 0

/** Окружение машины для buildSteps. */
export function detect(platform = process.platform) {
  const python = ['python3', 'python'].find((c) => ok(c, ['--version'])) || ''
  return {
    platform,
    python,
    hasDocx: python ? ok(python, ['-I', '-c', 'import docx']) : false,
    hasMakensis: ok('makensis', ['-VERSION']),
    hasDpkg: platform !== 'win32' && ok('dpkg-deb', ['--version']),
  }
}

/** Запуск шага: {status: 'ok'|'warn'|'fail', code, out}. */
export function runStep(step, { cwd, env, spawn = spawnSync }) {
  if (step.skip) return { status: 'warn', code: 0, out: step.skip }
  if (step.fail) return { status: 'fail', code: 1, out: step.fail }
  const e = { ...env, ...step.env, PYTHONDONTWRITEBYTECODE: '1' }
  for (const k of step.unset || []) delete e[k]
  const r = spawn(step.cmd, step.args, { cwd, env: e, encoding: 'utf8', maxBuffer: 1 << 28 })
  const out = `${r.stdout || ''}${r.stderr || ''}`
  if (r.error) return { status: 'fail', code: 1, out: `${step.cmd}: ${r.error.message}` }
  const code = r.status ?? 1
  return { status: code === 0 ? 'ok' : 'fail', code, out }
}

/** Все шаги по порядку (упавший не останавливает остальные); возвращает код первого провала, 0 — если провалов нет. */
export function runAll(steps, opts, report = () => {}) {
  let first = 0
  for (const step of steps) {
    const t = Date.now()
    const r = runStep(step, opts)
    report(step, r, `${((Date.now() - t) / 1000).toFixed(1)}s`)
    if (r.status === 'fail' && !first) first = r.code
  }
  return first
}

function main() {
  const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
  const gotmp = path.join(root, '.gotmp')
  let cwd = root
  if (process.argv.includes('--public')) {
    const n = exportPublic(root, '.gotmp/public-tree')
    cwd = path.join(gotmp, 'public-tree')
    console.log(`публичное дерево: ${n} файлов (.gotmp/public-tree)`)
  }
  // Временные файлы тестов — в .gotmp (антивирус, чистка), не в системный каталог
  // LC_ALL как на раннере CI: в русской локали printf даёт «3,75» и тесты lib.sh падают.
  // TMPDIR не трогаем: тесты release-sign/release-check ждут, что системный temp лежит вне .gotmp и вне репозитория
  const env = { ...process.env, LC_ALL: process.platform === 'darwin' ? 'en_US.UTF-8' : 'C.UTF-8' }
  const first = runAll(buildSteps(detect()), { cwd, env }, (step, r, time) => {
    console.log(`${{ ok: 'OK  ', warn: 'WARN', fail: 'FAIL' }[r.status]} ${step.name} (${time})`)
    if (r.status === 'ok') return
    const lines = r.out.split('\n').filter((l) => l.trim())
    for (const l of r.status === 'fail' ? lines.slice(-40) : lines) console.log(`     ${l}`)
  })
  console.log(first ? `ИТОГО: FAIL (код ${first})` : 'ИТОГО: OK')
  process.exit(first)
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
