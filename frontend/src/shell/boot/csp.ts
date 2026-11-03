// CSP выпуска: из dev-CSP (index.html) вырезаем то, что нужно только HMR dev-сервера Vite.
// ws:// — сокет HMR; worker-src blob: — SharedWorker клиента HMR (waitForSuccessfulPing при разрыве).
const devOnly = [' ws://localhost:* ws://127.0.0.1:*', "; worker-src 'self' blob:"]

const cspRe = /http-equiv="Content-Security-Policy"\s+content="([^"]*)"/

/**
 * releaseCsp убирает из html dev-части CSP; выпускная политика остаётся прежней байт в байт.
 * Если после замены в политике остались ws:// или worker-src (CSP в index.html переставили) —
 * бросает ошибку: сборка должна упасть, а не выпустить exe с dev-разрешениями.
 */
export function releaseCsp(html: string): string {
  const out = devOnly.reduce((s, part) => s.replace(part, ''), html)
  const csp = cspRe.exec(out)?.[1] ?? ''
  const left = ['ws://', 'worker-src'].filter((p) => csp.includes(p))
  if (left.length > 0) {
    throw new Error(
      `releaseCsp: в CSP index.html после замены остались dev-части (${left.join(', ')}); ` +
        'формат CSP изменился — обновите devOnly в src/shell/boot/csp.ts',
    )
  }
  return out
}
