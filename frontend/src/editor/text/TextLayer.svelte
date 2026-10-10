<script lang="ts">
  import { untrack } from 'svelte'
  import type { CaptionView, Settings, SpreadView, TextView } from '../../base/core/types'
  import { store } from '../../base/core/store.svelte'
  import { edit } from './edit.svelte'
  import { placesel } from '../places/placesel.svelte'
  import { additive } from '../places/gesture'
  import { emptyField } from '../canvas/field'
  import { moveSelected } from '../places/groupgesture'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { svgTextCss } from './fonts'
  import { layerIndex } from '../places/layers'
  import type { TextGestures } from './textgesture.svelte'
  import { pickOwns } from '../canvas/pickunder.svelte'

  // Блоки текста и подписи — в общем стеке `.content` разворота (спека 010 U6): у каждого своя обёртка
  // с z-index = 10 + z из Go, поэтому текст может лежать под фото. Внутри — SVG строк (уже перенесены
  // в Go, поворот — Rot; блок крутится целиком вокруг своего угла x, y) и, на большом развороте, прозрачная
  // рамка для выделения и переноса. Ручки, редактор и подсказки жестов — выше стека (TextOverlay).
  let { spread, settings, mini = false, tg }: { spread: SpreadView; settings: Settings; mini?: boolean; tg: TextGestures } = $props()

  const sw = $derived(settings.pageW * 2)
  const ph = $derived(settings.pageH)
  const pctX = (v: number) => `${(v / sw) * 100}%`
  const pctY = (v: number) => `${(v / ph) * 100}%`

  function downText(e: PointerEvent, t: TextView) {
    if (e.button !== 0) return
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
    if (e.button !== 0) return
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
    return !!(el?.closest?.('[data-photo], [data-place]') || (el?.closest?.('.stage') && emptyField(el)))
  }

  // Глобальные слушатели — только у большого разворота.
  $effect(() => {
    if (mini) return
    // Клик мимо: конец ввода и снятие выделения; свои элементы и панель текста помечены data-text-ui.
    const down = (e: PointerEvent) => {
      if (e.button !== 0 || !edit.sel || edit.sel.spread !== spread.id) return
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
            {#if l.text}<text x={l.x} y={l.y} style={svgTextCss(c.style, store.fonts)}>{l.text}</text>{/if}
          {/each}
        </g>
      </svg>
      {#if !mini}
        <!-- Обводку выделенной подписи и метку «!» рисует TextOverlay: в стеке их перекрыло бы фото выше -->
        <div
          class="hit caption"
          style:left={pctX(c.x)}
          style:top={pctY(c.y)}
          style:width={pctX(c.w)}
          style:height={pctY(c.h)}
          style:transform={c.rot ? `rotate(${c.rot}deg)` : undefined}
          data-text-ui
          data-through
          role="presentation"
          onpointerdown={(e) => downCaption(e, c)}
          ondblclick={() => edit.startEdit()}
        ></div>
      {/if}
    </div>
  {/if}
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
            {#if l.text}<text x={l.x} y={l.y} style={svgTextCss(t.style, store.fonts)}>{l.text}</text>{/if}
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
