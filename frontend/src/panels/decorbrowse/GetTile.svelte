<script lang="ts">
  import TplItem from '../../base/tpl/TplItem.svelte'
  import { primaryPress } from '../../base/core/platform'
  import { getName, getNote, getRatio, getTitle, INVITE_LABEL, INVITE_TIP, sampleUrl, type GetItem, type GetKind } from './decorget'

  // Плитка «Можно скачать» (144) окон «Фоны», «Стикеры», «Рамочки», «Развороты» и «Макеты альбома»: образец из набора,
  // которого нет в библиотеке. Выглядит как стоящая плитка (TplItem, картинка), но рамка пунктиром — «чего ещё нет».
  // Без `it` — приглашение «⤓ Ещё из интернета» в конце «Все»: без картинки, щелчок открывает окно «Наборы».
  // Серая (off) — пока идёт скачивание или библиотека только для чтения: подсказка с причиной остаётся (aria-disabled, не disabled).
  // Правый щелчок ничего не делает: крупного просмотра у образца нет (клетка 160–240 px).
  let {
    it,
    kind,
    box,
    off = '',
    onget,
    oninvite,
  }: {
    it?: GetItem
    kind: GetKind
    box?: number // пропорция стоящей плитки вида (ширина/высота); нет — клетка образца
    off?: string // причина серого; пусто — плитка живая
    onget?: (it: GetItem) => void
    oninvite?: () => void
  } = $props()

  const g = $derived(it?.get)
  const src = $derived(g ? sampleUrl(g.url) : '')
  const title = $derived(off || (g ? getTitle(g, kind) : INVITE_TIP))
  const press = (e: MouseEvent) => {
    if (!primaryPress(e) || off) return
    if (it) onget?.(it)
    else oninvite?.()
  }
</script>

<TplItem name={g ? getName(g) : INVITE_LABEL} note={g ? getNote(g) : undefined}>
  <button
    class="tile"
    class:off={!!off}
    class:fit={kind !== 'bg'}
    class:invite={!g}
    aria-disabled={off ? 'true' : undefined}
    style:aspect-ratio={g ? getRatio(g, kind, box) : '3 / 2'}
    {title}
    data-get-id={it?.id ?? 'invite'}
    onclick={press}
    oncontextmenu={(e) => e.preventDefault()}
  >
    {#if src}<img {src} alt="" loading="lazy" draggable="false" />{/if}
  </button>
</TplItem>

<style>
  .tile {
    display: block;
    position: relative;
    width: var(--tile, 168px);
    height: auto;
    padding: 0;
    overflow: hidden;
    border: 1px dashed var(--muted);
    border-radius: 4px;
    background: var(--panel2);
  }
  .tile:hover:not(.off) {
    border-color: var(--accent);
  }
  .tile.off {
    cursor: default;
    opacity: 0.45;
  }
  /* Приглашение: без картинки, пустая пунктирная плитка со словом в подписи */
  .tile.invite {
    background: transparent;
  }
  .tile img {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    object-fit: cover;
    pointer-events: none;
  }
  /* Не фон: рисунок вписан целиком, как у стоящих плиток (шашечку под стикер рисует издатель образца) */
  .tile.fit img {
    object-fit: contain;
  }
</style>
