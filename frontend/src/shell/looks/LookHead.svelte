<script lang="ts">
  import { READ_ONLY_HINT } from '../../panels/decor/decor'
  import TileSize from '../../panels/decorbrowse/TileSize.svelte'
  import { packsUi } from '../packs/packs.svelte'
  import { SAVE_LOOK_TIP } from './looks'
  import { looks } from './looks.svelte'
  import { saveLookUi } from './savelook.svelte'

  // Строка над отбором окна «Макеты альбома» (163), образец — DecorHead «Развороты»: размер миниатюр, «Наборы…» и
  // «Сохранить альбом как макет…» (пункт «Файл ▸» остаётся). Если сохранять нечего (нет разворотов), скажет сам вызов тостом.
  const readOnly = $derived(!!looks.cat?.readOnly)
</script>

<div class="bar">
  <span class="grow"></span>
  <TileSize kind="album" />
  <!-- 067 Ф2: те же наборы, что в окнах «Фоны», «Развороты»; окно «Наборы» открывается поверх -->
  <button class="ghost" title="Наборы библиотеки: добавить из файла, удалить" onclick={() => packsUi.show()}>Наборы…</button>
  <button class="ghost" disabled={readOnly} title={readOnly ? READ_ONLY_HINT : SAVE_LOOK_TIP} onclick={() => void saveLookUi.show()}
    >Сохранить альбом как макет…</button
  >
</div>

<style>
  .bar {
    flex: none;
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 30px;
  }
  .grow {
    flex: 1;
  }
</style>
