<script lang="ts">
  import { eventChord } from '../keys/chord'
  import { nameOf } from './palette'
  import { tipped } from '../ui/tip.svelte'

  // Один образец окна цвета. Наведение и фокус с клавиатуры показывают цвет на развороте, щелчок — выбирает.
  // Свой образец (own): правый щелчок и Delete убирают его из «Своих».
  let {
    hex,
    size,
    on = false,
    own = false,
    photo = false,
    onpick,
    onpreview,
    onforget,
  }: {
    hex: string
    size: number
    on?: boolean
    own?: boolean
    photo?: boolean
    onpick: (hex: string) => void
    onpreview: (hex: string | null) => void
    onforget?: (hex: string) => void
  } = $props()

  // Имя из наборов; у цвета «С фото» имени нет — подсказка называет источник.
  const label = $derived(photo ? `С фото · ${hex}` : `${nameOf(hex)} · ${hex}`)
  const tip = $derived(own ? `${label} · правый щелчок — убрать` : label)

  // Delete читаем через реестр клавиш (сочетание без модификаторов), а не сравнением event.key: страж keys.guard.
  function onkey(e: KeyboardEvent) {
    if (!own || eventChord(e) !== 'Delete') return
    e.preventDefault()
    e.stopPropagation()
    onforget?.(hex)
  }
</script>

<button
  type="button"
  class="chip"
  class:on
  style:--c={hex}
  style:width="{size}px"
  style:height="{size}px"
  aria-label={label}
  use:tipped={{ text: tip }}
  onclick={() => onpick(hex)}
  onpointerenter={() => onpreview(hex)}
  onpointerleave={() => onpreview(null)}
  onfocus={() => onpreview(hex)}
  onblur={() => onpreview(null)}
  oncontextmenu={(e) => {
    if (!own) return
    e.preventDefault()
    onforget?.(hex)
  }}
  onkeydown={onkey}
></button>

<style>
  .chip {
    padding: 0;
    border-radius: 5px;
    border: 1px solid rgba(255, 255, 255, 0.12);
    background: var(--c);
    transition: transform 0.08s;
  }
  .chip:hover:not(:disabled) {
    background: var(--c);
    border-color: var(--text);
    transform: scale(1.08);
  }
  .chip.on {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }
</style>
