<script lang="ts">
  import type { BackdropScope, SpreadView } from '../../base/core/types'
  import { drag } from '../../base/drag/drag.svelte'
  import { store } from '../../base/core/store.svelte'
  import { backdropAccepts } from '../../base/drag/drop'
  import { backdropOf, SCOPE_LABEL, zoneScopes } from './backdrop'
  import { clearBackdrop, editBackdrop, pickBackdropFile } from './backdropact'
  import { thumbUrl } from '../../base/core/media'
  import { bgmode } from './bgmode.svelte'
  import BackdropFade from './BackdropFade.svelte'
  import BackdropFields from './BackdropFields.svelte'

  // «Фото фона» в разделе «Фон» карточки разворота (спека 009): зона броска на каждую область,
  // «Свой файл…», «Править» (входит в режим фона) и «Убрать». split — цвета показаны по страницам.
  let { sp, split }: { sp: SpreadView; split: boolean } = $props()

  const scopes = $derived(zoneScopes(sp.backdrops, split))
  // Эта область правится в режиме фона: вместо «Править» — поля режима и «Готово».
  const editing = (scope: BackdropScope) => bgmode.cur?.spread === sp.id && bgmode.cur.scope === scope
  const label = (scope: BackdropScope) => (scopes.length > 1 ? SCOPE_LABEL[scope] : 'Фото фона')
</script>

<div class="zones" role="group" aria-label="Фото фона">
  {#each scopes as scope (scope)}
    {@const b = backdropOf(sp, scope)}
    <div
      class="zone"
      class:edit={editing(scope)}
      class:over={drag.over === `backdrop:${scope}` && backdropAccepts(drag.payload)}
      data-drop="backdrop"
      data-spread={sp.id}
      data-scope={scope}
    >
      <div class="cap">{label(scope)}</div>
      {#if b}
        <div class="have">
          <img src={thumbUrl(b.photo)} alt="" draggable="false" />
          <div class="info">
            <span class="name" title={store.byId.get(b.photo)?.name}>{store.byId.get(b.photo)?.name ?? 'Фото'}</span>
            {#if b.dpi > 0}<span class="dpi">{Math.round(b.dpi)} dpi</span>{/if}
          </div>
        </div>
        <BackdropFade {b} spread={sp.id} />
        {#if editing(scope)}
          <BackdropFields {b} spread={sp.id} />
        {:else}
          <div class="btns">
            <button onclick={() => editBackdrop(sp.id, scope)}>Править</button>
            <button class="ghost" onclick={() => clearBackdrop(sp.id, scope)}>Убрать</button>
          </div>
        {/if}
        <button class="ghost file" onclick={() => pickBackdropFile(sp.id, scope)}>Свой файл…</button>
      {:else}
        <div class="hint">Перетащите фото сюда</div>
        <button class="ghost file" onclick={() => pickBackdropFile(sp.id, scope)}>Свой файл…</button>
      {/if}
    </div>
  {/each}
</div>

<style>
  .zones {
    display: flex;
    flex-direction: column;
    gap: 6px;
    margin-top: 10px;
  }
  .zone {
    padding: 6px;
    border: 1px dashed var(--line);
    border-radius: 6px;
    font-size: 12px;
    transition:
      border-color 0.15s,
      background 0.15s;
  }
  .zone.edit {
    border-style: solid;
    border-color: var(--accent);
  }
  .zone.over {
    border-color: var(--accent);
    background: rgba(245, 217, 10, 0.08);
  }
  .cap {
    font-weight: 600;
  }
  .hint {
    margin: 6px 0;
    color: var(--muted);
  }
  .have {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 6px;
  }
  .have img {
    width: 44px;
    height: 44px;
    flex: none;
    object-fit: cover;
    border-radius: 4px;
    background: #333;
  }
  .info {
    min-width: 0;
    display: flex;
    flex-direction: column;
  }
  .name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .dpi {
    color: var(--muted);
  }
  .btns {
    display: flex;
    gap: 6px;
    margin-top: 6px;
  }
  .btns button {
    flex: 1;
    height: 24px;
    padding: 0 6px;
    font-size: 12px;
  }
  .file {
    width: 100%;
    height: 24px;
    margin-top: 6px;
    padding: 0 6px;
    font-size: 12px;
  }
</style>
