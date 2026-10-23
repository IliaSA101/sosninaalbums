<script lang="ts">
  import { primaryPress } from '../../base/core/platform'
  import { untrack } from 'svelte'
  import type { CaptionView, SpreadView, TextStyle, TextView } from '../../base/core/types'
  import { store } from '../../base/core/store.svelte'
  import { edit } from './edit.svelte'
  import { placesel } from '../places/placesel.svelte'
  import { additive } from '../places/gesture'
  import { emptyField } from '../canvas/field'
  import { moveSelected } from '../places/groupgesture'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { lineLen, svgTextCss } from './fonts'
  import { colorLive } from '../../base/color/colorlive.svelte'
  import { layerIndex } from '../places/layers'
  import type { TextGestures } from './textgesture.svelte'
  import { pickOwns } from '../canvas/pickunder.svelte'
  import { photoui } from '../photo/photoui.svelte'

  // Блоки текста и подписи — в общем стеке `.content` разворота (спека 010 U6): у каждого своя обёртка
  // с z-index = 10 + z из Go, поэтому текст может лежать под фото. Внутри — SVG строк (уже перенесены
  // в Go, поворот — Rot; блок крутится целиком вокруг своего угла x, y) и, на большом развороте, прозрачная
  // рамка для выделения и переноса. Ручки, редактор и подсказки жестов — выше стека (TextOverlay).
  let { spread, mini = false, tg }: { spread: SpreadView; mini?: boolean; tg: TextGestures } = $props()

  // Стиль строки с живым цветом из окна цвета (109); нет показа — как в View.
  const liveCss = (key: string, st: TextStyle) => svgTextCss(colorLive.style(key, st), store.fonts)

  const sw = $derived(spread.w)
  const ph = $derived(spread.h)
  const pctX = (v: number) => `${(v / sw) * 100}%`
  const pctY = (v: number) => `${(v / ph) * 100}%`

  function downText(e: PointerEvent, t: TextView) {
    if (!primaryPress(e)) return
    e.stopPropagation()
    // Текст «Стандартного» уровня на «Простом»: виден, но не выделяется (вопрос 5), только тост.
    if (prefs.blocked('text', () => edit.select({ kind: 'text', spread: spread.id, id: t.id }))) return
    // Shift/Ctrl+щелчок — в группу или из неё; перенос не начинается.
    if (additive(e)) return placesel.pickText(t.id)
    if (tg.inGroup(t.id)) return downMember(e, t)
    edit.select({ kind: 'text', spread: spread.id, id: t.id })
    tg.start(e, t, 'move')
  }

  // Тело члена группы: «Ручная» переносит всю группу, «Авто» — только этот текст (группа ждёт);
  // щелчок без движения оставляет один этот текст.
  function downMember(e: PointerEvent, t: TextView) {
    const solo = () => edit.select({ kind: 'text', spread: spread.id, id: t.id })
    const root = tg.root
    if (spread.grid && root) return moveSelected(e, root, solo)
    // Щелчок без движения решает сам жест (порог, Esc отменяет и его).
    tg.start(e, t, 'move', solo)
  }

  function downCaption(e: PointerEvent, c: CaptionView) {
    if (!primaryPress(e)) return
    e.stopPropagation()
    // Без функции «Подписи» готовая подпись видна и печатается, но не выделяется (спека 011): тост.
    if (prefs.blocked('caption', () => edit.select({ kind: 'caption', spread: spread.id, id: c.photo }))) return
    edit.select({ kind: 'caption', spread: spread.id, id: c.photo })
  }

  // Нажатие по своим элементам или с Shift/Ctrl на фото, месте и пустом поле выделение текста не снимает:
  // Shift/Ctrl+щелчок по фото или месту переносит одиночный текст в группу, а по пустому полю
  // (начало рамки с добавлением) — оставляет как есть.
  function keepsSelection(e: PointerEvent): boolean {
    const el = e.target as Element | null
    if (el?.closest?.('[data-text-ui]')) return true
    if (!additive(e)) return false
    return !!(el?.closest?.('[data-photo], [data-place], [data-sticker]') || (el?.closest?.('.stage') && emptyField(el)))
  }

  // Глобальные слушатели — только у большого разворота.
  $effect(() => {
    if (mini) return
    // Клик мимо: конец ввода и снятие выделения; свои элементы и панель текста помечены data-text-ui.
    const down = (e: PointerEvent) => {
      if (!primaryPress(e) || !edit.sel || edit.sel.spread !== spread.id) return
      // Перенос выбранного нижнего берёт Alt-выбор (спека 015): выделение не снимаем.
      if (pickOwns(e)) return
      if (keepsSelection(e)) return
      edit.clear()
    }
    // Страница под курсором — для кнопки «Текст».
    const over = (e: PointerEvent) => {
      const r = tg.rect()
      if (!r || e.clientX < r.left || e.clientX > r.right || e.clientY < r.top || e.clientY > r.bottom) return
      edit.lastPage = e.clientX < r.left + r.width / 2 ? 0 : 1
    }
    window.addEventListener('pointerdown', down, true)
    window.addEventListener('pointermove', over)
    return () => {
      window.removeEventListener('pointerdown', down, true)
      window.removeEventListener('pointermove', over)
    }
  })

  // Другой разворот — выделение прежнего снимается (досылая набранное).
  $effect(() => {
    const id = spread.id
    if (mini) return
    untrack(() => {
      if (edit.sel && edit.sel.spread !== id) edit.clear()
    })
  })
  // Блок пропал (Ctrl+Z, удаление, пустой текст) — выделять нечего.
  $effect(() => {
    if (!mini && edit.sel?.spread === spread.id) edit.dropIfGone()
  })
  // Выделение фото и текста взаимоисключающие: фото выбрали — текст отпускаем.
  $effect(() => {
    if (!mini && store.selectedCell && edit.sel) untrack(() => edit.clear())
  })
</script>

{#each spread.captions as c (c.photo)}
  {#if !tg.editing('caption', c.photo)}
    <div class="blk" style:z-index={layerIndex(c.z)}>
      <svg viewBox="0 0 {sw} {ph}" preserveAspectRatio="none" aria-hidden="true">
        <!-- Подпись повёрнутого места: Go отдаёт угол рамки и Rot, как у текста (экспорт рисует так же) -->
        <g transform={c.rot ? `rotate(${c.rot} ${c.x} ${c.y})` : undefined}>
          {#each c.lines as l, i (i)}
            {#if l.text}<text x={l.x} y={l.y} textLength={lineLen(l.w)} lengthAdjust="spacing" style={liveCss(`cap:${c.photo}`, c.style)}
                >{l.text}</text
              >{/if}
          {/each}
        </g>
      </svg>
      {#if !mini}
        <!-- Обводку выделенной подписи и метку «!» рисует TextOverlay: в стеке их перекрыло бы фото выше -->
        <div
          class="hit caption"
          class:named={!!c.who?.length}
          style:left={pctX(c.x)}
          style:top={pctY(c.y)}
          style:width={pctX(c.w)}
          style:height={pctY(c.h)}
          style:transform={c.rot ? `rotate(${c.rot}deg)` : undefined}
          data-text-ui
          data-through
          role="presentation"
          onpointerdown={(e) => downCaption(e, c)}
          ondblclick={() => (c.who?.length ? undefined : edit.startEdit())}
        ></div>
      {/if}
    </div>
  {/if}
{/each}

<!-- Заглушки подписей (030): серые, только на холсте; мышь не ловят и в стек выбора не входят -->
{#each spread.stubs ?? [] as s (s.place + '|' + s.photo)}
  <div class="blk stub" style:z-index={layerIndex(s.z)}>
    <svg viewBox="0 0 {sw} {ph}" preserveAspectRatio="none" aria-hidden="true">
      <g transform={s.rot ? 'rotate(' + s.rot + ' ' + s.x + ' ' + s.y + ')' : undefined}>
        {#each s.lines as l, i (i)}
          {#if l.text}<text x={l.x} y={l.y} textLength={lineLen(l.w)} lengthAdjust="spacing" style={svgTextCss(s.style, store.fonts)}
              >{l.text}</text
            >{/if}
        {/each}
      </g>
    </svg>
  </div>
{/each}

{#each spread.texts as t (t.id)}
  {#if !tg.editing('text', t.id)}
    {@const e = tg.eff(t)}
    {@const shift = e.mode === 'move' || e.mode === 'rotate' || e.mode === 'scale'}
    <div class="blk" style:z-index={layerIndex(t.z)}>
      <svg viewBox="0 0 {sw} {ph}" preserveAspectRatio="none" aria-hidden="true">
        <g
          transform="translate({shift ? e.x - t.x : 0} {shift
            ? e.y - t.y
            : 0}) rotate({e.rot} {t.x} {t.y}) translate({t.x} {t.y}) scale({e.scale}) translate({-t.x} {-t.y})"
        >
          {#each t.lines as l, i (i)}
            {#if l.text}<text x={l.x} y={l.y} textLength={lineLen(l.w)} lengthAdjust="spacing" style={liveCss(`text:${t.id}`, t.style)}
                >{l.text}</text
              >{/if}
          {/each}
        </g>
      </svg>
      {#if !mini}
        <!-- Обводку выделенного одиночного блока рисует TextOverlay (выше стека); здесь — только член группы -->
        <div
          class="hit block"
          class:selected={tg.inGroup(t.id) && !tg.selected('text', t.id)}
          class:moving={e.mode === 'move'}
          style:left={pctX(e.x)}
          style:top={pctY(e.y)}
          style:width={pctX(e.w)}
          style:height={pctY(e.h)}
          style:transform={e.rot ? `rotate(${e.rot}deg)` : undefined}
          data-text-ui
          data-through
          role="presentation"
          onpointerdown={(ev) => downText(ev, t)}
          onclick={(ev) => ev.stopPropagation()}
          oncontextmenu={(ev) => photoui.openText(ev, t.id)}
          ondblclick={() => edit.startEdit()}
        ></div>
      {/if}
    </div>
  {/if}
{/each}

<style>
  /* Обёртка блока лежит в стеке `.content` со своим z-index и мышь не ловит — её берёт только рамка. */
  .blk {
    position: absolute;
    inset: 0;
    pointer-events: none;
  }
  /* Заглушка бледнее подписи: видно, что это место под имя, а не готовый текст */
  .stub {
    opacity: 0.45;
    filter: grayscale(1);
  }
  svg {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    overflow: visible;
    pointer-events: none;
  }
  .hit {
    position: absolute;
    transform-origin: 0 0; /* поворот вокруг угла x, y, как в Go */
    pointer-events: auto;
    touch-action: none;
    cursor: move;
  }
  .hit.caption {
    cursor: pointer;
  }
  /* Подпись по шаблону: двойной щелчок вводит имя (037) */
  .hit.caption.named {
    cursor: text;
  }
  .hit:hover {
    outline: 1px dashed rgba(245, 217, 10, 0.7);
    outline-offset: 2px;
  }
  .hit.selected {
    outline: 1.5px solid var(--accent);
    outline-offset: 2px;
  }
  .hit.moving {
    cursor: grabbing;
  }
</style>
