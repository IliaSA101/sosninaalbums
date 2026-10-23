<script lang="ts">
  import type { PickResult } from '../../base/core/hooks'
  import { store } from '../../base/core/store.svelte'
  import Seg from '../../base/tpl/Seg.svelte'
  import type { AutoPlan } from '../../base/core/types'
  import AutoSource from './AutoSource.svelte'
  import { ALBUM_MODE, baseLabel, freeLabel } from './autoalbum'

  // Строка «Собрать: Свободные фото: N · Весь альбом заново» (107) и галочка «Добавить свободные фото ленты».
  // Под строкой — источник (AutoSource), под ним галочка. Образец — «Фото на разворот: Авто · Свой». Выбор не запоминается: каждое открытие окна решает AutoRun.open.
  let { d, plan }: { d: PickResult; plan: AutoPlan | null } = $props()

  const album = $derived(d.from === 'album')
  // Что было выбрано до режима альбома (щелчок по первому пункту вернёт его); режим без выбора — только строка источника
  const base = $derived(album ? d.base : d)
  const items = $derived(
    base
      ? [
          { id: 'base', label: baseLabel(base) },
          { id: 'album', label: ALBUM_MODE },
        ]
      : [],
  )
  const show = $derived(!!base && (album || store.view.spreads.length > 0))
  const free = $derived(d.freeIds?.length ?? 0)
</script>

{#if show}
  <div class="mode">
    Собрать:
    <Seg
      {items}
      value={album ? 'album' : 'base'}
      off={store.autoWaiting}
      onpick={(m) => store.autorun.setSource(m === 'album' ? 'album' : 'base')}
      label="Собрать"
    />
  </div>
{/if}
<AutoSource {d} {plan} />
{#if album && !d.again && free > 0}
  <label class="more">
    <input type="checkbox" checked={!!d.addFree} onchange={(e) => store.autorun.setAddFree(e.currentTarget.checked)} />
    {freeLabel(free)}
  </label>
{/if}

<style>
  .mode {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 4px 8px;
  }
  .more {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 13px;
  }
</style>
