// С2 (design §4.3), часть без чужих пользователей: путь по умолчанию (%APPDATA%\SosninaAlbums-stand-t0\mcp\sock),
// (д) Close удаляет файл, (в) после жёсткого убийства следующий запуск заменяет файл.
// Пункты (а) — кириллический пользователь и (б) — чужой пользователь — в c2-win.ps1. Usage: node c2.mjs <spikec>
// SPIKE_SOCK_C2 — путь сокета вместо умолчания (локальная проверка на Linux, чтобы не писать в ~/.config).
import fs from 'node:fs'
import path from 'node:path'
import { OUT, assert, check, hardKill, ingest, isWin, record, runGo, sleep, startServer, step, finish, stopServer } from './lib.mjs'

const exe = path.resolve(process.argv[2] || '')
const sock = process.env.SPIKE_SOCK_C2 || ''
const sockArgs = sock ? ['--sock', sock] : []
// Внимание: SPIKE_SOCK из окружения шага перебил бы путь по умолчанию — для spikec он снимается
const env = { SPIKE_SOCK: '' }
// Есть ли файл в папке: lstat/existsSync на файле сокета Windows ненадёжны, список папки — нет
const exists = (p) => {
  try {
    return fs.readdirSync(path.dirname(p)).includes(path.basename(p))
  } catch {
    return false
  }
}

await step('c2', async () => {
  assert(fs.existsSync(exe), `нет exe: ${exe}`)

  // (д) и путь по умолчанию: selftest от имени текущего пользователя (c2d/*)
  await check('c2d/selftest', async () => {
    const r = runGo(exe, ['--selftest', '--out', OUT, ...sockArgs], { env })
    ingest('c2d', r.out)
    assert(r.code === 0, `selftest код ${r.code}: ${r.err.slice(0, 300)}`)
    assert(/RESULT\tclose-removes-file\tOK/.test(r.out), 'нет OK close-removes-file')
  })

  // (в) убитый запуск оставляет файл; следующий запуск его заменяет
  let first
  await check('c2c/first-start', async () => {
    first = await startServer(exe, { tag: 'c2c1', sock, extra: [] })
    return `pid ${first.pid}, replaced=${first.replaced}`
  })
  if (first) {
    hardKill(first.pid)
    await sleep(700)
    const sp = sock || fs.readFileSync(first.ready, 'utf8').split(' ')[1]
    await check('c2c/stale-file-remains', async () => {
      assert(exists(sp), `после жёсткого убийства файла сокета нет: ${sp}`)
      return `файл остался: ${sp}`
    }, { strict: false })
    await check('c2c/bind-without-clean-fails', async () => {
      let failed = false
      try {
        const s = await startServer(exe, { tag: 'c2c-noclean', sock, noClean: true })
        stopServer(s)
      } catch {
        failed = true
      }
      assert(failed, 'Listen поверх старого файла прошёл без замены (Windows не держит имя занятым)')
      return 'bind поверх старого файла отказывает: без замены нельзя'
    }, { strict: false })
    let second
    await check('c2c/replace-stale', async () => {
      second = await startServer(exe, { tag: 'c2c2', sock })
      assert(second.replaced, `запуск прошёл, но замены не было (replaced=${second.replaced})`)
      return `новый pid ${second.pid}, старый файл заменён`
    })
    if (second) {
      await check('c2c/connect-after-replace', async () => {
        const r = runGo(exe, ['--clientcheck', '--out', OUT, ...sockArgs], { env })
        ingest('c2c/clientcheck', r.out)
        assert(r.code === 0, `подключение после замены: код ${r.code}`)
      })
      if (!isWin) {
        stopServer(second) // SIGTERM: слушатель закрывается сам и удаляет файл
        await sleep(800)
        await check('c2d/sigterm-removes-file', async () => {
          assert(!exists(sp), 'после SIGTERM файл сокета остался')
        }, { strict: false })
      } else {
        hardKill(second.pid)
      }
    }
  }
})
finish()
