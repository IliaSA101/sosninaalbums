<script lang="ts">
  import type { BackdropScope } from '../../base/core/types'
  import { store } from '../../base/core/store.svelte'
  import { isSheet, unitOf } from '../../base/geom/sheet'
  import Seg from '../../base/tpl/Seg.svelte'
  import type { SegItem } from '../../base/tpl/Seg.svelte'
  import { packsUi } from '../../shell/packs/packs.svelte'
  import { decor, type DecorCur } from './decor.svelte'
  import { albumWhere, type AlbumWhere } from '../albumbg/albumbg'
  import { QR_OPEN, QR_OPEN_TIP } from './qr'
  import { qrForm } from './qr.svelte'
  import { spreadSave } from './tplsave.svelte'
  import { frameWhereItems } from './frames'
  import { ADD_OWN, NOUN, READ_ONLY_HINT, ownKind, uploadHint } from './decor'
  import { setTile, tileOf, tileRange } from '../decorbrowse/tilesize'

  // Шапка окна «Фоны»: «Куда» (рамочка / фон альбома / область листа), размер миниатюр (133), «Наборы…», «Сохранить разворот…», QR и «Добавить своё…» (100-1).
  // Область (where, scope) и признак «фон альбома» считает окно: они нужны и плиткам.
  let { c, where, scope, album }: { c: DecorCur; where: SegItem<BackdropScope>[]; scope: BackdropScope; album: boolean } = $props()

  const sticker = $derived(c.kind === 'sticker')
  const frame = $derived(c.kind === 'frame') // 079: «Куда» — выделенные места или весь лист, а не область листа
  const spreads = $derived(c.kind === 'spread') // 067: плитки готовых разворотов, «Куда» отбирает их
  const readOnly = $derived(!!decor.cat?.readOnly)
  const noun = $derived(NOUN[c.kind])
  const albumPlaces = $derived(albumWhere(isSheet(store.view.settings), unitOf(store.view.settings).all))
  const frameWhere = $derived(frameWhereItems(decor.frameSel().length))
  const range = $derived(tileRange(c.kind))
</script>

<div class="bar">
  {#if frame}
    <!-- 079: без выделения пункта «Выделенные» нет (owner-ux 2.7) -->
    <span class="lbl">Куда</span>
    <Seg items={frameWhere} value={decor.frameWhereNow()} onpick={(w) => decor.setFrameWhere(w)} label="Куда поставить рамочку" />
  {:else if album}
    <!-- 098: фон альбома — на весь разворот или на каждую страницу; у листа без шва пункт один -->
    {#if albumPlaces.length > 1}
      <span class="lbl">Куда</span>
      <Seg
        items={albumPlaces}
        value={decor.albumPages ? 'pages' : 'spread'}
        onpick={(w: AlbumWhere) => decor.setAlbumPages(w === 'pages')}
        label="Куда поставить фон альбома"
      />
    {/if}
  {:else if (c.kind === 'bg' || spreads) && where.length > 1}
    <span class="lbl">Куда</span>
    <Seg items={where} value={scope} onpick={(s) => decor.setScope(s)} label="Куда поставить {noun}" />
  {/if}
  <span class="grow"></span>
  <!-- 133: ползунок и слова — те же, что у ленты фото (053); Ctrl+колесо над плитками делает то же -->
  <input
    class="size"
    type="range"
    min={range.lo}
    max={range.hi}
    value={tileOf(c.kind)}
    oninput={(e) => setTile(c.kind, Number(e.currentTarget.value))}
    title="Размер миниатюр"
    aria-label="Размер миниатюр"
    onchange={(e) => e.currentTarget.blur()}
  />
  <!-- 067 Ф2: те же наборы во всех трёх окнах; окно «Наборы» открывается поверх -->
  <button class="ghost" title="Наборы библиотеки: добавить из файла, удалить" onclick={() => packsUi.show()}>Наборы…</button>
  {#if spreads}
    <button
      class="ghost"
      disabled={decor.busy || readOnly}
      title={readOnly ? READ_ONLY_HINT : 'Сохранить места и оформление этого разворота в «Мои»'}
      onclick={() => spreadSave.show()}>Сохранить разворот…</button
    >
  {:else if ownKind(c.kind)}
    <!-- 100: код по ссылке — только когда стикер добавляют, а не заменяют -->
    {#if sticker && !decor.replacing()}
      <button class="ghost" disabled={decor.busy || readOnly} title={readOnly ? READ_ONLY_HINT : QR_OPEN_TIP} onclick={() => qrForm.show()}
        >{QR_OPEN}</button
      >
    {/if}
    <!-- 100-1: своё на виду — primary, крайняя справа, одна кнопка на три окна -->
    <button
      class="primary"
      disabled={decor.busy || readOnly}
      title={readOnly ? READ_ONLY_HINT : uploadHint(c.kind)}
      onclick={() => decor.upload()}>{ADD_OWN[c.kind]}</button
    >
  {/if}
</div>

<style>
  .bar {
    flex: none;
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 30px;
  }
  .lbl {
    color: var(--muted);
    font-size: 12px;
  }
  .grow {
    flex: 1;
  }
  .size {
    width: 72px;
  }
</style>
