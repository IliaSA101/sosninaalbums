<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import { thumbUrl } from '../../base/core/media'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import ColorPicker from '../../base/ui/ColorPicker.svelte'
  import Icon from '../../base/ui/Icon.svelte'
  import { LIB_MISSING_TEXT } from '../../panels/library/missing'
  import { decor } from '../../panels/decor/decor.svelte'
  import { tileSrc } from '../../panels/decor/decor'
  import { fadePct, pctFade } from '../backdrop/fade'
  import { placesel } from '../places/placesel.svelte'
  import LayerRow from '../inspector/LayerRow.svelte'
  import SnapToggle from '../photoprops/SnapToggle.svelte'
  import { STICKER_NEWER, inksOf, isLost, stKey, stickerName } from './sticker'
  import { sticker, stickerFadeLive } from './sticker.svelte'

  // Карточка одного стикера (048 Ф2, сц. 13): превью и название, цвета (у программы, 1–3), «Прозрачность», зеркало ↔ ↕,
  // «Заменить…» (центр, ширина и поворот остаются), «Привязка», расположение, «Удалить»; у своего — dpi и «!».
  // Ползунок: живой просмотр на oninput, один вызов Go на change, стрелки в одном фокусе — один шаг отмены (merge-сессия).
  // head=false — заголовок рисует сама карточка (docked, cardTitle): сворачиваемая шапка.
  let { head = true }: { head?: boolean } = $props()

  const st = $derived(placesel.soloSticker)
  const sid = $derived(store.spread?.id ?? '')
  const lost = $derived(!!st && isLost(st, store.missing))
  const fade = $derived(st ? stickerFadeLive.of(sid, st.id, st.fade) : 0)
  const low = $derived(!!st && st.dpi > 0 && st.dpi < store.view.minDPI)
  // Плитка каталога: имя своего берётся из библиотеки (View его не несёт), у программного — из View.
  const item = $derived(decor.stickerItem(st?.lib))
  const name = $derived(st ? (st.gen || !st.known ? stickerName(st) : (item?.name ?? 'Свой стикер')) : '')

  // Имена своих стикеров — из каталога, читается один раз.
  $effect(() => {
    if (st && !st.gen) void decor.ensureStickers()
  })

  const val = (e: Event) => +(e.currentTarget as HTMLInputElement).value
  const key = $derived(st ? stKey(sid, st.id) : '')

  // Chromium не шлёт change, если значение при отпускании равно исходному: тогда просмотр залип бы.
  function blur() {
    sticker.session.end()
    if (st && fadePct(fade) === fadePct(st.fade)) stickerFadeLive.release()
  }

  // Уход с листа или смена стикера: просмотр не остаётся на чужом.
  $effect(() => {
    void key
    return () => {
      sticker.session.end()
      stickerFadeLive.release()
    }
  })

  function replace() {
    const t = sticker.target()
    if (t) decor.show('sticker', t.spread, 'spread', t.id)
  }
</script>

{#if st}
  {#if head}<div class="head">Стикер</div>{/if}
  <div class="have" class:newer={!st.known}>
    {#if lost || !st.known}
      <span class="ph" aria-hidden="true"></span>
    {:else if st.gen && st.url}
      <img src={tileSrc(st.url, 128)} alt="" draggable="false" />
    {:else if st.photo}
      <img src={thumbUrl(st.photo)} alt="" draggable="false" />
    {/if}
    <span class="name" title={name}>{name}</span>
  </div>
  {#if !st.known}
    <p class="note">{STICKER_NEWER}</p>
  {:else if lost}
    <p class="note warn">{LIB_MISSING_TEXT}</p>
  {/if}

  {#if st.known && st.gen && st.inks > 0}
    <div class="row">
      <span class="lbl">{st.inks > 1 ? 'Цвета' : 'Цвет'}</span>
      <div class="inks">
        {#each inksOf(st) as c, i (i)}
          <ColorPicker
            value={c}
            label="Цвет стикера {i + 1}"
            onpick={(hex) =>
              sticker.setInks(
                st,
                inksOf(st).map((x, j) => (j === i ? hex : x)),
              )}
          />
        {/each}
      </div>
    </div>
  {/if}

  <div class="row" title="Чем больше, тем бледнее стикер: сквозь него проступает лист">
    <span class="lbl">Прозрачность</span>
    <input
      type="range"
      min="0"
      max="100"
      step="1"
      value={fadePct(fade)}
      aria-label="Прозрачность стикера, %"
      aria-valuetext="{fadePct(fade)} %"
      onfocus={() => sticker.session.begin(key)}
      onblur={blur}
      oninput={(e) => stickerFadeLive.set(sid, st.id, pctFade(val(e)))}
      onchange={(e) => {
        const merge = sticker.session.merge(key)
        return sticker.setFade(st, pctFade(val(e)), merge)
      }}
      onpointerup={(e) => e.currentTarget.blur()}
    />
    <span class="num">{fadePct(fade)} %</span>
  </div>

  <div class="lbl2">Зеркало</div>
  <div class="seg" role="group" aria-label="Зеркало">
    <button class:on={st.flipH} aria-pressed={st.flipH} title="Отразить по горизонтали" onclick={() => sticker.flip('h')}
      ><Icon name="flip-h" size={14} />↔</button
    >
    <button class:on={st.flipV} aria-pressed={st.flipV} title="Отразить по вертикали" onclick={() => sticker.flip('v')}
      ><Icon name="flip-v" size={14} />↕</button
    >
  </div>

  <button class="wide" title="Другой стикер на это место: центр, ширина и поворот остаются" onclick={replace}>Заменить…</button>

  {#if !st.gen && st.dpi > 0}
    <p class="dpi" class:low title={low ? `Нужно от ${store.view.minDPI} dpi` : 'dpi при печати'}>
      ≈ {Math.round(st.dpi)} dpi{#if low}<span class="bang" role="img" aria-label="Мало точек">!</span>{/if}
    </p>
  {/if}

  {#if prefs.can('manual')}<SnapToggle />{/if}
  <!-- Слой стикера — в общем стеке с фото и текстом; только «Ручная» -->
  {#if placesel.canLayer && prefs.can('layers')}
    <LayerRow onpick={(op) => placesel.layer(op)} />
  {/if}

  <button class="del" onclick={() => placesel.deleteSelected()} title="Удалить (Delete / Backspace)"
    ><Icon name="trash" size={14} />Удалить</button
  >
{/if}

<style>
  .head {
    font-size: 13px;
    font-weight: 600;
    margin-bottom: 8px;
  }
  .have {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 12px;
  }
  .have img,
  .ph {
    width: 44px;
    height: 44px;
    flex: none;
    object-fit: contain;
    border-radius: 4px;
    /* шашечка: видно, что у картинки есть прозрачность */
    background: repeating-conic-gradient(#d4d4d8 0 25%, #f4f4f5 0 50%) 0 0 / 10px 10px;
  }
  .ph {
    border: 1px dashed var(--line);
    background: transparent;
  }
  .name {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .newer .name {
    color: var(--muted);
  }
  .note {
    margin: 6px 0 0;
    font-size: 12px;
    color: var(--muted);
  }
  .note.warn {
    color: #ff8a8a;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-top: 8px;
    font-size: 12px;
  }
  .lbl {
    flex: none;
    min-width: 72px;
    color: #cfcfd4;
  }
  .inks {
    display: flex;
    gap: 6px;
  }
  input[type='range'] {
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
  .lbl2 {
    margin-top: 10px;
    font-size: 12px;
    color: var(--muted);
  }
  .seg {
    display: flex;
    margin-top: 4px;
    border-radius: 5px;
    overflow: hidden;
    gap: 1px;
  }
  .seg button {
    flex: 1;
    height: 26px;
    padding: 0 6px;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    font-size: 12px;
    border-radius: 0;
    background: #38383c;
    color: #cfcfd4;
  }
  .seg button.on {
    background: var(--accent);
    color: #1e1e1e;
  }
  .seg button.on:hover {
    background: #ffe53a;
  }
  .wide {
    margin-top: 10px;
    width: 100%;
    height: 26px;
    padding: 0 8px;
    font-size: 12px;
    background: #3a3a3e;
  }
  .dpi {
    margin: 8px 0 0;
    font-size: 12px;
    color: var(--muted);
  }
  .dpi.low {
    color: #ff8a8a;
  }
  .bang {
    display: inline-block;
    margin-left: 6px;
    width: 14px;
    height: 14px;
    border-radius: 50%;
    background: #d9382c;
    color: #fff;
    font-size: 10px;
    font-weight: 800;
    line-height: 14px;
    text-align: center;
  }
  .del {
    margin-top: 10px;
    width: 100%;
    height: 26px;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    font-size: 12px;
    background: #38383c;
    color: #cfcfd4;
  }
  .del:hover:not(:disabled) {
    color: #ff8a8a;
  }
</style>
