// Тесты release-check на маленьком макете репозитория во временной папке: `node --test scripts/`.
import assert from 'node:assert/strict'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'
import { NOTICE_MUST, changelogSection, checkAll, checkInstaller, checkLicense, checkNeeds, checkReadmeFeatures, checkReadmeMedia, parseJobs, workflowExec } from './release-check.mjs'

const REPO = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')

// Настоящие LICENSE и NOTICE.md репозитория: макет проверяется на тех же файлах, что уйдут в выпуск.
const LICENSE = fs.readFileSync(path.join(REPO, 'LICENSE'), 'utf8')
const NOTICE = fs.readFileSync(path.join(REPO, 'NOTICE.md'), 'utf8')

const YML = `name: Release
jobs:
  prepare:
    runs-on: ubuntu-latest
  windows:
    needs: prepare
  macos:
    needs: prepare
  pack:
    needs: [prepare, windows, macos]
    steps:
      - run: bash scripts/win/installer.sh --exe x --version 1.2.0 --out out
  mac-check:
    needs:
      - prepare
      - pack
    steps:
      - run: go test ./internal/layout -run '^TestGoldenFirst(16)?$' -count=1
  win-check:
    needs: [prepare, pack]
    steps:
      - run: pwsh scripts/win/installer-check.ps1 -Setup a -Next b -Version 1.2.0
  release:
    needs: [prepare, pack, mac-check, win-check]
`

const ID_A = 'aaaaaaaa-1111-2222-3333-444444444444'
const ID_B = 'bbbbbbbb-1111-2222-3333-444444444444'
const URL = (id) => `https://github.com/user-attachments/assets/${id}\n`
const MEDIA = URL(ID_A) + URL(ID_B) + URL(ID_A) + URL(ID_B)

const FEATURES = '\n## Возможности\n\n- Старое (1.1): раз.\n- Новое (1.2): два.\n\n## Сборка\n'

const FILES = {
  'docs/media/demo.webp': 'w',
  'docs/media/screenshot.jpg': 'j',
  'docs/media/CREDITS.md': '# Фото\n',
  'wails.json': JSON.stringify({ info: { productVersion: '1.2.0' } }),
  'README.md': '# X\n\n- Status: version 1.2.0, actively developed\n' + MEDIA + FEATURES,
  'frontend/src/shell/dialogs/whatsnew.ts': "export const WHATS_NEW_VERSION = '1.2.0'\nexport const WHATS_NEW: readonly string[] = [\n  'раз',\n]\n",
  'CHANGELOG.md': '# История\n\n## 1.2.0 — 2026-01-01\n\n### Добавлено\n\n- Новое (051)\n- Ещё (052)\n\n## 1.1.0 — 2025-12-01\n\n- Старое\n',
  '.github/workflows/release.yml': YML,
  '.github/release-notes.md': 'Скачайте `SosninaAlbums-{{VERSION}}-windows-x64-setup.exe`. Без установки: `SosninaAlbums-{{VERSION}}.exe` в zip.\n',
  'docs/install/guide.md': 'В папке `SosninaAlbums-<версия>.exe`.\n',
  'scripts/win/installer.sh': '#!/bin/sh\n',
  'scripts/win/installer-check.ps1': '# x\n',
  'scripts/win/installer-check-lib.ps1': '# x\n',
  LICENSE,
  'NOTICE.md': NOTICE,
  'docs/specs/1.2/051-one.md': '# 051\n',
  'docs/specs/1.2/052-two.md': '# 052\n',
  'docs/specs/README.md': '| 051 | [один](1.2/051-one.md) | done |\n| 052 | [два](1.2/052-two.md) | done |\n',
  'internal/layout/a_test.go': 'package layout\n\nfunc TestGoldenFirst(t *testing.T) {}\nfunc TestGoldenFirst16(t *testing.T) {}\n',
  'internal/layout/testdata/golden/first.sha': 'abc\n',
}

// Макет с точечными правками: mod[путь] = новое содержимое (null — удалить файл).
function repo(mod = {}) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'relcheck-'))
  const all = { ...FILES, ...mod }
  for (const [rel, text] of Object.entries(all)) {
    if (text === null) continue
    const f = path.join(root, rel)
    fs.mkdirSync(path.dirname(f), { recursive: true })
    fs.writeFileSync(f, text)
  }
  return root
}
const run = (mod, version = '') => {
  const root = repo(mod)
  try {
    return checkAll(root, version)
  } finally {
    fs.rmSync(root, { recursive: true, force: true })
  }
}

test('макет целиком — без ошибок, версия берётся из wails.json', () => {
  assert.deepEqual(run({}), { version: '1.2.0', errors: [] })
  assert.deepEqual(run({}, '1.2.0').errors, [])
})

test('версия: каждое из четырёх мест отдельно', () => {
  const bad = (mod, part) => {
    const e = run(mod, '1.2.0').errors
    assert.equal(e.length, 1, e.join('\n'))
    assert.match(e[0], part)
  }
  bad({ 'wails.json': JSON.stringify({ info: { productVersion: '1.1.0' } }) }, /wails\.json/)
  bad({ 'README.md': '- Status: version 1.1.0, x\n' + MEDIA + FEATURES }, /README/)
  bad({ 'frontend/src/shell/dialogs/whatsnew.ts': "export const WHATS_NEW_VERSION = '1.1.0'\nexport const WHATS_NEW: readonly string[] = ['а']\n" }, /whatsnew/)
  bad({ 'CHANGELOG.md': '## 1.1.0\n\n- а\n' }, /нет раздела/)
})

test('версия: нет файлов и пустые значения', () => {
  assert.ok(run({ 'wails.json': null }, '1.2.0').errors.some((e) => /wails\.json: нет файла/.test(e)))
  assert.ok(run({ 'README.md': null }, '1.2.0').errors.some((e) => /README/.test(e)))
  assert.ok(run({}, 'abc').errors[0].includes('X.Y.Z'))
  assert.ok(run({ 'wails.json': '{}' }).errors[0].includes('--version'))
  const e = run({ 'frontend/src/shell/dialogs/whatsnew.ts': "export const WHATS_NEW_VERSION = '1.2.0'\nexport const WHATS_NEW: readonly string[] = []\n" }, '1.2.0').errors
  assert.deepEqual(e, ['whatsnew.ts: список WHATS_NEW пуст'])
})

test('CHANGELOG: раздел без пунктов и чужой первый раздел', () => {
  assert.ok(run({ 'CHANGELOG.md': '## 1.2.0\n\nтекст\n\n## 1.1.0\n\n- а\n' }, '1.2.0').errors.some((e) => /без пунктов/.test(e)))
  assert.ok(run({ 'CHANGELOG.md': '## 1.3.0\n\n- б\n\n## 1.2.0\n\n- а\n' }, '1.2.0').errors.some((e) => /первым идёт раздел 1\.3\.0/.test(e)))
  assert.equal(changelogSection('## 1.2.0\n- a\n## 1.2.1\n- b\n', '1.2.0'), '- a')
  assert.equal(changelogSection('## 1.2.10\n- a\n', '1.2.1'), null) // 1.2.1 не равно 1.2.10
})

test('спеки: не в таблице README и не упомянутая рядом с упомянутой', () => {
  const e = run({ 'docs/specs/README.md': '| 051 | x |\n' }, '1.2.0').errors
  assert.deepEqual(e, ['спека 052 (docs/specs/1.2) не в таблице docs/specs/README.md'])
  // номера в CHANGELOG уже есть (051) — значит, должны быть все
  assert.deepEqual(run({}, '1.2.0').errors, [])
  const no = run({ 'docs/specs/1.2/053-three.md': '#\n', 'docs/specs/README.md': FILES['docs/specs/README.md'] + '| 053 | x |\n' }, '1.2.0').errors
  assert.deepEqual(no, ['спека 053 (docs/specs/1.2) не упомянута в CHANGELOG 1.2.0'])
  // CHANGELOG без номеров — сверка молчит, патч без своей папки спек тоже
  assert.deepEqual(run({ 'CHANGELOG.md': '## 1.2.0\n\n- а\n' }, '1.2.0').errors, [])
  assert.deepEqual(run({ 'wails.json': '{"info":{"productVersion":"1.2.1"}}', 'README.md': '- Status: version 1.2.1, x\n' + MEDIA + FEATURES, 'CHANGELOG.md': '## 1.2.1\n\n- а\n', 'frontend/src/shell/dialogs/whatsnew.ts': "export const WHATS_NEW_VERSION = '1.2.1'\nexport const WHATS_NEW: readonly string[] = ['а']\n" }).errors, [])
})

test('needs: все задания в цепочке публикации', () => {
  assert.deepEqual(checkNeeds(YML), [])
  const lost = YML.replace('needs: [prepare, pack, mac-check, win-check]', 'needs: [prepare, pack, win-check]')
  assert.deepEqual(checkNeeds(lost), ['release.yml: «release» не ждёт задание «mac-check» (needs, в том числе через цепочку)'])
  // задание, которое никто не ждёт (забыли добавить в needs новой сборки)
  const extra = YML.replace('  release:', '  linux:\n    needs: prepare\n  release:')
  assert.deepEqual(checkNeeds(extra), ['release.yml: «release» не ждёт задание «linux» (needs, в том числе через цепочку)'])
  assert.deepEqual(checkNeeds(YML.replace('needs: [prepare, pack, mac-check, win-check]', 'needs: [prepare, pack, mac-check, win-check, ghost]')), [
    'release.yml: needs ссылается на несуществующее задание «ghost»',
  ])
  assert.deepEqual(checkNeeds('jobs:\n  a:\n    runs-on: x\n'), ['release.yml: нет задания «release»'])
})

test('parseJobs: строка, список в скобках, блочный список, без needs', () => {
  assert.deepEqual(parseJobs(YML), {
    prepare: [],
    windows: ['prepare'],
    macos: ['prepare'],
    pack: ['prepare', 'windows', 'macos'],
    'mac-check': ['prepare', 'pack'],
    'win-check': ['prepare', 'pack'],
    release: ['prepare', 'pack', 'mac-check', 'win-check'],
  })
})

test('arm64: тест из release.yml должен существовать, эталон — быть непустым', () => {
  assert.ok(run({ 'internal/layout/a_test.go': 'package layout\n\nfunc TestGoldenFirst16(t *testing.T) {}\n' }, '1.2.0').errors.some((e) => /«TestGoldenFirst»/.test(e)))
  assert.ok(run({ 'internal/layout/testdata/golden/first.sha': '' }, '1.2.0').errors.some((e) => /пустой эталон/.test(e)))
  assert.ok(run({ 'internal/layout/testdata/golden/first.sha': null }, '1.2.0').errors.some((e) => /нет эталонов/.test(e)))
  assert.ok(run({ '.github/workflows/release.yml': 'jobs:\n  release:\n    needs: []\n' }, '1.2.0').errors.some((e) => /нет проверки эталонов/.test(e)))
})

test('release-check: лицензия GPL-3.0 и условие 7(b)', () => {
  // файлы самого репозитория проходят
  assert.deepEqual(checkLicense(REPO), [])
  assert.match(LICENSE, /GNU GENERAL PUBLIC LICENSE\s+Version 3, 29 June 2007/)
  // CRLF (Windows, autocrlf) — тот же текст
  assert.deepEqual(run({ LICENSE: LICENSE.replace(/\n/g, '\r\n') }, '1.2.0').errors, [])
  const lic = (mod) => run(mod, '1.2.0').errors
  // нет файлов
  assert.deepEqual(lic({ LICENSE: null }), ['LICENSE: нет файла'])
  assert.deepEqual(lic({ 'NOTICE.md': null }), ['NOTICE.md: нет файла'])
  // чужая лицензия, пустой файл, правка в одну букву, строка сверху и снизу — всё не GPL v3 «как есть»
  for (const bad of ['MIT License\n\nCopyright (c) 2026\n', '', LICENSE.replace('Version 3', 'Version 2'), LICENSE.replace('GNU', 'GNY'), 'Sosnina Albums\n' + LICENSE, LICENSE + '\nAbout\n']) {
    const e = lic({ LICENSE: bad })
    assert.equal(e.length, 1, JSON.stringify(bad.slice(0, 30)))
    assert.match(e[0], /LICENSE: не точный текст GNU GPL v3/)
  }
  // каждая строка условия 7(b) обязательна, а пустой NOTICE ругается на все
  for (const [what, needle] of NOTICE_MUST) {
    const e = lic({ 'NOTICE.md': NOTICE.split(needle).join('') })
    assert.deepEqual(e, [`NOTICE.md: нет условия 7(b): ${what}`])
  }
  assert.equal(lic({ 'NOTICE.md': '' }).length, NOTICE_MUST.length)
})

// Макет с workflow ci.yml: modes — режимы файлов в «индексе» (путь → режим), остальных в индексе нет.
const exec = (yml, modes = {}) => {
  const root = repo({ '.github/workflows/ci.yml': yml })
  try {
    return workflowExec(root, (rel) => modes[rel] ?? null)
  } finally {
    fs.rmSync(root, { recursive: true, force: true })
  }
}

test('workflowExec: прямой вызов 100644 — ошибка', () => {
  const yml = 'jobs:\n  a:\n    steps:\n      - run: scripts/x.sh --flag\n'
  assert.deepEqual(exec(yml, { 'scripts/x.sh': '100644' }), ['ci.yml:4: scripts/x.sh без +x (git update-index --chmod=+x или вызов через bash)'])
  // файла нет в индексе — тоже ошибка; ключ run без «- » и ./ в начале
  assert.deepEqual(exec('jobs:\n  a:\n    steps:\n      - name: n\n        run: ./scripts/y.sh\n'), [
    'ci.yml:5: scripts/y.sh нет в git-индексе (git update-index --chmod=+x или вызов через bash)',
  ])
})

test('workflowExec: вызов через bash, sh, python, node, pwsh — ок', () => {
  const cmds = ['bash scripts/x.sh', 'sh scripts/x.sh', '/bin/bash scripts/x.sh', 'python scripts/x.py', 'python3 -I scripts/x.py', 'node scripts/x.mjs', 'pwsh scripts/x.ps1', 'powershell -File scripts/x.ps1', 'chmod +x scripts/linux/*.sh', 'echo "scripts/x.sh"']
  for (const c of cmds) assert.deepEqual(exec(`jobs:\n  a:\n    steps:\n      - run: ${c}\n`, { 'scripts/x.sh': '100644' }), [], c)
  // пустой и закомментированный run, workflow без run — тишина
  assert.deepEqual(exec('jobs:\n  a:\n    steps:\n      - run:\n      - run: # scripts/x.sh\n'), [])
  assert.deepEqual(exec('name: пусто\n'), [])
})

test('workflowExec: прямой вызов 100755 — ок', () => {
  const yml = 'jobs:\n  a:\n    steps:\n      - run: scripts/x.sh\n      - run: |\n          a=1\n          (scripts/y.sh; true)\n'
  assert.deepEqual(exec(yml, { 'scripts/x.sh': '100755', 'scripts/y.sh': '100755' }), [])
})

test('workflowExec: после && в многострочном run — ошибка, несколько файлов и вызовов', () => {
  const yml = 'jobs:\n  a:\n    steps:\n      - run: |\n          set -e\n          make && scripts/a.sh\n          x | scripts/b.sh || scripts/c.sh\n          bash scripts/d.sh\n      - name: после\n        run: echo ok\n'
  const modes = { 'scripts/a.sh': '100644', 'scripts/b.sh': '100755', 'scripts/c.sh': '100644', 'scripts/d.sh': '100644' }
  assert.deepEqual(exec(yml, modes), [
    'ci.yml:6: scripts/a.sh без +x (git update-index --chmod=+x или вызов через bash)',
    'ci.yml:7: scripts/c.sh без +x (git update-index --chmod=+x или вызов через bash)',
  ])
  // блок кончается на ключе того же отступа: строка вне блока не считается командой
  const out = exec('jobs:\n  a:\n    steps:\n      - run: |\n          echo\n        env:\n          X: scripts/z.sh\n')
  assert.deepEqual(out, [])
})

test('workflowExec: второй workflow и git-репозиторий настоящий', () => {
  const root = repo({ '.github/workflows/ci.yml': 'jobs: {}\n', '.github/workflows/z.yml': 'jobs:\n  a:\n    steps:\n      - run: scripts/x.sh\n' })
  try {
    // макет не репозиторий git: ошибка чтения режима — в списке, а не исключение
    const e = workflowExec(root)
    assert.equal(e.length, 1)
    assert.match(e[0], /^z\.yml:4: scripts\/x\.sh — не прочитан режим/)
  } finally {
    fs.rmSync(root, { recursive: true, force: true })
  }
  // текущее дерево репозитория чисто
  assert.deepEqual(workflowExec(REPO), [])
})

test('ролики README: заглушка, число адресов, файлы, размеры, старый снимок', () => {
  const m = (mod) => run(mod, '1.2.0').errors
  const head = '- Status: version 1.2.0, x\n'
  assert.deepEqual(m({}), [])
  const stub = m({ 'README.md': head + URL('TODO-README-MEDIA-A') + URL('TODO-README-MEDIA-B') })
  assert.ok(stub.some((e) => /TODO-README-MEDIA/.test(e)))
  assert.ok(m({ 'README.md': head + URL(ID_A) + URL(ID_A) }).some((e) => /адресов роликов user-attachments 1/.test(e)))
  assert.ok(m({ 'README.md': head + URL(ID_A) + URL(ID_B) + URL(ID_A) }).some((e) => /встречается 1 раз/.test(e)))
  assert.ok(m({ 'docs/media/demo.webp': null }).some((e) => /demo\.webp: нет файла/.test(e)))
  assert.ok(m({ 'docs/media/CREDITS.md': null }).some((e) => /CREDITS\.md: нет файла/.test(e)))
  assert.ok(m({ 'docs/media/demo.webp': 'x'.repeat(2621441) }).some((e) => /demo\.webp.*больше предела/.test(e)))
  assert.ok(m({ 'docs/media/screenshot.jpg': 'x'.repeat(409601) }).some((e) => /screenshot\.jpg.*больше предела/.test(e)))
  assert.deepEqual(m({ 'docs/media/demo.webp': 'x'.repeat(2621440) }), [])
  assert.ok(m({ 'README.md': head + MEDIA + '![x](docs/screenshot.png)\n' }).some((e) => /screenshot\.png/.test(e)))
  const root = repo({})
  try {
    assert.deepEqual(checkReadmeMedia(root), [])
  } finally {
    fs.rmSync(root, { recursive: true, force: true })
  }
})

test('установщик: win-check, вызовы скриптов и имя exe в заметках и инструкции', () => {
  const root = repo()
  try {
    assert.deepEqual(checkInstaller(root, YML), [])
    // нет задания win-check (release.yml ещё без установщика)
    const no = YML.replace(/  win-check:[\s\S]*?(?=  release:)/, '').replace(', win-check]', ']')
    assert.deepEqual(checkInstaller(root, no).filter((e) => /win-check/.test(e)), ['release.yml: нет задания «win-check» (проверка установщика на windows-latest)'])
    // win-check не ждёт pack
    const nopack = YML.replace('  win-check:\n    needs: [prepare, pack]', '  win-check:\n    needs: [prepare]')
    assert.deepEqual(checkInstaller(root, nopack), ['release.yml: «win-check» не ждёт «pack» (установщик собирается там)'])
    // скрипты не вызываются
    assert.deepEqual(checkInstaller(root, YML.replace('installer-check.ps1', 'other.ps1')), ['release.yml: не вызывается scripts/win/installer-check.ps1'])
    // помощники проверки — отдельный файл: без него installer-check.ps1 не запустится
    fs.rmSync(path.join(root, 'scripts/win/installer-check-lib.ps1'))
    assert.deepEqual(checkInstaller(root, YML), ['scripts/win/installer-check-lib.ps1: нет файла'])
  } finally {
    fs.rmSync(root, { recursive: true, force: true })
  }
  const bad = (mod) => run(mod, '1.2.0').errors
  assert.deepEqual(bad({ '.github/release-notes.md': 'Скачайте `SosninaAlbums-{{VERSION}}-windows-x64-setup.exe`. В zip `SosninaAlbums.exe`.\n' }), [
    '.github/release-notes.md: нет имени exe в zip «SosninaAlbums-{{VERSION}}.exe»',
  ])
  assert.deepEqual(bad({ '.github/release-notes.md': 'В zip `SosninaAlbums-{{VERSION}}.exe`.\n' }), [
    '.github/release-notes.md: нет установщика «SosninaAlbums-{{VERSION}}-windows-x64-setup.exe»',
  ])
  assert.deepEqual(bad({ 'docs/install/guide.md': 'В папке `SosninaAlbums.exe`.\n' }), ['docs/install/guide.md: нет имени exe в zip «SosninaAlbums-<версия>.exe»'])
  assert.deepEqual(bad({ 'docs/install/guide.md': null }), ['docs/install/guide.md: нет файла'])
  assert.deepEqual(bad({ 'scripts/win/installer.sh': null }), ['scripts/win/installer.sh: нет файла'])
})

test('«Возможности» в README: пункт текущей версии, диапазон, нет раздела', () => {
  const head = '- Status: version 1.2.0, x\n' + MEDIA
  const m = (tail) => run({ 'README.md': head + tail }, '1.2.0').errors
  assert.deepEqual(m(FEATURES), [])
  assert.deepEqual(m('## Возможности\n\n- Серия (1.1–1.3): раз.\n'), [])
  assert.deepEqual(m('## Возможности (1.2.0)\n\n- Вот (1.2): раз.\n'), [])
  assert.ok(m('## Возможности\n\n- Старое (1.1): раз.\n\n## Сборка\n- Новое (1.2)\n').some((e) => /нет пункта с «\(1\.2\)»/.test(e)))
  assert.ok(m('## Сборка\n').some((e) => /нет раздела/.test(e)))
  assert.deepEqual(checkReadmeFeatures('/nonexistent-root', '1.2.0'), ['README.md: нет файла'])
})
