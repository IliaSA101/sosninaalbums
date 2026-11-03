<script lang="ts">
  import { onMount, untrack } from 'svelte'
  import { edit } from './edit.svelte'
  import { editorCss, ptToMm } from './fonts'
  import { editorShift, measureBase } from './baseline'
  import { cleanInput } from './textedit'
  import { editorFrame, type TextBox } from '../../base/geom/textbox'
  import { store } from '../../base/core/store.svelte'
  import type { TextStyle } from '../../base/core/types'

  // Редактор в рамке блока: те же шрифт, кегль, ширина и поворот, что у SVG-строк, межстрочный 1.2.
  // «По тексту» — рамка по содержимому от якоря (см. editorFrame), закреплённая — от угла блока.
  // Текст кладётся один раз при открытии; ответы Go DOM не трогают (курсор не прыгает).
  // Первая база CSS не равна базе Go (asc шрифта): разницу гасят отступы, min-height и translateY (спека 156, baseline.ts).
  let {
    box,
    auto,
    maxW,
    text,
    style,
    lines,
    sw,
    ph,
    k,
  }: {
    box: TextBox
    auto: boolean
    maxW: number
    text: string
    style: TextStyle
    lines: { y: number }[]
    sw: number
    ph: number
    k: number
  } = $props()

  const f = $derived(editorFrame(box, auto, maxW))
  // База первой строки браузера (px) из невидимого зонда; null — пока не мерили или раскладки нет.
  let base = $state<number | null>(null)
  const sh = $derived(editorShift(lines, box, base, ptToMm(style.size) * k, k))
  const transform = $derived(
    [f.rot ? `rotate(${f.rot}deg)` : '', f.shift ? `translateX(${-f.shift * 100}%)` : '', sh.shiftY ? `translateY(${sh.shiftY}px)` : '']
      .filter(Boolean)
      .join(' ') || undefined,
  )
  const px = (v: number) => (sh.minH > 0 ? `${v}px` : undefined)

  let el: HTMLDivElement
  let probe: HTMLDivElement

  const remeasure = () => {
    if (probe) base = measureBase(probe)
  }

  // Замер сразу (шрифт мог ещё не загрузиться — его тянет первая раскладка зонда), затем после загрузки.
  // Зависим от строки-ключа ($derived не будит эффект при том же значении), а не от объекта style:
  // его отдаёт colorLive на каждом кадре цвета.
  const probeKey = $derived(`${style.font}|${style.size}|${style.bold}|${style.italic}|${k}`)
  $effect(() => {
    void probeKey
    untrack(remeasure)
    let dead = false
    void document.fonts?.ready.then(() => {
      if (!dead) remeasure()
    })
    return () => {
      dead = true
    }
  })

  // Смена шрифта во время правки: новое начертание догружается уже после первого замера.
  $effect(() => {
    const fs = document.fonts
    if (!fs) return
    fs.addEventListener('loadingdone', remeasure)
    return () => fs.removeEventListener('loadingdone', remeasure)
  })

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

<!-- Зонд базы: тот же шрифт и кегль, вне редактора; буква x нужна, иначе пустая строка схлопнется в 0 -->
<div bind:this={probe} class="probe" aria-hidden="true" style={editorCss(style, store.fonts, k)}><span class="pb"></span>x</div>

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
  style:padding-top={px(sh.padTop)}
  style:padding-bottom={px(sh.padBottom)}
  style:min-height={px(sh.minH)}
  style={editorCss(style, store.fonts, k)}
  oninput={() => edit.input(cleanInput(el.innerText))}
  onkeydown={keydown}
  onpointerdown={(e) => e.stopPropagation()}
  onclick={(e) => e.stopPropagation()}
></div>

<style>
  .probe {
    position: absolute;
    top: 0;
    left: 0;
    visibility: hidden;
    pointer-events: none;
    white-space: pre;
    margin: 0;
    padding: 0;
  }
  .pb {
    display: inline-block;
    width: 0;
    height: 0;
  }
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
