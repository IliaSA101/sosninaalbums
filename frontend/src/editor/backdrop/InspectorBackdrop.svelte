<script lang="ts">
  import type { BackdropScope, SpreadView } from '../../base/core/types'
  import { drag } from '../../base/drag/drag.svelte'
  import { store } from '../../base/core/store.svelte'
  import { backdropAccepts } from '../../base/drag/drop'
  import { backdropOf, SCOPE_LABEL, zoneScopes } from './backdrop'
  import { coverLabels, coverScopes } from '../cover/cover'
  import { isCover, unitOf } from '../../base/geom/sheet'
  import { gridAct } from '../../panels/layouts/gridact.svelte'
  import { clearBackdrop, editBackdrop, pickBackdropFile } from './backdropact'
  import { thumbUrl } from '../../base/core/media'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { decor } from '../../panels/decor/decor.svelte'
  import { bgmode } from './bgmode.svelte'
  import { LIB_MISSING_TEXT } from '../../panels/library/missing'
  import BackdropFade from './BackdropFade.svelte'
  import BackdropFields from './BackdropFields.svelte'

  // «Фото фона» в разделе «Фон» карточки разворота (спека 009): зона броска на каждую область,
  // «Галерея фонов…» и «Свой файл…», «Править» (входит в режим фона) и «Убрать». split — цвета показаны по страницам.
  let { sp, split }: { sp: SpreadView; split: boolean } = $props()

  // Обложка (032): зоны броска задаёт её раскладка, а не цвета страниц; названия — сторонки.
  const cover = $derived(isCover(sp))
  const unit = $derived(unitOf(store.view.settings))
  const scopes = $derived(cover ? coverScopes(store.cover) : zoneScopes(sp.backdrops, split))
  const names = $derived(cover ? coverLabels(store.cover) : SCOPE_LABEL)
  // Эта область правится в режиме фона: вместо «Править» — поля режима и «Готово».
  const editing = (scope: BackdropScope) => bgmode.cur?.spread === sp.id && bgmode.cur.scope === scope
  const label = (scope: BackdropScope) => (cover || scopes.length > 1 ? names[scope] : 'Фото фона')

  // Свой из библиотеки (048): имя — из каталога, пропавший файл — подсказка «загрузите тот же».
  const missing = (photo: string) => store.missing.includes(photo)
  const ownName = (lib: string) => decor.nameOf(lib) ?? 'Свой фон'
  $effect(() => {
    if (sp.backdrops?.some((b) => b.lib) || sp.patterns?.length) void decor.ensureNames()
  })
</script>

<div class="zones" role="group" aria-label="Фото фона" data-guide="bg-photo">
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
        {@const lost = !!b.lib && missing(b.photo)}
        <div class="have">
          {#if lost}<span class="ph" aria-hidden="true"></span>{:else}<img src={thumbUrl(b.photo)} alt="" draggable="false" />{/if}
          <div class="info">
            {#if b.lib}
              <span class="name" title={ownName(b.lib)}>{ownName(b.lib)}</span>
            {:else}
              <span class="name" title={store.byId.get(b.photo)?.name}>{store.byId.get(b.photo)?.name ?? 'Фото'}</span>
            {/if}
            {#if b.dpi > 0}<span class="dpi">{Math.round(b.dpi)} dpi</span>{/if}
          </div>
        </div>
        {#if lost}<div class="lost" role="status">{LIB_MISSING_TEXT}</div>{/if}
        <BackdropFade {b} spread={sp.id} />
        {#if editing(scope)}
          <BackdropFields {b} spread={sp.id} />
        {:else}
          <div class="btns">
            <button onclick={() => editBackdrop(sp.id, scope)}>Править</button>
            <button class="ghost" onclick={() => clearBackdrop(sp.id, scope)}>Убрать</button>
          </div>
        {/if}
      {:else}
        <div class="hint">Перетащите фото сюда</div>
      {/if}
      <div class="files">
        {#if prefs.can('decor')}
          <button class="ghost" data-guide="bg-gallery" title="Узоры и фоны из библиотеки" onclick={() => decor.show('bg', sp.id, scope)}
            >Галерея фонов…</button
          >
        {/if}
        <button class="ghost" onclick={() => pickBackdropFile(sp.id, scope)}>Свой файл…</button>
      </div>
    </div>
  {/each}
</div>
{#if prefs.can('decor') && !cover}
  <!-- 049 Ф2: одно окно на лист, а не на область фона; открывается тем же путём, что «Правка ▸ Копировать оформление…» -->
  <button
    class="ghost copy"
    data-guide="copy-decor"
    title="Поставить такое же оформление на другие {unit.many}"
    onclick={() => gridAct.openDecor(sp.id)}>Копировать оформление…</button
  >
  <!-- 067: готовые развороты — места и оформление одним щелчком; окно то же, что «Правка ▸ Готовые развороты…» -->
  <button
    class="ghost copy"
    data-guide="spread-tpl"
    title="Поставить готовый разворот: места для фото и оформление"
    onclick={() => decor.show('spread', sp.id, 'spread')}>Готовые развороты…</button
  >
{/if}

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
    flex-wrap: wrap; /* узкая колонка: кнопки переносятся, а не сжимаются */
    gap: 6px;
    margin-top: 6px;
  }
  .btns button {
    flex: 1 0 auto;
    white-space: nowrap;
    height: 24px;
    padding: 0 6px;
    font-size: 12px;
  }
  .files {
    display: flex;
    flex-wrap: wrap; /* узкая колонка: кнопки переносятся, а не сжимаются */
    gap: 6px;
    margin-top: 6px;
  }
  .files button {
    flex: 1 0 auto;
    white-space: nowrap;
    height: 24px;
    padding: 0 6px;
    font-size: 12px;
  }
  .copy {
    width: 100%;
    height: 24px;
    margin-top: 6px;
    padding: 0 6px;
    font-size: 12px;
  }
  .ph {
    width: 44px;
    height: 44px;
    flex: none;
    border: 1px dashed var(--line);
    border-radius: 4px;
  }
  .lost {
    margin-top: 6px;
    color: var(--accent);
    line-height: 1.35;
  }
</style>
