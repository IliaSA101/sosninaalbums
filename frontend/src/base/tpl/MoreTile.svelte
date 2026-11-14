<script lang="ts">
  import { cardWidth, type CardSheet } from './layoutcard'

  // Служебная плитка в ряду раскладок — «Ещё», «Похожие: N» («Свернуть похожие»): не раскладка, пунктир на фоне панели.
  // Размер как у LayoutCard. Классы `card more` — за них зацепляется прокрутка «Ещё» в VariantGallery.
  let {
    label,
    sheet,
    h,
    title,
    expanded,
    disabled = false,
    onclick,
  }: {
    label: string
    sheet: CardSheet
    h: number
    title?: string
    expanded?: boolean // у «Похожих» — раскрыты ли
    disabled?: boolean
    onclick: (e: MouseEvent) => void
  } = $props()
</script>

<button class="card more" style:width="{cardWidth(h, sheet)}px" style:height="{h}px" {title} aria-expanded={expanded} {disabled} {onclick}
  >{label}</button
>

<style>
  button {
    all: unset;
    flex: none;
    cursor: pointer;
    display: grid;
    place-items: center;
    background: transparent;
    outline: 1px dashed var(--line);
    outline-offset: 0;
    color: var(--muted);
    font-size: 13px;
    text-align: center;
  }
  button:hover:not(:disabled) {
    outline-color: #8a8a90;
    color: var(--text);
  }
  button:disabled {
    cursor: progress;
  }
</style>
