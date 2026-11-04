<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import type { SpreadView } from '../../base/core/types'
  import { isCover } from '../../base/geom/sheet'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { namesUi } from '../../panels/names/names.svelte'
  import CaptionSheet from '../captions/CaptionSheet.svelte'
  import { unitWords } from '../captions/unitword'
  import CardGroup from './CardGroup.svelte'
  import { capsKind, capsSum } from './groupsum'

  // Группа «Подписи» листа (030, спека 124): общий стиль и шаблон «к каждому фото»; когда выделять нечего
  // (одни заглушки) — здесь. Общая для карточки листа и обложки (id — spread.caps / cover.caps, память раздельная).
  // Есть, когда включён общий стиль, есть подписи или есть фото, которым их добавить.
  let { sp }: { sp: SpreadView } = $props()
  const cw = $derived(unitWords(store.view.settings, sp))
  const shown = $derived(prefs.can('captions') && (!!sp.caps || sp.captions.length > 0 || sp.photos.length > 0))
  const text = $derived(capsSum(capsKind({ styled: !!sp.caps, template: !!sp.caps?.text, own: sp.captions.length })))
</script>

{#if shown}
  <CardGroup id={isCover(sp) ? 'cover.caps' : 'spread.caps'} title="Подписи {cw.gen}" summary={text}>
    <div role="group" aria-label="Подписи {cw.gen}" data-guide="sheet-caps">
      <CaptionSheet {sp} />
      <button
        class="ghost"
        data-guide="names-btn"
        title="Вставить список фамилий и имён по порядку фото {cw.gen}"
        onclick={() => namesUi.show(sp.id)}>Имена из списка…</button
      >
    </div>
  </CardGroup>
{/if}

<style>
  .ghost {
    width: 100%;
    height: 24px;
    margin-top: 8px;
    padding: 0 6px;
    font-size: 12px;
  }
</style>
