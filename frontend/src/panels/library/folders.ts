// Число фото по папкам проекта для окна «Папки» (спека 024). Go не нужен: у каждого фото есть путь.

// Ключ пути: без различия регистра, разделитель «/», без завершающего «/».
function key(p: string): string {
  return p.toLowerCase().replaceAll('\\', '/').replace(/\/+$/, '')
}

// Сколько фото у каждой папки. Сканер обходит папки по порядку списка и отбрасывает дубли, поэтому фото
// вложенной или повторной папки достаётся первой из подходящих. Результат параллелен folders.
export function folderCounts(folders: readonly string[], photos: readonly { path: string }[]): number[] {
  const idx = new Map<string, number>()
  folders.forEach((f, i) => {
    const k = key(f)
    if (!idx.has(k)) idx.set(k, i)
  })
  const out = folders.map(() => 0)
  for (const ph of photos) {
    let p = key(ph.path)
    let best = -1
    for (let i = p.lastIndexOf('/'); i >= 0; i = p.lastIndexOf('/')) {
      p = p.slice(0, i)
      const at = idx.get(p)
      if (at !== undefined && (best < 0 || at < best)) best = at
    }
    if (best >= 0) out[best]++
  }
  return out
}
