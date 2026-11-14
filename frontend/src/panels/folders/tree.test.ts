import { describe, expect, it } from 'vitest'
import { GROUP_KEY, batchKey, fileKey, looseGroup } from './loose'
import {
  addedText,
  ancestors,
  buildTree,
  buttonText,
  cachedFilterTree,
  cachedTree,
  checkState,
  countParts,
  removeLabel,
  removeTitle,
  rowTitle,
  filterTree,
  findNode,
  inAlbum,
  nodeKey,
  onlyThis,
  photoRows,
  pruneHidden,
  removeText,
  rows,
  toggleSubtree,
  treeStats,
  visibleCounts,
  type TreeNode,
} from './tree'

const ph = (...paths: string[]) => paths.map((path, i) => ({ path, id: 'p' + i }))
const at = (forest: TreeNode[], dir: string): TreeNode => {
  const n = findNode(forest, nodeKey(dir))
  if (n?.key !== nodeKey(dir)) throw new Error('нет узла ' + dir)
  return n
}
const paths = (l: { path: string }[]) => l.map((p) => p.path)
const counts = (n: TreeNode): unknown => ({ name: n.name, own: n.own, total: n.total, kids: n.kids.map(counts) })

// Весна 2026: 2 своих, Март 2026 › Избранное Март, Апрель 2026.
const SPRING = ph(
  'E:\\Съёмки\\Весна 2026\\1.jpg',
  'E:\\Съёмки\\Весна 2026\\2.jpg',
  'E:\\Съёмки\\Весна 2026\\Март 2026\\3.jpg',
  'E:\\Съёмки\\Весна 2026\\Март 2026\\Избранное Март\\4.jpg',
  'E:\\Съёмки\\Весна 2026\\Март 2026\\Избранное Март\\5.jpg',
  'E:\\Съёмки\\Весна 2026\\Апрель 2026\\6.jpg',
)
const ROOT = 'E:\\Съёмки\\Весна 2026'

describe('buildTree', () => {
  it('counts: свои и с подпапками, глубочайшая папка фото', () => {
    const [r] = buildTree([ROOT], SPRING)
    expect(counts(r)).toEqual({
      name: 'Весна 2026',
      own: 2,
      total: 6,
      kids: [
        { name: 'Апрель 2026', own: 1, total: 1, kids: [] },
        { name: 'Март 2026', own: 1, total: 3, kids: [{ name: 'Избранное Март', own: 2, total: 2, kids: [] }] },
      ],
    })
    expect(r.root).toBe(true)
    expect(r.kids[0].root).toBe(false)
    expect(at([r], ROOT + '\\Март 2026\\Избранное Март').path).toBe(ROOT + '\\Март 2026\\Избранное Март')
  })

  it('empty dirs hidden: подпапки без фото нет в дереве', () => {
    const [r] = buildTree(['D:/a'], ph('D:/a/x/1.jpg'))
    expect(r.kids.map((k) => k.name)).toEqual(['x'])
    expect(findNode([r], nodeKey('D:/a/empty'))?.key).toBe(nodeKey('D:/a'))
  })

  it('intermediate: папка без своих фото, но с фото глубже — «0 (N)»', () => {
    const [r] = buildTree(['D:/a'], ph('D:/a/b/c/1.jpg', 'D:/a/b/c/2.jpg'))
    expect(r.own).toBe(0)
    expect(r.total).toBe(2)
    const b = at([r], 'D:/a/b')
    expect([b.own, b.total, b.kids[0].own]).toEqual([0, 2, 2])
  })

  it('empty root: добавленная папка видна всегда, даже без фото и на отключённом диске', () => {
    const f = buildTree(['D:/a', 'Z:/gone'], ph('D:/a/1.jpg'))
    expect(f.map((n) => [n.name, n.total])).toEqual([
      ['a', 1],
      ['gone', 0],
    ])
    expect(buildTree(['D:/a'], [])[0].total).toBe(0)
    expect(buildTree([], ph('D:/a/1.jpg'))).toEqual([])
  })

  it('drive root: «E:», фото прямо в корне диска', () => {
    const [r] = buildTree(['E:\\'], ph('E:\\1.jpg', 'E:\\x\\2.jpg'))
    expect([r.name, r.own, r.total]).toEqual(['E:', 1, 2])
  })

  it('same names: у одинаковых имён — бледная родительская папка', () => {
    const f = buildTree(['D:\\Ивановы\\Отбор', 'E:\\Петровы\\Отбор', 'E:\\Свадьба'], [])
    expect(f.map((n) => n.hint)).toEqual(['Ивановы', 'Петровы', undefined])
  })

  it('регистр и разделители не важны; D:/a не захватывает D:/ab', () => {
    expect(buildTree(['d:/Свадьба/'], ph('D:\\свадьба\\x\\1.jpg'))[0].total).toBe(1)
    expect(buildTree(['D:/a', 'D:/ab'], ph('D:/ab/1.jpg', 'D:/a/2.jpg')).map((n) => n.total)).toEqual([1, 1])
  })

  it('повтор папки — одна строка; вложенные записи (старый файл до свёртки Go) — фото у ближайшей', () => {
    expect(buildTree(['D:/a', 'd:\\A'], ph('D:/a/1.jpg')).map((n) => n.total)).toEqual([1])
    const f = buildTree(['D:/a', 'D:/a/sub'], ph('D:/a/sub/1.jpg', 'D:/a/2.jpg'))
    expect(f.map((n) => [n.name, n.total, n.kids.length])).toEqual([
      ['a', 1, 0],
      ['sub', 1, 0],
    ])
  })

  it('естественный порядок подпапок: «Часть 2» раньше «Часть 10»', () => {
    const [r] = buildTree(['D:/a'], ph('D:/a/Часть 10/1.jpg', 'D:/a/Часть 2/1.jpg'))
    expect(r.kids.map((k) => k.name)).toEqual(['Часть 2', 'Часть 10'])
  })

  it('cachedTree: тот же состав папок и тот же список фото — то же дерево', () => {
    const t = cachedTree()
    const first = t([ROOT], SPRING)
    expect(t([ROOT.toLowerCase()], SPRING)).toBe(first)
    expect(t([ROOT], [...SPRING])).not.toBe(first)
  })

  // Go держит фото за «.»-подпапкой отдельным (View.loose): в дереве папок его нет, счёт корня не двоится.
  it('отдельное фото под корнем за «.»-подпапкой не строит узел и не считается в корне', () => {
    const photos = ph('D:/a/1.jpg', 'D:/a/.edits/x.jpg') // id: p0, p1
    const loose = [{ id: 'b1', at: 0, files: [{ path: 'D:/a/.edits/x.jpg', photo: 'p1' }] }]
    const [r] = buildTree(['D:/a'], photos, loose)
    expect([r.own, r.total, r.kids.length]).toEqual([1, 1, 0])
    expect(buildTree(['D:/a'], photos)[0].total).toBe(2) // без loose — прежний счёт
    const t = cachedTree()
    const first = t(['D:/a'], photos, loose)
    expect(t(['D:/a'], photos, [{ ...loose[0] }])).toBe(first) // новый массив того же состава — то же дерево
    expect(t(['D:/a'], photos, [])).not.toBe(first)
    // флажок корня отдельное не прячет
    expect(paths(filterTree(photos, new Set([nodeKey('D:/a')])))).toEqual(['D:/a/.edits/x.jpg'])
  })
})

describe('rows', () => {
  it('rows open roots: корни раскрыты, глубже закрыто; глубина и порядок', () => {
    const f = buildTree([ROOT, 'D:/b'], [...SPRING, ...ph('D:/b/1.jpg')])
    const byDefault = (n: TreeNode) => n.root
    expect(rows(f, byDefault).map((r) => [r.node.name, r.depth])).toEqual([
      ['Весна 2026', 0],
      ['Апрель 2026', 1],
      ['Март 2026', 1],
      ['b', 0],
    ])
    const open = new Set([nodeKey(ROOT), nodeKey(ROOT + '\\Март 2026')])
    expect(rows(f, (n) => open.has(n.key)).map((r) => r.depth)).toEqual([0, 1, 1, 2, 0])
  })
})

describe('флажки', () => {
  const f = buildTree([ROOT, 'D:/b'], [...SPRING, ...ph('D:/b/1.jpg')])
  const spring = f[0]
  const mart = at(f, ROOT + '\\Март 2026')
  const fav = at(f, ROOT + '\\Март 2026\\Избранное Март')

  it('check states: on / off / mixed', () => {
    const hidden = new Set([fav.key])
    const vis = visibleCounts(f, hidden)
    expect(checkState(spring, vis.get(spring)!)).toBe('mixed')
    expect(checkState(mart, vis.get(mart)!)).toBe('mixed')
    expect(checkState(fav, vis.get(fav)!)).toBe('off')
    expect(checkState(f[1], vis.get(f[1])!)).toBe('on')
  })

  it('toggle subtree: «—» и пустой — показать всё, галочка — скрыть всё', () => {
    const hidden = new Set([fav.key])
    toggleSubtree(mart, hidden) // mixed → всё показано
    expect(hidden.size).toBe(0)
    toggleSubtree(mart, hidden) // on → всё скрыто (ключи только папок со своими фото)
    expect([...hidden].sort()).toEqual([mart.key, fav.key].sort())
    toggleSubtree(mart, hidden) // off → показать
    expect(hidden.size).toBe(0)
  })

  it('only: видно ровно поддерево', () => {
    const hidden = new Set<string>()
    onlyThis(f, fav, hidden)
    expect(paths(filterTree(SPRING, hidden))).toEqual([SPRING[3].path, SPRING[4].path])
    onlyThis(f, mart, hidden)
    expect(paths(filterTree(SPRING, hidden))).toEqual([SPRING[2].path, SPRING[3].path, SPRING[4].path])
  })

  it('photoRows: одна строка с фото — флажков нет', () => {
    expect(photoRows(buildTree(['D:/a'], ph('D:/a/1.jpg')))).toBe(1)
    expect(photoRows(buildTree(['D:/a'], ph('D:/a/b/c/1.jpg')))).toBe(1)
    expect(photoRows(f)).toBe(5) // Весна, Март, Избранное, Апрель, b
  })
})

describe('filterTree', () => {
  const f = buildTree([ROOT], SPRING)
  const fav = at(f, ROOT + '\\Март 2026\\Избранное Март')

  it('filterTree same array: ничего не скрыто — тот же массив', () => {
    expect(filterTree(SPRING, new Set())).toBe(SPRING)
  })

  it('hides subfolder only: фото подпапки нет, фото родителя есть', () => {
    const out = filterTree(SPRING, new Set([fav.key]))
    expect(paths(out)).toEqual(paths(SPRING).filter((p) => !p.includes('Избранное')))
  })

  it('скрыта папка целиком — с подпапками; фото вне дерева остаётся; регистр не важен', () => {
    const hidden = new Set<string>()
    toggleSubtree(f[0], hidden)
    expect(filterTree([...SPRING, ...ph('Z:/x/1.jpg')], hidden).map((p) => p.path)).toEqual(['Z:/x/1.jpg'])
    expect(filterTree(ph('d:/A/1.jpg'), new Set([nodeKey('D:\\a\\')]))).toEqual([])
  })

  it('cached: те же фото и скрытые — тот же ответ без пересчёта; иначе новый', () => {
    const c = cachedFilterTree()
    const first = c(SPRING, new Set([fav.key]))
    const calls = treeStats.keyCalc
    expect(c(SPRING, new Set([fav.key]))).toBe(first)
    expect(c([...SPRING], new Set([fav.key]))).not.toBe(first)
    expect(c(SPRING, new Set([f[0].key]))).not.toBe(first)
    expect(c(SPRING, new Set())).toBe(SPRING)
    expect(treeStats.keyCalc).toBe(calls) // ключи папок уже на объектах фото
  })

  it('key memo и скорость: 20 тыс. фото в 200 папках — ключ считается один раз на фото', () => {
    const many = Array.from({ length: 20000 }, (_, i) => ({ id: 'q' + i, path: `D:/big/д${i % 200}/с${i % 7}/${i}.jpg` }))
    const before = treeStats.keyCalc
    const tree = buildTree(['D:/big'], many)
    expect(treeStats.keyCalc - before).toBe(20000)
    expect(tree[0].total).toBe(20000)
    expect(tree[0].kids).toHaveLength(200)
    const hidden = new Set<string>()
    toggleSubtree(tree[0].kids[0], hidden)
    expect(filterTree(many, hidden)).toHaveLength(20000 - 100)
    buildTree(['D:/big'], many)
    expect(inAlbum(tree[0].kids[0], many, new Set(['q0', 'q1']))).toBe(1)
    expect(treeStats.keyCalc - before).toBe(20000) // фильтр, второе дерево и счёт для вопроса — без пересчёта
  })
})

describe('pruneHidden', () => {
  it('prune by roots: ключи вне корней забыты, без фото под корнем — нет (идёт чтение)', () => {
    const hidden = new Set([nodeKey('D:/a/x'), nodeKey('D:/gone/y'), nodeKey('D:/ab'), 'f:d:/loose/1.jpg', 'f:d:/old.jpg'])
    pruneHidden(hidden, ['D:/a'], new Set(['f:d:/loose/1.jpg']))
    expect([...hidden].sort()).toEqual([nodeKey('D:/a/x'), 'f:d:/loose/1.jpg'].sort())
  })

  it('prune при одном корне: скрытая подпапка остаётся (054 сбрасывал при одной папке)', () => {
    const hidden = new Set([nodeKey('D:/a/fav')])
    pruneHidden(hidden, ['D:/a'])
    expect(hidden.size).toBe(1)
  })

  it('корень диска', () => {
    const hidden = new Set([nodeKey('E:/x')])
    pruneHidden(hidden, ['E:\\'])
    expect(hidden.size).toBe(1)
  })
})

describe('inAlbum (orphansIfRemoved)', () => {
  const f = buildTree([ROOT, 'D:/b'], [...SPRING, ...ph('D:/b/1.jpg')])
  const all = [...SPRING, { path: 'D:/b/1.jpg', id: 'b1' }]

  it('фото альбома под узлом с подпапками; чужие папки и фото вне ленты не считаются', () => {
    const used = new Set(['p0', 'p4', 'b1', 'lost'])
    expect(inAlbum(f[0], all, used)).toBe(2)
    expect(inAlbum(at(f, ROOT + '\\Март 2026'), all, used)).toBe(1)
    expect(inAlbum(f[1], all, used)).toBe(1)
    expect(inAlbum(f[0], all, new Set())).toBe(0)
  })
})

describe('тексты', () => {
  it('removeText: подпапки, число, единственное фото', () => {
    const [r] = buildTree([ROOT], SPRING)
    expect(removeText(r, 6)).toBe(
      'В альбоме стоят 6 фото из «Весна 2026» и её подпапок. На листах они останутся, а из ленты пропадут все 6 фото папки.',
    )
    expect(removeText(r, 1)).toContain('В альбоме стоит 1 фото')
    const [one] = buildTree(['D:/a'], ph('D:/a/1.jpg'))
    expect(removeText(one, 1)).toBe('В альбоме стоит 1 фото из «a». На листе оно останется, а из ленты пропадёт 1 фото папки.')
  })

  it('addedText: подпапка, та же, новая с поглощением, без тоста', () => {
    const fav = ROOT + '\\Март 2026\\Избранное Март'
    expect(addedText({ dir: fav, within: ROOT, merged: 0, looseIn: 0 })).toBe(
      '«Избранное Март» уже входит в «Весна 2026» — отмечена в списке',
    )
    expect(addedText({ dir: fav, within: ROOT, merged: 0, looseIn: 0 }, false)).toBe('«Избранное Март» уже входит в «Весна 2026»')
    expect(addedText({ dir: ROOT.toUpperCase(), within: ROOT, merged: 0, looseIn: 0 })).toBe(
      '«Весна 2026» уже добавлена — отмечена в списке',
    )
    expect(addedText({ dir: ROOT, within: '', merged: 2, looseIn: 0 })).toBe('Папка «Весна 2026» добавлена — в неё вошли прежние папки: 2')
    expect(addedText({ dir: ROOT, within: '', merged: 0, looseIn: 3 })).toBe(
      'Папка «Весна 2026» добавлена — отдельные фото из неё (3) теперь в папке',
    )
    expect(addedText({ dir: ROOT, within: '', merged: 0, looseIn: 0 })).toBe('')
  })
})

// Go оставляет подпапку отдельным корнем, если сканер в неё не заходит (fix-097g1): «.edits», имя устройства, ссылка.
describe('вложенный корень [A, A\\.edits]', () => {
  const EDITS = ROOT + '\\.edits'
  const photos = ph(ROOT + '\\1.jpg', ROOT + '\\Март 2026\\2.jpg', EDITS + '\\3.jpg', EDITS + '\\sub\\4.jpg')
  const forest = () => buildTree([ROOT, EDITS], photos)

  it('nested root rows: две строки-корня со своими числами «1 (2)», .edits не входит в поддерево A', () => {
    const f = forest()
    expect(f.map(counts)).toEqual([
      { name: 'Весна 2026', own: 1, total: 2, kids: [{ name: 'Март 2026', own: 1, total: 1, kids: [] }] },
      { name: '.edits', own: 1, total: 2, kids: [{ name: 'sub', own: 1, total: 1, kids: [] }] },
    ])
    expect(f.map((n) => n.root)).toEqual([true, true]) // ✕ у обоих
    expect(countParts(f[0])).toEqual({ own: '1', all: '(2)' })
    expect(countParts(f[1])).toEqual({ own: '1', all: '(2)' })
    // Порядок списка Go не важен: .edits первым — те же строки
    expect(buildTree([EDITS, ROOT], photos).map((n) => [n.name, n.total])).toEqual([
      ['.edits', 2],
      ['Весна 2026', 2],
    ])
  })

  it('nested root only: «только» у A прячет .edits и наоборот; флажок A не трогает .edits; prune их не забывает', () => {
    const f = forest()
    const hidden = new Set<string>()
    onlyThis(f, f[0], hidden)
    expect(paths(filterTree(photos, hidden))).toEqual(paths(photos.slice(0, 2)))
    onlyThis(f, f[1], hidden)
    expect(paths(filterTree(photos, hidden))).toEqual(paths(photos.slice(2)))
    hidden.clear()
    toggleSubtree(f[0], hidden)
    expect(paths(filterTree(photos, hidden))).toEqual(paths(photos.slice(2)))
    pruneHidden(hidden, [ROOT, EDITS])
    expect(hidden.size).toBe(2)
  })

  it('nested root orphans: ✕ у A считает только фото A (.edits останется в ленте); ✕ у .edits — только его', () => {
    const f = forest()
    const used = new Set(['p0', 'p2', 'p3'])
    expect(inAlbum(f[0], photos, used)).toBe(1)
    expect(inAlbum(f[1], photos, used)).toBe(2)
    expect(removeText(f[0], 1)).toBe(
      'В альбоме стоит 1 фото из «Весна 2026» и её подпапок. На листе оно останется, а из ленты пропадут все 2 фото папки.',
    )
    expect(findNode(f, nodeKey(EDITS))).toBe(f[1])
  })
})

describe('отдельные фото в дереве (097)', () => {
  const NOW = new Date(2026, 9, 21, 15, 0)
  const at = (h: number, m: number) => new Date(2026, 9, 21, h, m).getTime() / 1000
  const P1 = 'D:\\Архив\\Портреты\\IMG_2041.jpg'
  const P2 = 'C:\\Загрузки\\bouquet.jpg'
  const GONE = 'D:\\Телефон\\gone.jpg'
  const LOOSE = [
    {
      id: 'b1',
      at: at(14, 5),
      files: [
        { path: P1, photo: 'l1' },
        { path: P2, photo: 'l2' },
      ],
    },
    { id: 'b2', at: at(14, 30), files: [{ path: GONE, photo: 'l3' }] },
  ]
  const lphotos = [
    { path: P1, id: 'l1' },
    { path: P2, id: 'l2' },
  ]
  const here = (id: string) => lphotos.some((p) => p.id === id)
  const noWhy = () => undefined
  const group = (scanning = false, why: (id: string) => string | undefined = noWhy) => looseGroup(LOOSE, here, why, scanning, NOW)!

  it('loose group: ветка → добавления по порядку → файлы с бледной папкой; ✕ у добавления и файла, у ветки нет', () => {
    const g = group()
    expect([g.key, g.kind, g.name, g.root, g.total]).toEqual([GROUP_KEY, 'group', 'Фото отдельно', false, 2])
    expect(g.kids.map((b) => [b.key, b.kind, b.name, b.root, b.total])).toEqual([
      [batchKey('b1'), 'batch', '14:05', true, 2],
      [batchKey('b2'), 'batch', '14:30', true, 0],
    ])
    const [f1, f2] = g.kids[0].kids
    expect([f1.key, f1.kind, f1.name, f1.hint, f1.own, f1.loose]).toEqual([
      fileKey(P1),
      'file',
      'IMG_2041.jpg',
      'Портреты',
      1,
      { id: 'b1', file: P1, photo: 'l1' },
    ])
    expect(f2.hint).toBe('Загрузки')
    expect(g.kids[0].loose).toEqual({ id: 'b1', file: '' })
    expect(ancestors(f1)).toEqual([g, g.kids[0]])
    expect(countParts(g)).toEqual({ own: '2', all: '' })
    expect(countParts(f1)).toEqual({ own: '', all: '' })
    expect(rowTitle(g)).toBe('Фото, добавленные по одному')
    expect(rowTitle(g.kids[0])).toBe('Добавлено сегодня в 14:05')
    expect(rowTitle(f1)).toBe(P1)
    expect([removeTitle(f1), removeLabel(f1)]).toEqual(['Убрать из ленты', 'Убрать «IMG_2041.jpg» из ленты'])
    expect([removeTitle(g.kids[0]), removeLabel(g.kids[0])]).toEqual(['Убрать из ленты', 'Убрать «14:05» из ленты'])
    expect(looseGroup([], here, noWhy, false, NOW)).toBeNull()
  })

  it('missing file: «нет на диске» только после чтения; в числа не входит; флажок серый (total 0)', () => {
    const gone = group().kids[1].kids[0]
    expect([gone.missing, gone.own, gone.total]).toEqual([true, 0, 0])
    expect(rowTitle(gone)).toBe(GONE + '\nНет на диске')
    expect(group(true).kids[1].kids[0].missing).toBe(false) // идёт чтение — ещё не «нет»
  })

  it('unread tip: путь, затем «Не читается: причина из Go» без добавок', () => {
    const bad = group(false, (id) => (id === 'l3' ? 'внутри HEIC, а не JPEG. Совет из Go' : undefined)).kids[1].kids[0]
    expect(rowTitle(bad)).toBe(GONE + '\nНе читается: внутри HEIC, а не JPEG. Совет из Go')
    const big = group(false, () => 'слишком большое (больше 250 Мп)').kids[1].kids[0]
    expect(rowTitle(big)).toBe(GONE + '\nНе читается: слишком большое (больше 250 Мп)')
  })

  it('filter loose: флажок файла, «только» у добавления и у папки; фото папок не задеты', () => {
    const folders = buildTree([ROOT], SPRING)
    const g = group()
    const forest = [...folders, g]
    const all = [...SPRING, ...lphotos]
    const hidden = new Set<string>()
    toggleSubtree(g.kids[0].kids[1], hidden) // букет
    expect(paths(filterTree(all, hidden))).toEqual(paths([...SPRING, lphotos[0]]))
    expect(checkState(g.kids[0], visibleCounts(forest, hidden).get(g.kids[0])!)).toBe('mixed')
    onlyThis(forest, g.kids[0], hidden)
    expect(paths(filterTree(all, hidden))).toEqual(paths(lphotos))
    onlyThis(forest, folders[0], hidden)
    expect(paths(filterTree(all, hidden))).toEqual(paths(SPRING))
    expect(photoRows(forest)).toBe(6) // 4 папки со своими фото + 2 файла
  })

  it('кнопка без папок, но с отдельными — «Папки» без числа', () => {
    expect(buttonText(0, false)).toBe('Папки')
    expect(buttonText(0, true)).toBe('Папки · фильтр')
  })
})
