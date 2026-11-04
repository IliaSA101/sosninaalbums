<script lang="ts">
  import { primaryPress } from '../../base/core/platform'
  import TplItem from '../../base/tpl/TplItem.svelte'
  import { lookUrl } from '../../base/core/media'
  import type { LookItem } from './looktypes'
  import { cardNote, lookKey, lookTileTitle, tileStep } from './looks'

  // Плитка макета (163), образец — DecorTile: лицевая сторона обложки (нет её — разворот 1), под ней имя целиком и
  // «от 26 фото · 7 разворотов»; ✎ и × — у своих. Щелчок и правый щелчок (Enter и пробел — это щелчок кнопки) — крупно,
  // а не «поставить»: макет меняет весь альбом. Ширину задаёт --tile контейнера; px — размер, от которого считается картинка.
  let {
    it,
    px,
    r,
    ph,
    readOnly,
    onpeek,
    onrename,
    onhide,
  }: {
    it: LookItem
    px: number
    r: number // пропорция плитки (tileRatio)
    ph: number // метка «на фото ленты» (looks.ph): меняется с лентой и правкой — картинка перезапрашивается
    readOnly: boolean
    onpeek: (it: LookItem) => void
    onrename: (it: LookItem) => void
    onhide: (it: LookItem) => void
  } = $props()

  const i = $derived(tileStep(it))
  const src = $derived(lookUrl(lookKey(it.id), { i, px, r, front: i === 0, ph }))
  // Ctrl+щелчок на Mac — правый щелчок: крупно он откроет один раз, через contextmenu
  const press = (e: MouseEvent) => primaryPress(e) && onpeek(it)
  function ctx(e: MouseEvent) {
    e.preventDefault()
    onpeek(it)
  }
</script>

<TplItem
  stack
  name={it.name}
  note={cardNote(it)}
  onrename={it.own && !readOnly ? () => onrename(it) : undefined}
  ondelete={it.own && !readOnly ? () => onhide(it) : undefined}
  renameLabel="Переименовать макет"
  deleteTitle="Убрать из библиотеки"
  deleteLabel="Убрать макет из библиотеки"
>
  <button class="tile" style:aspect-ratio={r} title={lookTileTitle(it)} data-decor-id={it.id} onclick={press} oncontextmenu={ctx}>
    <img src={src || undefined} alt="" loading="lazy" draggable="false" />
  </button>
</TplItem>

<style>
  .tile {
    display: block;
    position: relative;
    width: var(--tile);
    height: auto;
    padding: 0;
    overflow: hidden;
    border: 1px solid var(--line);
    border-radius: 4px;
    background: var(--panel2);
  }
  .tile:hover:not(:disabled) {
    border-color: var(--accent);
  }
  .tile img {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    object-fit: contain;
    pointer-events: none;
  }
</style>
