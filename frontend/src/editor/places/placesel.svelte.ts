// Выделение и правка мест без режима (спека 007/008): ref выделенного, предпросмотр до отпускания,
// действия кнопок и клавиш. Геометрию считает Go; здесь — выделение и вызовы Api.*.
// Замок пропорций и привязка — настройки окна (localStorage, как modes), в проект не попадают.
//
// ref — ID фото (в «Авто» и в сетке) или ID пустого места. Выделение фото в store (selectedCell) —
// то же самое: первый ref, если это фото.
import { SvelteMap, SvelteSet } from 'svelte/reactivity'
import { store, Api } from '../../base/core/store.svelte'
import { gridAct } from '../../panels/layouts/gridact.svelte'
import { edit } from '../text/edit.svelte'
import { selectAll, toggleRef, toggleText, pruneTexts, pruneSel, type GroupSel } from './groupsel'
import { arrowDelta, nudge } from '../../base/geom/grid'
import { gridSession, refsBoxes, sampleFirst } from '../../base/geom/gridedit'
import { groupBox, type GxText } from '../../base/groupgeom/groupxf'
import { frameAngle, groupOBB, mixedAngles, reduce90, type Turn } from '../../base/groupgeom/groupframe'
import { normDeg } from '../../base/geom/textbox'
import { marqueePick, type Hit } from '../canvas/marquee'
import { layerTargets } from './layers'
import type { LayerOp } from './minipanel'
import type { Rect, RotRect } from '../../base/core/types'

// Ключ прежний: у пользователя уже сохранены замок и привязка.
const KEY = 'gridEdit'

export interface LiveText extends Rect {
  scale: number
  rot?: number // повёрнутый группой текст (018); нет — угол из View
}

class PlaceSel {
  // Выделенные ref текущего разворота; первый — образец (поля X/Y/Ш/В, толстая рамка).
  refs = $state<string[]>([])
  // Тексты группы (2+ объекта, спека 009 U5). Одиночный текст живёт в edit.sel, не здесь. Фона тут нет.
  texts = $state<string[]>([])
  // Предпросмотр переноса/размера до отпускания кнопки (мм разворота), ключ — ref.
  live = $state<{ spread: string; boxes: Map<string, RotRect> } | null>(null)
  // Тексты группы в жесте: рамка (угол до поворота, размер) и множитель кегля, ключ — ID текста.
  liveTexts = $state<Map<string, LiveText> | null>(null)
  // Голубые направляющие прилипания, мм.
  guides = $state<{ x: number | null; y: number | null }>({ x: null, y: null })
  // Последний поворот группы (018): угол рамки, пока выделение и углы членов те же. В проекте угла группы нет.
  lastTurn = $state<Turn | null>(null)
  // Угол рамки во время протяжки ⟳ (живой); вне жеста null.
  liveTurn = $state<number | null>(null)
  lock = $state(false)
  snap = $state(true)
  // Страница, над которой курсор был в последний раз: «+ Место» встаёт на неё.
  page: 0 | 1 = 0

  // Последнее добавленное щелчком (Shift/Ctrl) — образец «Одинакового размера»; null — первое выделенное.
  private sample: string | null = null
  private session = ''
  private chain: Promise<unknown> = Promise.resolve()

  constructor() {
    try {
      const v = JSON.parse(localStorage.getItem(KEY) ?? 'null')
      if (v && typeof v === 'object') {
        this.lock = v.lock === true
        this.snap = v.snap !== false
      }
    } catch {
      // Без хранилища — значения по умолчанию.
    }
  }

  private save() {
    try {
      localStorage.setItem(KEY, JSON.stringify({ lock: this.lock, snap: this.snap }))
    } catch {
      // Без хранилища настройка не запомнится.
    }
  }

  toggleLock() {
    this.lock = !this.lock
    this.save()
  }

  toggleSnap() {
    this.snap = !this.snap
    this.save()
  }

  // Выделенное на текущем развороте с рамками (образец первым); исчезнувшее отбрасывается.
  // id у каждого — то, что принимает Go: ID места (сетка) или фото («Авто»).
  selected = $derived(refsBoxes(store.spread, this.refs))

  // Места и тексты с ids (ID текстов; чужие ID отбрасываются) с живым предпросмотром жеста.
  private liveMembers(ids: string[]): { places: RotRect[]; texts: GxText[] } {
    const s = store.spread
    if (!s) return { places: [], texts: [] }
    const live = this.live?.spread === s.id ? this.live.boxes : null
    const liveT = this.liveTexts
    return {
      places: this.selected.map((b) => live?.get(b.id) ?? b),
      texts: s.texts
        .filter((t) => ids.includes(t.id))
        .map((t) => {
          const l = liveT?.get(t.id)
          return { x: l?.x ?? t.x, y: l?.y ?? t.y, w: l?.w ?? t.w, h: l?.h ?? t.h, rot: l?.rot ?? t.rot, size: t.style.size }
        }),
    }
  }

  // Описанный прямоугольник выделенных мест и текстов с ids (линейки): с живым предпросмотром. Нет выделенного — null.
  boxOf(ids: string[]): Rect | null {
    if (!store.spread) return null
    const m = this.liveMembers(ids)
    return groupBox(m.places, m.texts)
  }

  // Ключ выделения для угла рамки: разворот, refs и тексты группы.
  key = $derived(`${store.spread?.id}|${[...this.refs, ...this.texts.map((t) => 't:' + t)].sort().join(',')}`)

  // Текущие (не живые) углы членов группы: ID → °.
  private rots(): Map<string, number> {
    const m = new SvelteMap<string, number>()
    for (const i of this.selected) m.set(i.id, i.rot ?? 0)
    for (const t of store.spread?.texts ?? []) if (this.texts.includes(t.id)) m.set(t.id, t.rot)
    return m
  }

  // Угол рамки группы φ (спека 018 §2): живой при протяжке ⟳, иначе последний поворот, иначе общий угол членов, иначе 0.
  phi = $derived(this.liveTurn ?? frameAngle(this.lastTurn, this.key, this.rots()))
  // У членов разные углы и поворота нет — подсказка поля «Угол».
  mixed = $derived(this.liveTurn === null && mixedAngles(this.rots()))

  // Рамка группы: описанный прямоугольник в осях φ (рисует GroupFrame, поля карточки, мини-панель), с живым предпросмотром.
  frame = $derived.by(() => {
    const m = this.liveMembers(this.texts)
    return groupOBB(m.places, m.texts, reduce90(this.phi))
  })

  // После RotateGroup: запомнить угол рамки с углами членов из свежего вида (вызывать после обновления store).
  // key — ключ выделения, снятый ДО вызова Go: за ожидание выделение могло смениться, тогда поворот забывается.
  noteTurn(deg: number, key: string) {
    if (key !== this.key) return
    this.lastTurn = { key, deg: normDeg(deg), rots: this.rots() }
  }

  // Новое выделение (ключ сменился) забывает последний поворот: угол — общий угол членов, иначе 0°.
  private dropTurn() {
    if (this.lastTurn && this.lastTurn.key !== this.key) this.lastTurn = null
  }

  // Правка по очереди: следующая берёт вид после предыдущей (удержанная стрелка в поле шлёт change чаще, чем
  // отвечает Go). Сбой одной не рвёт цепочку.
  queue<T>(fn: () => Promise<T>): Promise<T> {
    const r = this.chain.then(fn)
    this.chain = r.catch(() => undefined)
    return r
  }

  // ---------- выделение ----------

  select(refs: string[]) {
    this.refs = refs
    this.texts = []
    this.sample = null
    this.session = ''
    this.dropTurn()
  }

  // Что выделено сейчас (одиночный текст — из edit; подпись в группу не входит).
  private state(): GroupSel {
    const t = edit.sel?.kind === 'text' && edit.sel.spread === store.spread?.id ? edit.sel.id : null
    return { refs: this.refs, texts: this.texts, text: t }
  }

  // Применить выделение: одиночный текст — в edit, остальное — сюда. edit.select сбрасывает placesel,
  // поэтому refs и texts ставятся после него.
  private apply(g: GroupSel) {
    const sid = store.spread?.id
    if (g.text && sid) edit.select({ kind: 'text', spread: sid, id: g.text })
    else if (edit.sel) edit.clear()
    this.refs = g.refs
    this.texts = g.texts
    this.sample = null
    this.session = ''
    this.dropTurn()
  }

  // Shift/Ctrl+щелчок по фото или месту — в группу и из неё (одиночный текст уходит в группу).
  pick(ref: string, additive: boolean) {
    if (additive) {
      this.apply(toggleRef(this.state(), ref))
      if (this.refs.includes(ref)) this.sample = ref
    } else this.select([ref])
    this.session = gridSession()
  }

  // Shift/Ctrl+щелчок по тексту.
  pickText(id: string) {
    this.apply(toggleText(this.state(), id))
  }

  // Ctrl+A на холсте: все фото, места и тексты текущего разворота.
  selectAll() {
    this.apply(selectAll(store.spread))
  }

  // Выделение рамкой (Marquee): найденное заменяет выделение или, с Shift/Ctrl, добавляется к нему.
  pickHit(hit: Hit, additive: boolean) {
    this.apply(marqueePick(this.state(), hit, additive))
  }

  // Группа: 2 и больше объекта (места, фото, тексты).
  get isGroup() {
    return this.selected.length + this.texts.length > 1
  }

  // Сколько объектов выделено (фото, места, тексты; группа — от 2).
  get count() {
    return this.refs.length + this.texts.length + (edit.sel?.kind === 'text' ? 1 : 0)
  }

  clear() {
    this.refs = []
    this.texts = []
    this.sample = null
    this.live = null
    this.liveTexts = null
    this.liveTurn = null
    this.lastTurn = null
    this.guides = { x: null, y: null }
    this.session = ''
  }

  // После правки: фото, которых больше нет в альбоме, и места, которых нет на развороте, выпадают;
  // так же тексты.
  prune() {
    const places = store.spread?.places
    const left = this.refs.filter((r) => store.used.has(r) || !!places?.some((p) => p.id === r))
    if (left.length !== this.refs.length) this.refs = left
    const texts = pruneTexts(this.texts, store.spread)
    if (texts === this.texts) return
    // Из группы из двух выпал один: оставшийся одиночный текст живёт в edit.sel, а не в texts.
    this.apply(pruneSel(this.state(), store.spread))
  }

  // ---------- правки ----------

  // Стрелки (только на сетке): серия — один шаг undo (merge = сессия). Очередь, чтобы удержанная
  // клавиша не считала от устаревшего вида: каждый шаг берёт рамки из актуального store.view.
  // Группа (2+ объекта, в том числе с текстами) едет через TransformGroup, одно место — через SetPlaces.
  nudge(key: string, shift: boolean) {
    const sid = store.spread?.id
    if (!sid || !store.spread?.grid || (this.refs.length === 0 && this.texts.length === 0)) return
    if (!this.session) this.session = gridSession()
    const merge = this.session
    const refs = [...this.refs]
    const texts = [...this.texts]
    this.chain = this.chain.then(async () => {
      const s = store.view.spreads.find((x) => x.id === sid)
      const cur = refsBoxes(s, refs)
      if (!s?.grid) return
      if (cur.length + texts.length > 1) {
        const { dx, dy } = arrowDelta(key, shift)
        const ids = [...cur.map((b) => b.id), ...pruneTexts(texts, s)]
        await gridAct.placeEdit(sid, Api.TransformGroup(sid, ids, 1, 0, 0, dx, dy, merge))
        return
      }
      if (cur.length === 0) return
      // Повёрнутое место зажимает Go (вылет не растёт): прижимать его к неповёрнутым границам нельзя — прыгнет.
      const { dx, dy } = arrowDelta(key, shift)
      const moved = cur[0].rot
        ? [{ x: cur[0].x + dx, y: cur[0].y + dy, w: cur[0].w, h: cur[0].h }]
        : nudge(cur, key, shift, { w: store.view.settings.pageW * 2, h: store.view.settings.pageH })
      await gridAct.placeEdit(
        sid,
        Api.SetPlaces(
          sid,
          moved.map((b, i) => ({ id: cur[i].id, x: b.x, y: b.y, w: b.w, h: b.h })),
          merge,
        ),
      )
    })
  }

  // Delete / «Удалить»: тексты группы и фото уходят разом, одним шагом undo (Api.DeleteSelection);
  // фото — в ленту (у «Авто» раскладка перестроится, у сетки место остаётся пустым и выделенным);
  // пустое место — убирается (второй Delete по месту).
  async deleteSelected() {
    const s = store.spread
    const texts = [...this.texts]
    const items = refsBoxes(s, this.refs)
    const photos = new SvelteSet(items.map((i) => i.photo).filter(Boolean))
    // Выделенное из ленты альбома может ещё не быть на этом развороте.
    for (const r of this.refs) if (store.used.has(r)) photos.add(r)
    const empties = items.filter((i) => !i.photo).map((i) => i.id)
    // Есть фото — пустые места остаются выделенными (второй Delete их уберёт).
    const places = photos.size > 0 ? [] : empties
    if (texts.length === 0 && photos.size === 0 && places.length === 0) return
    const freed = photos.size > 0 && s?.grid ? items.filter((i) => i.photo).map((i) => i.id) : []
    if (!(await store.run(Api.DeleteSelection(s?.id ?? '', [...photos], places, texts)))) return
    // Удалённые тексты и места выпадают из выделения сами (apply → prune).
    if (photos.size > 0) this.select([...freed, ...empties])
  }

  // «+ Место»: на страницу под курсором (последнюю наведённую), новое место выделяется.
  // У «Авто» первая такая правка закрепляет разворот (gridAct.placeEdit скажет об этом).
  async addPlace() {
    const s = store.spread
    if (!s) return
    const before = new Set((s.places ?? []).map((p) => p.id))
    if (!(await gridAct.placeEdit(s.id, Api.AddPlace(s.id, this.page)))) return
    // У «Авто» до правки мест не было: новое — пустое место, а не закреплённые ячейки.
    const added = store.spread?.places?.find((p) => !before.has(p.id) && !p.photo)
    if (added) this.pick(added.id, false)
  }

  // ArrangePlaces: same | sameW | sameH | fitPhoto | splitH | splitV | front | back; образец «same» — последнее щёлкнутое, иначе первое выделенное.
  async arrange(op: string) {
    const s = store.spread
    if (!s || this.selected.length === 0) return
    await gridAct.placeEdit(s.id, Api.ArrangePlaces(s.id, op, this.ordered(op)))
  }

  // Id выделенного для ArrangePlaces; у «Одинакового размера» образец (Go берёт первый id) — последнее щёлкнутое.
  private ordered(op: string): string[] {
    const ids = this.selected.map((i) => i.id)
    if (op !== 'same' && op !== 'sameW' && op !== 'sameH') return ids
    return sampleFirst(ids, this.sample ? refsBoxes(store.spread, [this.sample])[0]?.id : null)
  }

  // Зеркало ↔ (axis 'h') / ↕ ('v') у выделенных фото: Go сам решает «всем или никому»; раскладку
  // не трогает, поэтому store.run, а не placeEdit (закрепления нет).
  async flip(axis: 'h' | 'v') {
    const photos = this.selected.map((i) => i.photo).filter(Boolean)
    if (photos.length === 0) return
    await store.run(Api.SetFlip(photos, axis))
  }

  // ↻ 90° (quarters 1) / ↺ (−1) у выделенных фото: поворот кадра, место стоит; «Авто» Go пересобирает сам.
  // Раскладку не закрепляет, поэтому store.run, а не placeEdit.
  async turn(quarters: -1 | 1 | 2) {
    const photos = this.selected.map((i) => i.photo).filter(Boolean)
    if (photos.length === 0) return
    await store.run(Api.TurnPhotos(photos, quarters))
  }

  // Цели слоя, выравнивания и линеек: выделенные места и фото, тексты группы и одиночный текст
  // (подпись не объект — она при фото). Одна производная на всех.
  targetIds = $derived.by(() => {
    const sid = store.spread?.id
    const t = edit.sel
    return layerTargets(
      this.selected.map((i) => i.id),
      this.texts,
      sid && t?.kind === 'text' && t.spread === sid ? t.id : null,
    )
  })
  // Слои есть только у «Ручной»: у «Авто» Go порядок сбрасывает. Уровень проверяют вызывающие (prefs.can('layers')).
  canLayer = $derived(!!store.spread?.grid && this.targetIds.length > 0)

  // Порядок слоёв выделенного (front | up | down | back): места, фото и тексты — один стек Go.
  async layer(op: LayerOp) {
    const s = store.spread
    if (!s || !this.canLayer) return
    // Цель и разворот фиксируем до ожидания: за него выделение или разворот могут смениться.
    const ids = [...this.targetIds]
    if (ids.length === 0) return
    // Набранный текст уходит раньше (idle досылает его сразу), иначе порядок применится к старому состоянию.
    await edit.idle()
    await store.run(Api.ArrangeLayers(s.id, op, ids))
  }

  // «Целиком / С обрезкой» в карточке: подсвечено, только если так у всех выделенных.
  allContain = $derived(this.selected.length > 0 && this.selected.every((p) => p.fit === 'contain'))
  allCover = $derived(this.selected.length > 0 && this.selected.every((p) => p.fit !== 'contain'))
  async setFit(contain: boolean) {
    const s = store.spread
    if (!s || this.selected.length === 0 || (contain ? this.allContain : this.allCover)) return
    await gridAct.placeEdit(
      s.id,
      Api.SetPlaceFit(
        s.id,
        this.selected.map((i) => i.id),
        contain ? 'contain' : '',
      ),
    )
  }
}

export const placesel = new PlaceSel()
