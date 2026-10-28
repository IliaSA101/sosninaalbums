// Сводка спайка 143/Т0: results.tsv → таблица в $GITHUB_STEP_SUMMARY и вердикты по критериям design §4.3.
// Код 1, если есть FAIL или у обязательного критерия нет ни одной записи (шаг упал раньше проверок).
import fs from 'node:fs'
import path from 'node:path'
import { OUT } from './lib.mjs'

const win = process.platform === 'win32'
const file = path.join(OUT, 'results.tsv')
const rows = fs.existsSync(file)
  ? fs.readFileSync(file, 'utf8').split('\n').filter(Boolean).map((l) => {
      const [id, status, ...d] = l.split('\t')
      return { id, status, detail: d.join(' ') }
    })
  : []

const criteria = [
  ['С1 stdio у exe с подсистемой GUI' + (win ? '' : ' (на этой ОС подсистемы нет — проверяется stdio)'), (r) => r.id.startsWith('c1/'), true],
  ['С2 (а) кириллический пользователь ≥ 20 букв', (r) => r.id.startsWith('c2a/'), win],
  ['С2 (б) другой пользователь без прав администратора — отказ', (r) => r.id.startsWith('c2b/'), win],
  ['С2 (в) после жёсткого убийства файл заменяется', (r) => r.id.startsWith('c2c/'), true],
  ['С2 (д) Close удаляет файл', (r) => r.id.startsWith('c2d/'), true],
  ['С4 клиент Go SDK, две ревизии', (r) => r.id.startsWith('c4go-'), true],
  ['С4 сырой JSON-RPC: заглушка, повтор, пределы', (r) => r.id.startsWith('c4raw/'), true],
  ['С4 MCP Inspector --cli', (r) => r.id.startsWith('c4insp/'), false],
]

let bad = 0
const lines = [`## Спайк 143/Т0 — ${process.platform}`, '', '| Критерий | Вердикт | OK | FAIL |', '|---|---|---|---|']
for (const [name, pick, required] of criteria) {
  const rs = rows.filter(pick)
  const ok = rs.filter((r) => r.status === 'OK').length
  const fail = rs.filter((r) => r.status === 'FAIL').length
  let v = 'ПРОШЁЛ'
  if (fail > 0) v = 'НЕ ПРОШЁЛ'
  else if (ok === 0) v = required ? 'НЕ ЗАПУСКАЛСЯ (шаг упал раньше проверок)' : 'пропущен'
  if (fail > 0 || (ok === 0 && required)) bad++
  lines.push(`| ${name} | ${v} | ${ok} | ${fail} |`)
}
lines.push('', 'С2 (г) Kaspersky — только на ноутбуке, в CI пропущен.', '')
const fails = rows.filter((r) => r.status === 'FAIL')
if (fails.length) {
  lines.push('### Провалы', '', ...fails.map((r) => `- \`${r.id}\` — ${r.detail}`), '')
}
lines.push('<details><summary>Все записи</summary>', '', '| id | статус | подробности |', '|---|---|---|', ...rows.map((r) => `| \`${r.id}\` | ${r.status} | ${r.detail.replace(/\|/g, '\\|')} |`), '', '</details>')
const text = lines.join('\n')
console.log(text)
if (process.env.GITHUB_STEP_SUMMARY) fs.appendFileSync(process.env.GITHUB_STEP_SUMMARY, text + '\n', 'utf8')
process.exit(bad > 0 || fails.length > 0 ? 1 : 0)
