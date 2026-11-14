import { admit } from './viewdelta'
import type { View } from './types'

// Цепочка видов (075 §1.4): номер последнего применённого вида, дубли, дельты не к нашему виду и пересинхрон.
// Вид сам не хранит и не пишет: берёт текущий у хозяина (store), отдаёт принятый ему же.
export interface ChainHost {
  current(): View
  show(v: View): void // принять вид: заменить store.view и пересчитать зависимое
  fetch(): Promise<View> // полный вид у Go (GetView)
  fail(e: unknown): void
}

export class ViewChain {
  // Номер последнего применённого вида; 0 — вид пришёл мимо цепочки.
  private seq = 0
  private resyncing = false
  private resyncSeq = 0
  private inflight: Promise<boolean> = Promise.resolve(true)

  constructor(private readonly host: ChainHost) {}

  // Принять вид. null — решено сразу (применён или пропущен); Promise — вид пришёл не к нашему,
  // идёт пересинхрон: он выполнится, когда вид догнан (true) или не удалось (false, ошибка уже показана).
  take(raw: View | null | undefined): Promise<boolean> | null {
    if (!raw) return null
    const a = admit(this.host.current(), this.seq, raw)
    if (a.kind === 'skip') return null
    if (a.kind === 'resync') return this.resync(a.seq)
    this.seq = a.view.seq ?? 0
    this.host.show(a.view)
    return null
  }

  // Один GetView за раз; дельты, пришедшие за это время, поднимают нужный seq и ждут тот же запрос.
  private resync(seq: number): Promise<boolean> {
    this.resyncSeq = Math.max(this.resyncSeq, seq)
    if (this.resyncing) return this.inflight
    this.resyncing = true
    this.inflight = (async () => {
      try {
        for (let i = 0; i < 3; i++) {
          this.take(await this.host.fetch())
          if (this.seq >= this.resyncSeq) break // ответ старше пришедших дельт — спрашиваем ещё раз
        }
        return true
      } catch (e) {
        this.host.fail(e)
        return false
      } finally {
        this.resyncing = false
        this.resyncSeq = 0
      }
    })()
    return this.inflight
  }
}
