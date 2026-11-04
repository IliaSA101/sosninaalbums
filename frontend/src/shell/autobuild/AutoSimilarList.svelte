<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import { thumbUrl } from '../../base/core/media'
  import TplItem from '../../base/tpl/TplItem.svelte'
  import TplRow from '../../base/tpl/TplRow.svelte'
  import type { PhotosState } from './autosimilar.svelte'
  import { dropTitle, keptOf } from './autosimilar'

  // Ряд отброшенных как похожие (078-6): плитка — кадр (приглушён) с уголком «на что похож»; щелчок или Enter — «Вернуть»,
  // повторно — снова убрать. Устройство как у «Что соберётся» в блоке «Раскладки»: TplRow + TplItem.
  let { ph }: { ph: PhotosState } = $props()

  const drops = $derived(ph.drops ?? [])
  const back = $derived(keptOf(drops, ph.keep))
  const nameOf = (id: string) => store.byId.get(id)?.name ?? id
  const empty = $derived(ph.dropsError ? 'Не удалось показать кадры' : ph.drops === null ? '' : 'Нет кадров')
</script>

<div class="drops">
  <div class="head">
    <span>{drops.length > 0 ? `Убираю: ${drops.length - back}, вернул: ${back}` : ''}</span>
    <button type="button" class="link" disabled={drops.length === 0 || back === drops.length} onclick={() => ph.keepAll()}
      >Вернуть все</button
    >
  </div>
  <TplRow count={drops.length} {empty} dim={ph.dropsBusy && drops.length > 0} watch={ph.similar}>
    {#each drops as d (d.id)}
      {@const on = ph.keep.has(d.id)}
      <TplItem name={nameOf(d.id)} note={on ? 'вернётся' : undefined}>
        <button
          type="button"
          class="tile"
          class:back={on}
          aria-pressed={on}
          title={dropTitle(nameOf(d.id), nameOf(d.like), on)}
          onclick={() => ph.toggleKeep(d.id)}
        >
          <img src={thumbUrl(d.id)} alt="" draggable="false" />
          <img class="like" src={thumbUrl(d.like)} alt="" draggable="false" />
        </button>
      </TplItem>
    {/each}
  </TplRow>
</div>

<style>
  .drops {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    min-height: 20px;
    color: var(--muted);
    font-size: 12px;
  }
  .link {
    padding: 0 4px;
    background: transparent;
    border: 0;
    color: var(--accent);
    font-size: 12px;
    cursor: pointer;
  }
  .link:disabled {
    color: var(--muted);
    cursor: default;
  }
  .tile {
    position: relative;
    display: block;
    width: 80px;
    height: 80px;
    padding: 0;
    overflow: hidden;
    background: var(--panel2);
    border: 1px solid var(--line);
    border-radius: 4px;
    cursor: pointer;
  }
  .tile img {
    width: 100%;
    height: 100%;
    object-fit: contain;
    opacity: 0.4; /* убираем: приглушён, как использованные в ленте */
    filter: saturate(0.4);
  }
  .tile.back img {
    opacity: 1;
    filter: none;
  }
  .tile:hover {
    border-color: var(--muted);
  }
  .tile.back {
    border-color: var(--accent);
  }
  /* Уголок «на что похож»: всегда яркий, чтобы его можно было узнать */
  .tile img.like {
    position: absolute;
    right: 0;
    bottom: 0;
    width: 28px;
    height: 28px;
    opacity: 1;
    filter: none;
    background: var(--panel2);
    border: 1px solid var(--line);
    border-radius: 3px 0 0 0;
  }
</style>
