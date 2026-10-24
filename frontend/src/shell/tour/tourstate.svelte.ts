// Первые шаги (спека 113, design §4.4–4.5): состояние тура и его исполнение. Решения «что в шаге» — tour.ts под тестами;
// здесь вызовы store/prefs/guide. Тур не модальный и проект не правит: лист, выделение и масштаб — вид, кольцо держит
// общий Spot справочника (режим hold).
import { tick } from 'svelte'
import { Api, store } from '../../base/core/store.svelte'
import { jobs } from '../../base/core/jobs.svelte'
import { modalStack } from '../../base/modal/modalstack.svelte'
import { prefs } from '../../base/prefs/prefs.svelte'
import { hasSources } from '../../panels/folders/folders'
import { aboutUi } from '../dialogs/about.svelte'
import { guide, runStep } from '../guide/guidestate.svelte'
import { seek, spot } from '../guide/spotstate.svelte'
import { activeSteps, enterSteps, hasPhotoOnSpread, resolve, visibleBox, type Box, type TourCtx, type TourFrom } from './tour'
import { MORE_TOAST, SKIP_TOAST } from './steps'

// Строка в log.txt через App.LogUI; без рантайма Go (браузер, тесты) молча.
function logUi(text: string) {
  try {
    Api.LogUI(text).catch(() => {})
  } catch {
    // журнала нет — тур от этого не зависит
  }
}

export type TourAt = 'intro' | 'final' | number

class Tour {
  open = $state(false)
  // Приветствие, финальная карточка или номер шага среди активных.
  at = $state<TourAt>('intro')
  from = $state<TourFrom>('menu')
  // Места на экране нет (seek не нашёл ни его, ни «⋯»): кольца нет, карточка по центру.
  missing = $state(false)
  // Открыто меню или поповер, начатые щелчком из выреза (Tour.svelte следит за body): ловушка их не закрывает.
  popup = $state(false)
  // Тур уже был в этом сеансе (любой запуск): автозапуск повторно приветствие не открывает. Иначе после закрытия тура,
  // начатого из меню, пока Go не ответил на SetTour, tourDue ещё true — приветствие вернулось бы и перетёрло done на pending.
  started = false
  private entered = false
  private seq = 0
  private foreign = false
  private ownSel: string | null = null
  private sawPhoto = false
  private offered = false

  steps = $derived(activeSteps(prefs.can, this.from))
  step = $derived(typeof this.at === 'number' ? this.steps[this.at] : undefined)
  // Состояние окна для плана: пересчитывается при изменении альбома, выделения и уровня.
  ctx = $derived<TourCtx>({ ...guide.ctx(), sources: hasSources(store.view) })
  res = $derived(this.step ? resolve(this.step, this.ctx) : null)
  // Ключ, при смене которого место ищется заново.
  key = $derived(`${this.at}|${this.res?.spot ?? ''}`)

  // Открыто окно («Автосборка», F1, «Открываем альбом») или «Показать» справочника ведёт свой план: тур кольцо не берёт.
  // Без чтения spot.cur: от него зависит эффект поиска места, а сам поиск меняет spot.cur.
  get blocked(): boolean {
    return modalStack.any() || jobs.openShown || guide.busy
  }

  // Карточка, затемнение и кольцо спрятаны: blocked или мигает чужое кольцо «Показать».
  get hidden(): boolean {
    return this.blocked || (!!spot.cur && !spot.cur.hold)
  }

  // Вне выреза щелчки проходят, пока открыта галерея раскладок или меню/поповер из выреза: иначе их не нажать.
  get passThrough(): boolean {
    return this.popup || this.ctx.galleryOpen
  }

  // Рамка кольца тура в px окна; null — кольца нет (приветствие, финал, места нет).
  get ring(): Box | null {
    return this.open && !this.hidden && spot.cur?.hold ? spot.box : null
  }

  // Области шага, которых карточка не закрывает (step.avoid), в px окна: видимая часть, нет на экране — пропуск.
  // Читает ring, поэтому пересчитывается вместе с кольцом (тики spot.box) и при resize окна.
  get avoid(): Box[] {
    if (!this.ring || !this.step?.avoid) return []
    const win = { w: window.innerWidth, h: window.innerHeight }
    const out: Box[] = []
    for (const sel of this.step.avoid)
      for (const el of document.querySelectorAll<HTMLElement>(sel)) {
        const r = el.getBoundingClientRect()
        const b = visibleBox({ left: r.left, top: r.top, width: r.width, height: r.height }, win)
        if (b) out.push(b)
      }
    return out
  }

  // from: 'auto' — первый запуск (пишет pending), 'menu' — «Справка ▸ Первые шаги» и стартовое окно, 'more' — продолжение.
  start(from: TourFrom) {
    if (prefs.startOpen) prefs.closeStart()
    this.seq++
    this.from = from
    this.open = true
    this.entered = false
    this.missing = false
    this.ownSel = null
    this.sawPhoto = false
    this.foreign = false
    this.popup = false
    this.started = true
    if (from === 'auto') void prefs.setTour('first', 'pending')
    if (from === 'more') this.go(0)
    else this.at = 'intro'
  }

  // К шагу i: чужое кольцо гаснет, вид приводится (один раз при входе), место ищет эффект по смене key.
  go(i: number) {
    if (!this.open) return
    if (spot.cur && !spot.cur.hold) spot.cancel()
    this.foreign = false
    const step = this.steps[i]
    if (!step) return
    this.at = i
    this.missing = false
    const ctx = this.ctx
    const plan = enterSteps(step, ctx, !this.entered)
    this.entered = true
    const select = plan.some((s) => s.do === 'selectPhoto')
    for (const s of plan) runStep(s)
    // Выделение, поставленное самим туром, на закрытии снимается; своё человек не теряет.
    if (select && !ctx.selPhoto) this.ownSel = store.selectedCell
  }

  // Число шагов изменилось (уровень «Свой» выключил часть функций): номер шага зажимается в список.
  // Шагов не осталось — тур закрывается как пропущенный (done не пишется, если тур не был первым).
  fit(count: number) {
    if (!this.open || typeof this.at !== 'number' || this.at < count) return
    if (count === 0) this.finish('skip')
    else this.go(count - 1)
  }

  next() {
    if (!this.open) return
    if (this.at === 'intro') this.go(0)
    else if (this.at === 'final') this.finish('done')
    else if (this.at + 1 < this.steps.length) this.go(this.at + 1)
    else {
      this.release()
      this.at = 'final'
    }
  }

  back() {
    if (this.open && typeof this.at === 'number' && this.at > 0) this.go(this.at - 1)
  }

  // Тур пройден: «Готово»/«Закрыть» — done, «Пропустить» и Esc — skip (с тостом). Закрыли программу посреди тура — в
  // config.json остаётся pending, и при следующем запуске приветствие придёт снова.
  finish(reason: 'done' | 'skip', via: 'click' | 'key' = 'click') {
    if (!this.open) return
    // Запись done необратима (q113-done-sticky): в log.txt остаётся, чем и на каком шаге тур закрыт.
    logUi(`тур закрыт: ${reason}, ${via}, шаг ${String(this.at)}, запуск ${this.from}`)
    this.open = false
    this.popup = false
    this.seq++
    this.release()
    if (this.ownSel && store.selectedCell === this.ownSel) store.selectedCell = null
    this.ownSel = null
    // Фото на разворотах уже есть: продолжение предлагать незачем — человек его и так видел.
    const photos = this.sawPhoto || hasPhotoOnSpread(this.ctx)
    void this.persist(this.from === 'auto' || prefs.tourDue, this.from !== 'more' && photos && (this.from === 'auto' || prefs.tourMore))
    if (reason === 'skip') store.notify(SKIP_TOAST)
  }

  // Две записи подряд: ответы с флагами не должны перегнать друг друга.
  private async persist(first: boolean, spread: boolean) {
    if (first) await prefs.setTour('first', 'done')
    if (spread) await prefs.setTour('spread', 'done')
  }

  // Шаг 6 был живым: «продолжение» не нужно.
  markPhoto() {
    this.sawPhoto = true
  }

  // Кольцо тура убрать (чужое не трогаем).
  release() {
    if (spot.cur?.hold) spot.cancel()
  }

  // Найти место шага и взять кольцо в режим hold. Вызывает эффект Tour.svelte при смене key и снятии «скрыто».
  async place() {
    const my = ++this.seq
    if (!this.open) return
    if (spot.cur && !spot.cur.hold) {
      this.foreign = true // кольцо «Показать»: ждём его конца (onSpot)
      return
    }
    this.foreign = false
    const res = this.res
    if (!res) return this.release()
    await tick()
    if (my !== this.seq || !this.open) return
    const found = await seek(res.spot)
    if (my !== this.seq || !this.open) return
    if (spot.cur && !spot.cur.hold) {
      this.foreign = true
      return
    }
    this.missing = !found
    if (found) spot.flash(found.el, '', '', undefined, { hold: true, onLost: () => void this.place() })
    else this.release()
    if (res.live && this.step?.id === 'photo') this.markPhoto()
  }

  // Слежение за кольцом: чужое погасло — тур берёт кольцо снова.
  onSpot(cur: { hold: boolean } | null) {
    if (!this.open) return
    if (cur && !cur.hold) this.foreign = true
    else if (!cur && this.foreign && !this.blocked) {
      this.foreign = false
      void this.place()
    }
  }

  // Предложение продолжения (один раз): тост с «Показать». Тост в окне один: пока виден другой (итог обновления,
  // «Вернуть»), предложение ждёт — иначе оно его вытеснит, а пометка «done» сгорит впустую (q113-offer).
  // Пометка пишется только при фактическом показе; Tour.svelte следит за store.toast и зовёт снова.
  offerMore() {
    if (this.offered || store.toast) return
    this.offered = true
    store.notify(MORE_TOAST, 'info', { label: 'Показать', run: () => this.start('more') })
    void prefs.setTour('spread', 'done')
  }

  video() {
    void aboutUi.link('videos')
  }

  openArticle(id: string) {
    guide.openArticle(id)
  }
}

export const tour = new Tour()
