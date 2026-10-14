<script lang="ts">
  import type { CaptionView, SpreadView } from '../../base/core/types'
  import { store } from '../../base/core/store.svelte'
  import { edit } from './edit.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { boxOf, toSpread } from '../../base/geom/textbox'
  import { editText } from './textedit'
  import type { TextGestures } from './textgesture.svelte'
  import TextEditor from './TextEditor.svelte'
  import TextHandles from './TextHandles.svelte'

  // Над стеком разворота (спека 010 U6): редактор текста, рамка выделенного блока с ручками и подсказки
  // жеста. Блок может лежать под фото, а ручки и редактор обязаны быть доступны — поэтому они вне `.content`.
  let { spread, width, tg }: { spread: SpreadView; width: number; tg: TextGestures } = $props()

  const sw = $derived(spread.w)
  const ph = $derived(spread.h)
  const k = $derived(width / sw) // пикселей на мм
  const pctX = (v: number) => `${(v / sw) * 100}%`
  const pctY = (v: number) => `${(v / ph) * 100}%`
  const gesture = $derived(tg.gesture)
  const warn = (c: CaptionView) => `не хватает ${Math.ceil(c.overflow)} мм: уменьшите кегль${c.over ? '' : ' или поставьте на фото'}`
</script>

<div class="tover">
  {#each spread.captions as c (c.photo)}
    {#if tg.editing('caption', c.photo)}
      <TextEditor box={boxOf(c)} auto={false} maxW={store.view.textMaxW} text={editText(c)} style={c.style} {sw} {ph} {k} />
    {:else}
      <!-- Подпись не слой (она при фото), но обводка и «!» должны быть видны, даже если выше лежит фото -->
      {#if tg.selected('caption', c.photo)}
        <div class="capclip">
          <div
            class="capsel"
            style:left={pctX(c.x)}
            style:top={pctY(c.y)}
            style:width={pctX(c.w)}
            style:height={pctY(c.h)}
            style:transform={c.rot ? `rotate(${c.rot}deg)` : undefined}
          ></div>
        </div>
      {/if}
      {#if c.overflow > 0}
        <!-- Угол повёрнутой подписи: метка стоит на нём, а не на углу неповёрнутой рамки -->
        {@const tr = toSpread(boxOf(c), c.w, 0)}
        <span
          class="warn"
          role="img"
          aria-label="Подпись не помещается"
          title={warn(c)}
          data-text-ui
          style:left={pctX(tr.x)}
          style:top={pctY(tr.y)}>!</span
        >
      {/if}
    {/if}
  {/each}

  {#each spread.texts as t (t.id)}
    {#if tg.editing('text', t.id)}
      <TextEditor box={boxOf(t)} auto={t.auto} maxW={store.view.textMaxW} text={editText(t)} style={t.style} {sw} {ph} {k} />
    {:else if tg.selected('text', t.id)}
      {@const e = tg.eff(t)}
      <div
        class="frame"
        style:left={pctX(e.x)}
        style:top={pctY(e.y)}
        style:width={pctX(e.w)}
        style:height={pctY(e.h)}
        style:transform={e.rot ? `rotate(${e.rot}deg)` : undefined}
        data-through
        role="presentation"
        onclick={(ev) => ev.stopPropagation()}
      >
        <TextHandles
          auto={t.auto}
          rotate={prefs.can('textRotate')}
          scale={prefs.can('textRotate')}
          onrotate={(ev) => tg.rotate(ev, t)}
          onresize={(ev, side) => tg.start(ev, t, side)}
          onauto={() => edit.setAuto(t.id)}
          onscale={(ev, corner) => tg.scale(ev, t, corner)}
        />
      </div>
    {/if}
  {/each}

  {#if gesture?.label}
    <div class="angle" style:left="{gesture.label.x + 14}px" style:top="{gesture.label.y + 14}px">{gesture.label.text}</div>
  {/if}
  {#if gesture?.guideX != null}
    <div class="guide v" style:left={pctX(gesture.guideX)}></div>
  {/if}
  {#if gesture?.guideY != null}
    <div class="guide h" style:top={pctY(gesture.guideY)}></div>
  {/if}
</div>

<style>
  /* Выше стека `.content` (3) и притемнения (5), вровень с рамкой выделения; мышь ловят только ручки и редактор. */
  .tover {
    position: absolute;
    inset: 0;
    z-index: 7;
    pointer-events: none;
  }
  /* Обводка выделенного блока и носитель ручек: тело блока мышь не ловит (его берёт сам блок в стеке). */
  .frame {
    position: absolute;
    transform-origin: 0 0; /* поворот вокруг угла x, y, как в Go */
    outline: 1.5px solid var(--accent);
    outline-offset: 2px;
    pointer-events: none;
  }
  /* Обводка подписи режется по краю листа, как и сама подпись в `.content`; запас 4 px, чтобы обводка у края была видна.
     Рамку текста `.frame` не режем: у неё ручки, их нужно достать и у края. */
  .capclip {
    position: absolute;
    inset: 0;
    overflow: clip;
    overflow-clip-margin: 4px;
    pointer-events: none;
  }
  .capsel {
    position: absolute;
    transform-origin: 0 0; /* поворот вокруг угла x, y, как в Go */
    outline: 1.5px solid var(--accent);
    outline-offset: 2px;
    pointer-events: none;
  }
  /* Центр метки — правый верхний угол рамки подписи */
  .warn {
    position: absolute;
    width: 18px;
    height: 18px;
    margin: -9px 0 0 -9px;
    border-radius: 50%;
    background: #e0803a;
    color: #1e1e1e;
    font-size: 12px;
    font-weight: 700;
    line-height: 18px;
    text-align: center;
    cursor: help;
    pointer-events: auto;
  }
  .angle {
    position: absolute;
    padding: 2px 6px;
    border-radius: 4px;
    background: #1b1b1d;
    color: #fff;
    font-size: 12px;
    pointer-events: none;
    z-index: 3;
  }
  /* Голубая направляющая прилипания. */
  .guide {
    position: absolute;
    background: #4aa3ff;
    pointer-events: none;
    z-index: 3;
  }
  .guide.v {
    top: 0;
    bottom: 0;
    width: 1px;
  }
  .guide.h {
    left: 0;
    right: 0;
    height: 1px;
  }
</style>
