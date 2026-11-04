<script lang="ts">
  import type { BackdropScope, SpreadView } from '../../base/core/types'
  import TplItem from '../../base/tpl/TplItem.svelte'
  import { seenIf } from '../../base/ui/seen'
  import { decor } from '../decor/decor.svelte'
  import { tilePx, type DecorItem, type DecorKind } from '../decor/decor'
  import { tileLook, tplNote, tplSrc, tplTitle } from '../decor/spreadtpl'
  import { primaryPress } from '../../base/core/platform'
  import { tileUrl } from './tilesize'
  import { PEEK_TIP } from './peekaddr'

  // Плитка окон «Фоны», «Стикеры», «Рамочки», «Развороты» (вынесена из DecorDialog в 133): щелчок ставит, ✎ и × — у своих.
  // Ширину задаёт --tile контейнера; hold — размер, от которого считается px картинки (не меньше прежнего за сеанс окна).
  let {
    it,
    kind,
    sp,
    scope,
    current,
    readOnly,
    noun,
    hold,
    dpr,
    onpeek,
    label,
  }: {
    it: DecorItem
    kind: DecorKind
    sp: SpreadView
    scope: BackdropScope
    current: boolean // плитка, что стоит в области сейчас
    readOnly: boolean
    noun: string
    hold: number
    dpr: number
    onpeek: (it: DecorItem) => void // правый щелчок (и Shift+F10 на плитке) — крупный просмотр (вариант Б вопроса 2)
    label?: string // подпись под плиткой; по умолчанию имя (при выбранном стиле — без приставки, browse.tileLabel)
  } = $props()

  const sticker = $derived(kind === 'sticker')
  const frame = $derived(kind === 'frame')
  const spreads = $derived(kind === 'spread') // 067: лист одной пропорции на всё окно
  const look = $derived(spreads ? tileLook(it, sp, scope, hold, dpr) : null)
  const px = $derived(tilePx(hold, dpr))

  const title = $derived(`${tplTitle(it)}\n${PEEK_TIP}`)
  function ctx(e: MouseEvent) {
    e.preventDefault()
    onpeek(it)
  }
  // Ctrl+щелчок на Mac — это правый щелчок: постановкой он быть не должен.
  const put = (e: MouseEvent) => primaryPress(e) && decor.pick(it)
</script>

<TplItem
  name={label ?? it.name}
  note={spreads ? tplNote(it) : undefined}
  onrename={it.own && !readOnly ? () => decor.askRename(it) : undefined}
  ondelete={it.own && !readOnly ? () => decor.askHide(it) : undefined}
  renameLabel="Переименовать {noun}"
  deleteTitle="Убрать из библиотеки"
  deleteLabel="Убрать {noun} из библиотеки"
>
  {#if look}
    <!-- 067: лист одной пропорции на всё окно; страница в «Весь разворот» стоит по центру на своей -->
    <button
      class="tile tpl"
      style:aspect-ratio={look.box}
      {title}
      data-decor-id={it.id}
      disabled={decor.busy}
      onclick={put}
      oncontextmenu={ctx}
    >
      <img src={tplSrc(it.url, look.px, look.r)} alt="" loading="lazy" draggable="false" />
    </button>
  {:else}
    <button
      class="tile"
      class:current
      class:stk={sticker}
      class:stkdark={sticker && it.dark}
      class:frm={frame}
      class:chk={frame && it.own}
      aria-current={current ? 'true' : undefined}
      use:seenIf={current}
      style:aspect-ratio={sticker || frame ? '1 / 1' : `${sp.w} / ${sp.h}`}
      style:background={it.color || undefined}
      {title}
      data-decor-id={it.id}
      onclick={put}
      oncontextmenu={ctx}
    >
      <img src={tileUrl(it.url, px)} alt="" loading="lazy" draggable="false" />
    </button>
  {/if}
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
  /* Стоящий сейчас — рамка, как у текущей раскладки в галерее ⊞ */
  .tile.current {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }
  .tile img {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    object-fit: cover;
    pointer-events: none;
  }
  /* Готовый разворот: картинка вписана в кнопку (страница уже листа — по краям подложка).
     Рамочка: квадратная плитка, рамка вокруг силуэта вписана целиком */
  .tile.tpl img,
  .tile.frm img {
    object-fit: contain;
  }
  /* Стикер: квадратная плитка, рисунок вписан целиком, под ним шашечка (видно прозрачность);
     у рамочки шашечка — только у своих */
  .tile.stk,
  .tile.chk {
    background: repeating-conic-gradient(#d4d4d8 0 25%, #f4f4f5 0 50%) 0 0 / 14px 14px;
  }
  /* Светлый стикер (116, Go: dark): тот же рисунок, тёмные тона темы */
  .tile.stk.stkdark {
    background: repeating-conic-gradient(var(--line) 0 25%, var(--panel2) 0 50%) 0 0 / 14px 14px;
  }
  .tile.stk img {
    object-fit: contain;
    inset: 6px;
    width: calc(100% - 12px);
    height: calc(100% - 12px);
  }
</style>
