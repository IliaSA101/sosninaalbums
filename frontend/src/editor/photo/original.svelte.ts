// Оригинал под лупой окна крупного просмотра (128, ступень 2): запрос /media/o/<id> после того, как зум выше 1:1 по превью
// устоялся, и разбор ответа. UI различает только успех и error у <img>: 404 и 415 для него одно — остаётся превью.
import { sameShape, type Size } from './loupe'

// Пауза после последнего шага зума перед запросом: колесо даёт десятки шагов, а в exe Go отдаёт оригиналы по одному.
export const ORIG_DELAY = 300

// idle — не запрошен; load — запрошен, грузится; ok — пришёл и ложится на превью; fail — ошибка или другие пропорции.
type Phase = 'idle' | 'load' | 'ok' | 'fail'

export class PreviewOriginal {
  private owner = $state('') // фото, которому принадлежит запрос; другое фото — оригинала нет
  private phase = $state<Phase>('idle')

  // Адрес нужен, пока запрос идёт или оригинал показан: снятый из разметки <img> убирает src (cancelOnUnmount).
  live(id: string | null | undefined): boolean {
    return !!id && this.owner === id && (this.phase === 'load' || this.phase === 'ok')
  }
  ready(id: string | null | undefined): boolean {
    return !!id && this.owner === id && this.phase === 'ok'
  }
  // Оригинал не годится (ошибка сервера или другие пропорции): предел зума — 1:1 по превью.
  failed(id: string | null | undefined): boolean {
    return !!id && this.owner === id && this.phase === 'fail'
  }
  // Оригинал нужен, но ещё не пришёл: ждём паузу или грузим — значок «Оригинал загружается…».
  waiting(id: string | null | undefined): boolean {
    return !!id && !this.ready(id) && !this.failed(id)
  }

  // Зум выше 1:1 по превью (want) устоялся на ORIG_DELAY — запрос. Возврат — снятие таймера. Другое фото сбрасывает прежний запрос.
  sync(id: string | null | undefined, want: boolean): (() => void) | void {
    if (this.owner !== id && this.phase !== 'idle') this.reset()
    if (!id || !want || this.phase !== 'idle') return
    const t = setTimeout(() => {
      this.owner = id
      this.phase = 'load'
    }, ORIG_DELAY)
    return () => clearTimeout(t)
  }

  // load у <img> оригинала: пропорции те же, что у превью (±1 %), — показываем; иначе WebView повернул его иначе, остаётся превью.
  loaded(id: string, got: Size, preview: Size) {
    if (this.owner !== id || this.phase !== 'load') return
    this.phase = sameShape(got, preview) ? 'ok' : 'fail'
  }
  error(id: string) {
    if (this.owner === id && this.phase === 'load') this.phase = 'fail'
  }

  reset() {
    this.owner = ''
    this.phase = 'idle'
  }
}
