<script lang="ts">
  import { screenDpr } from '../../base/ui/dpr.svelte'
  import { seenIf } from '../../base/ui/seen'
  import { store } from '../../base/core/store.svelte'
  import TplItem from '../../base/tpl/TplItem.svelte'
  import { lookUrl } from '../../base/core/media'
  import { ALL, themeItems, tilePx } from '../../panels/decor/decor'
  import { isGetItem } from '../../panels/decorbrowse/decorget'
  import { getOffHint } from '../../panels/decorbrowse/getoff'
  import GetTile from '../../panels/decorbrowse/GetTile.svelte'
  import { packsUi } from '../packs/packs.svelte'
  import { cardNote, lookKey, tileRatio, tileStep, EMPTY_MINE, EMPTY_THEME } from './looks'
  import { looks } from './looks.svelte'

  // Плитки макетов темы (образец — DecorDialog): лицевая сторона обложки (нет её — разворот 1), имя, «от 26 фото · 7 разворотов».
  // Щелчок показывает макет справа, а не ставит его; ✎ и × — у своих.
  const TILE_W = 168 // CSS px, как у плиток «Развороты»
  const items = $derived(themeItems(looks.cat, looks.theme))
  const px = $derived(Math.min(1024, tilePx(TILE_W, screenDpr.value)))
  const readOnly = $derived(!!looks.cat?.readOnly)
  const cover = $derived(store.cover?.sheet)
  // 144: плитки «Можно скачать» серые, пока идёт скачивание и в библиотеке «только чтение»
  const getOff = $derived(getOffHint(packsUi.net, readOnly))
  const getBox = $derived(tileRatio(store.view.settings, { cover: false })) // как у стоящей плитки разворота
</script>

<div class="tiles" aria-busy={looks.loading}>
  {#each items as it (it.id)}
    {#if isGetItem(it)}
      <GetTile {it} kind="album" box={getBox} off={getOff} onget={(t) => looks.askGet(t)} />
    {:else}
      {@const i = tileStep(it)}
      {@const r = tileRatio(store.view.settings, it, cover)}
      <TplItem
        name={it.name}
        note={cardNote(it)}
        onrename={it.own && !readOnly ? () => looks.askRename(it) : undefined}
        ondelete={it.own && !readOnly ? () => looks.askHide(it) : undefined}
        renameLabel="Переименовать макет"
        deleteTitle="Убрать из библиотеки"
        deleteLabel="Убрать макет из библиотеки"
      >
        <button
          class="tile"
          class:current={it.id === looks.sel}
          aria-pressed={it.id === looks.sel}
          use:seenIf={it.id === looks.sel}
          style:aspect-ratio={r}
          title={[it.name, cardNote(it)].filter(Boolean).join(' · ')}
          onclick={() => looks.choose(it.id)}
        >
          <img src={lookUrl(lookKey(it.id), { i, px, r, front: i === 0 }) || undefined} alt="" loading="lazy" draggable="false" />
        </button>
      </TplItem>
    {/if}
  {/each}
  {#if looks.theme === ALL && looks.cat?.invite}
    <!-- 144: списка наборов ещё не было — приглашение в конце «Все» -->
    <GetTile kind="album" oninvite={() => packsUi.show()} />
  {/if}
  {#if looks.cat && items.length === 0}
    <div class="empty">{looks.theme === 'mine' ? EMPTY_MINE : EMPTY_THEME}</div>
  {/if}
</div>

<style>
  .tiles {
    flex: 1;
    min-height: 0;
    display: grid;
    grid-template-columns: repeat(auto-fill, 168px);
    align-content: start;
    gap: 12px 10px;
    padding: 5px 6px 6px 5px; /* место под рамку выбранной плитки */
    overflow-y: auto;
  }
  .tile {
    display: block;
    position: relative;
    width: 168px;
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
  /* Выбранный макет — рамка, как у стоящей плитки в «Развороты» */
  .tile.current {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }
  .tile img {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    object-fit: contain;
    pointer-events: none;
  }
  .empty {
    grid-column: 1 / -1;
    padding: 30px 0 0 8px;
    color: var(--muted);
    font-size: 13px;
  }
</style>
