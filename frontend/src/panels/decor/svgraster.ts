// SVG-стикеры (048, design §5.4): Go проверил файл и отдал его задачей {токен, base64, W×H}; окно рисует его
// в PNG и возвращает Go по токену. SVG только в <img> (режим «картинка»: скрипты и сеть закрыты), в DOM не кладётся.
// Задания идут по очереди; сбой одного (не открылся, холст «грязный», таймаут, отказ Go) — пропуск с причиной,
// остальные продолжаются. Итог собирается в тот же вид, что у импорта фонов, и идёт в тот же тост.
import { errText } from '../../base/words/errtext'
import type { app } from '../../../wailsjs/go/models'
import type { DecorImport, DecorItem, DecorSkip } from './decor'

// Что умеет браузер; в тестах подменяется (в node нет ни Image, ни canvas).
export interface RasterEnv {
  load(src: string): Promise<CanvasImageSource> // не читается → отказ
  png(img: CanvasImageSource, w: number, h: number): Promise<string> // base64 без префикса
}

export const MAX_SIDE = 4096 // длинная сторона PNG: больше Go не принимает
export const RASTER_TIMEOUT = 20_000 // мс на одно задание: зависшая картинка не держит окно

const WHY = {
  size: 'неверный размер рисунка',
  load: 'не удалось прочитать рисунок',
  time: 'рисунок не открылся за отведённое время',
  taint: 'программа не смогла его нарисовать — сохраните как PNG',
  draw: 'не удалось нарисовать',
} as const

class RasterError extends Error {
  constructor(readonly why: string) {
    super(why)
  }
}

export const svgSrc = (data: string): string => `data:image/svg+xml;base64,${data}`

export const domEnv: RasterEnv = {
  async load(src) {
    const img = new Image()
    img.src = src
    await img.decode()
    return img
  },
  async png(img, w, h) {
    const cv = document.createElement('canvas')
    cv.width = w
    cv.height = h
    const ctx = cv.getContext('2d')
    if (!ctx) throw new Error('нет холста')
    ctx.drawImage(img, 0, 0, w, h)
    const url = cv.toDataURL('image/png') // «грязный» холст бросает SecurityError
    const i = url.indexOf(',')
    return i < 0 ? '' : url.slice(i + 1)
  },
}

// Промис против таймера: таймер всегда снимается.
function within<T>(p: Promise<T>, ms: number): Promise<T> {
  let t: ReturnType<typeof setTimeout> | undefined
  const timer = new Promise<never>((_, rej) => {
    t = setTimeout(() => rej(new RasterError(WHY.time)), ms)
  })
  return Promise.race([p, timer]).finally(() => clearTimeout(t))
}

const sideOk = (n: number): boolean => Number.isInteger(n) && n >= 1 && n <= MAX_SIDE

// Одно задание → PNG (base64). Причина отказа — в RasterError.
async function render(job: app.DecorSvg, env: RasterEnv, timeoutMs: number): Promise<string> {
  if (!sideOk(job.w) || !sideOk(job.h)) throw new RasterError(WHY.size)
  return within(
    (async () => {
      let img: CanvasImageSource
      try {
        img = await env.load(svgSrc(job.data))
      } catch {
        throw new RasterError(WHY.load)
      }
      try {
        const png = await env.png(img, job.w, job.h)
        if (!png) throw new Error('пусто')
        return png
      } catch (e) {
        throw new RasterError((e as { name?: string })?.name === 'SecurityError' ? WHY.taint : WHY.draw)
      }
    })(),
    timeoutMs,
  )
}

// Все задания по очереди. send — `Api.AddDecorPNG(token, base64)`. Итог — добавленные и пропущенные.
export async function rasterSvgs(
  jobs: readonly app.DecorSvg[] | null | undefined,
  send: (token: string, png: string) => Promise<DecorItem>,
  opts: { env?: RasterEnv; timeoutMs?: number } = {},
): Promise<DecorImport> {
  const added: DecorItem[] = []
  const skipped: DecorSkip[] = []
  const env = opts.env ?? domEnv
  for (const job of jobs ?? []) {
    try {
      added.push(await send(job.token, await render(job, env, opts.timeoutMs ?? RASTER_TIMEOUT)))
    } catch (e) {
      // caps-ok: причина встаёт после имени файла, в середине фразы
      skipped.push({ name: job.name, why: e instanceof RasterError ? e.why : errText(e) || WHY.draw })
    }
  }
  return { added, skipped }
}

// Итог Go (добавлено/пропущено сразу) + итог по SVG: один тост на всю загрузку.
export const mergeImport = (a: DecorImport, b: DecorImport): DecorImport => ({
  added: [...a.added, ...b.added],
  skipped: [...a.skipped, ...b.skipped],
})
