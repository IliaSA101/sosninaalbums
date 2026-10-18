// Тесты выпуска набора библиотеки (спека 067 D1): zip, sha256, подпись каталога.
import assert from 'node:assert/strict'
import crypto from 'node:crypto'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { test } from 'node:test'
import { buildPack, catalogRow, makeZip, publicRaw, readZip, signBytes, upsertCatalog, verifyBytes } from './library-pack.mjs'

function setDir() {
  const d = fs.mkdtempSync(path.join(os.tmpdir(), 'libpack-'))
  fs.mkdirSync(path.join(d, 'bg'))
  fs.mkdirSync(path.join(d, 'spreads'))
  fs.writeFileSync(path.join(d, 'bg', 'one.png'), crypto.randomBytes(500))
  fs.writeFileSync(path.join(d, 'spreads', 'hero.json'), JSON.stringify({ v: 1, pages: 2 }))
  fs.writeFileSync(
    path.join(d, 'manifest.src.json'),
    JSON.stringify({
      id: 'test-set',
      version: 2,
      minApp: '0.16.0',
      title: 'Тест',
      license: 'CC0-1.0',
      items: [
        { file: 'bg/one.png', kind: 'bg', theme: 'wedding', name: 'Один' },
        { file: 'spreads/hero.json', kind: 'spread', name: 'Главное' },
      ],
    }),
  )
  return d
}

test('zip читается, sha256 и размеры совпадают с манифестом', () => {
  const { manifest, files } = buildPack(setDir())
  const zip = readZip(makeZip(files))
  const m = JSON.parse(zip.get('manifest.json').toString())
  assert.deepEqual(m, manifest)
  for (const it of m.items) {
    const data = zip.get(it.file)
    assert.equal(data.length, it.size)
    assert.equal(crypto.createHash('sha256').update(data).digest('hex'), it.sha256)
  }
  assert.equal(m.items[1].theme, 'basic')
})

test('плохие имена и повторы отвергаются', () => {
  const d = setDir()
  const p = path.join(d, 'manifest.src.json')
  const src = JSON.parse(fs.readFileSync(p, 'utf8'))
  src.items[1].file = '../x.json'
  fs.writeFileSync(p, JSON.stringify(src))
  assert.throws(() => buildPack(d))
  src.items[1].file = 'BG/One.png'
  fs.writeFileSync(p, JSON.stringify(src))
  assert.throws(() => buildPack(d))
})

test('каталог подписан, порча байта ломает подпись', () => {
  const { manifest, files } = buildPack(setDir())
  const zip = makeZip(files)
  const cat = upsertCatalog(null, catalogRow(manifest, zip, 'test-set-2.zip'))
  assert.equal(cat.packs[0].counts.bg, 1)
  assert.equal(cat.packs[0].size, zip.length)
  assert.ok(cat.packs[0].url.startsWith('https://github.com/IliaSA101/sosninaalbums-library/releases/download/'))
  const data = Buffer.from(JSON.stringify(cat, null, 2) + '\n')
  const { privateKey } = crypto.generateKeyPairSync('ed25519')
  const pub = publicRaw(privateKey)
  assert.equal(Buffer.from(pub, 'base64').length, 32)
  const sig = signBytes(data, privateKey)
  assert.equal(verifyBytes(data, sig, pub), true)
  const bad = Buffer.from(data)
  bad[10] ^= 1
  assert.equal(verifyBytes(bad, sig, pub), false)
  const other = publicRaw(crypto.generateKeyPairSync('ed25519').privateKey)
  assert.equal(verifyBytes(data, sig, other), false)
})

test('каталог: замена по id, старее — отказ', () => {
  const { manifest, files } = buildPack(setDir())
  const row = catalogRow(manifest, makeZip(files), 'a.zip')
  const c1 = upsertCatalog(null, row)
  assert.equal(upsertCatalog(c1, { ...row, version: 3 }).packs.length, 1)
  assert.throws(() => upsertCatalog(c1, { ...row, version: 1 }))
})
