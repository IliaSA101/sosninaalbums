<script lang="ts">
  import { Api, store } from '../../base/core/store.svelte'
  import { thumbUrl } from '../../base/core/media'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { bdSession } from '../../editor/backdrop/bgmode'
  import { fadePct, pctFade } from '../../editor/backdrop/fade'
  import { ALBUM_FADE, albumFadeLive } from '../../editor/backdrop/fade.svelte'
  import { decor } from '../decor/decor.svelte'
  import { tileSrc } from '../decor/decor'
  import { ALBUM_BG, ALBUM_CLEAR, ALBUM_FADE_CAP, ALBUM_PICK, albumItem, albumName } from '../decor/albumbg'

  // Строка «Фон альбома» в «Настройках альбома» (098 §3): образец плитки, «Выбрать…» / «Убрать», ползунок прозрачности.
  // Как «Цвет страниц альбома» рядом, но действует сразу и не ждёт «Применить»: фон — не поле Settings, у него свой шаг отмены.
  const ab = $derived(store.view.albumBg)
  const items = $derived(decor.bgItems)
  const tile = $derived(ab ? albumItem(ab, items) : undefined)
  // Образец: плитка каталога, иначе миниатюра самого фото (ключа библиотеки уже нет)
  const sample = $derived(tile ? tileSrc(tile.url, 96) : ab?.photo ? thumbUrl(ab.photo) : '')
  const name = $derived(ab ? albumName(ab, items) : '')
  // Без уровня «Фоны» выбрать нельзя, но уже стоящий можно убрать
  const canPick = $derived(prefs.can('decor'))

  $effect(() => {
    if (ab) void decor.ensureNames() // имя и плитка узора — из каталога «фоны», он читается один раз
  })

  const pct = $derived(fadePct(albumFadeLive.of(ALBUM_FADE, ALBUM_FADE, ab?.fade ?? 0)))
  const val = (e: Event) => +(e.currentTarget as HTMLInputElement).value

  // Ползунок как у фото фона (017): просмотр на oninput, один вызов Go на отпускание; стрелки в одном фокусе — одна сессия merge.
  let session = ''
  function commit(e: Event) {
    const f = pctFade(val(e))
    const merge = session
    return albumFadeLive.commit(() => store.run(Api.SetAlbumBgFade(f, merge)))
  }
  // Chromium не шлёт change, если значение при отпускании равно исходному: тогда просмотр залип бы.
  function blur() {
    session = ''
    if (pct === fadePct(ab?.fade ?? 0)) albumFadeLive.release()
  }
  // Фон убрали: живой просмотр не должен остаться (зависим от наличия, а не от объекта: сессия стрелок живёт между ответами Go)
  const has = $derived(!!ab)
  $effect(() => {
    if (!has) albumFadeLive.clear()
  })
</script>

<div class="albumbg" role="group" aria-label={ALBUM_BG} data-guide="album-bg">
  <div class="line">
    <span class="cap">{ALBUM_BG}</span>
    <span class="act">
      {#if ab}
        <span class="sw" title={name} style:background-image={sample ? `url("${sample}")` : undefined}></span>
      {:else}
        <span class="sw empty" aria-hidden="true"></span>
      {/if}
      {#if canPick}<button type="button" onclick={() => decor.showAlbum()}>{ALBUM_PICK}</button>{/if}
      {#if ab}<button type="button" class="ghost" onclick={() => store.run(Api.ClearAlbumBg())}>{ALBUM_CLEAR}</button>{/if}
    </span>
  </div>
  {#if ab}
    <div class="name" title={name}>{name}</div>
    <div class="fade" title="Чем больше, тем бледнее фон: сквозь него проступает цвет страницы">
      <span class="cap2">{ALBUM_FADE_CAP}</span>
      <input
        type="range"
        min="0"
        max="100"
        step="1"
        value={pct}
        aria-label="Прозрачность фона альбома, %"
        aria-valuetext="{pct} %"
        onfocus={() => (session = bdSession())}
        onblur={blur}
        oninput={(e) => albumFadeLive.set(ALBUM_FADE, ALBUM_FADE, pctFade(val(e)))}
        onchange={commit}
        onpointerup={(e) => e.currentTarget.blur()}
      />
      <span class="num">{pct} %</span>
    </div>
  {/if}
</div>

<style>
  .albumbg {
    display: grid;
    gap: 4px;
    font-size: 13px;
  }
  .line {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 8px;
  }
  .act {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  /* Образец — тот же размер и рамка, что у цвета страниц рядом */
  .sw {
    width: 30px;
    height: 24px;
    border: 1px solid var(--line);
    border-radius: 5px;
    background-color: var(--panel2);
    background-size: cover;
    background-position: center;
  }
  .sw.empty {
    background: linear-gradient(135deg, transparent 46%, var(--muted) 46% 54%, transparent 54%), var(--panel2);
  }
  .name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--muted);
    font-size: 12px;
  }
  .fade {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
  }
  .cap2 {
    flex: none;
    color: #cfcfd4;
  }
  input {
    flex: 1;
    min-width: 0;
    height: 16px;
    margin: 0;
    accent-color: var(--accent);
  }
  .num {
    flex: none;
    width: 34px;
    text-align: right;
    font-variant-numeric: tabular-nums;
  }
</style>
