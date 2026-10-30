<script lang="ts">
  import { onMount } from 'svelte'
  import { edit } from './edit.svelte'
  import { editorCss } from './fonts'
  import { cleanInput } from './textedit'
  import { editorFrame, type TextBox } from '../../base/geom/textbox'
  import { store } from '../../base/core/store.svelte'
  import type { TextStyle } from '../../base/core/types'

  // Редактор в рамке блока: те же шрифт, кегль, ширина и поворот, что у SVG-строк, межстрочный 1.2.
  // «По тексту» — рамка по содержимому от якоря (см. editorFrame), закреплённая — от угла блока.
  // Текст кладётся один раз при открытии; ответы Go DOM не трогают (курсор не прыгает).
  let {
    box,
    auto,
    maxW,
    text,
    style,
    sw,
    ph,
    k,
  }: { box: TextBox; auto: boolean; maxW: number; text: string; style: TextStyle; sw: number; ph: number; k: number } = $props()

  const f = $derived(editorFrame(box, auto, maxW))
  const transform = $derived(
    [f.rot ? `rotate(${f.rot}deg)` : '', f.shift ? `translateX(${-f.shift * 100}%)` : ''].filter(Boolean).join(' ') || undefined,
  )

  let el: HTMLDivElement

  onMount(() => {
    // Намеренно мимо Svelte: текст кладём один раз, дальше DOM ведёт браузер (иначе курсор прыгает).
    // eslint-disable-next-line svelte/no-dom-manipulating
    el.replaceChildren(document.createTextNode(text))
    el.focus()
    const sel = getSelection()
    if (!sel) return
    const r = document.createRange()
    r.selectNodeContents(el)
    if (!edit.selectAll) r.collapse(false) // курсор в конец
    sel.removeAllRanges()
    sel.addRange(r)
  })

  // Esc: ввод закрыт, ответ Go ещё идёт — снимаем фокус, чтобы горячие клавиши (Ctrl+Z) работали.
  $effect(() => {
    if (edit.closing) el.blur()
  })

  function keydown(e: KeyboardEvent) {
    if (e.key === 'Escape' || (e.key === 'Enter' && (e.ctrlKey || e.metaKey))) {
      e.preventDefault()
      e.stopPropagation()
      edit.finish()
    }
  }
</script>

<!-- plaintext-only: вставка простым текстом, разметка невозможна (WebView2 — Chromium) -->
<div
  bind:this={el}
  class="editor"
  data-text-ui
  role="textbox"
  tabindex="0"
  aria-multiline="true"
  aria-label="Текст на развороте"
  contenteditable={edit.closing ? 'false' : 'plaintext-only'}
  spellcheck="false"
  class:auto={f.w === null}
  style:left="{(f.x / sw) * 100}%"
  style:top="{(f.y / ph) * 100}%"
  style:width={f.w === null ? 'max-content' : `${(f.w / sw) * 100}%`}
  style:max-width={f.w === null ? `${f.maxW * k}px` : undefined}
  style:transform
  style={editorCss(style, store.fonts, k)}
  oninput={() => edit.input(cleanInput(el.innerText))}
  onkeydown={keydown}
  onpointerdown={(e) => e.stopPropagation()}
  onclick={(e) => e.stopPropagation()}
></div>

<style>
  .editor {
    position: absolute;
    min-height: 1.2em;
    transform-origin: 0 0; /* поворот вокруг угла (или якоря), как в Go */
    margin: 0;
    padding: 0;
    outline: 1.5px solid var(--accent);
    outline-offset: 2px;
    background: rgba(255, 255, 255, 0.35);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    font-synthesis: none;
    font-variant-ligatures: none;
    font-feature-settings:
      'liga' 0,
      'clig' 0,
      'calt' 0;
    user-select: text;
    cursor: text;
    pointer-events: auto;
    z-index: 2;
  }
  /* Пустой блок «по тексту» не схлопывается в ноль: курсор должен быть виден. */
  .editor.auto {
    min-width: 0.3em;
  }
</style>
