<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import ColorPicker from '../../base/ui/ColorPicker.svelte'
  import { decor } from '../../panels/decor/decor.svelte'
  import { tileSrc } from '../../panels/decor/decor'
  import { placesel } from '../places/placesel.svelte'
  import TiltRow from '../photoprops/TiltRow.svelte'
  import { borderAct } from './borderact.svelte'
  import { borderlive } from './borderlive.svelte'
  import { SIZE, WIDTH, borderRows, inkPatch, targetsOf } from './borderrows'
  import { BORDER_LOST_FILE, BORDER_NEWER, BORDER_UNLISTED } from './borderstate'
  import { LIB_MISSING_TEXT } from '../../panels/library/missing'

  // Раздел «Рамочка» карточки фото, места и группы (079): образец — строка «Узор» в «Фоне». Нет рамочки — «Выбрать…»
  // (окно «Рамочки», «Куда» по выделению); есть — плитка и имя, цвета, «Толщина, мм» (встроенная) или «Размер, %»
  // (своя), «Сменить…», «Убрать». Ползунок — живой показ окна фото (borderlive) и один шаг отмены на серию (merge на
  // фокус строки); цвет — шаг на выбор. У группы разное — «—», первая правка ставит всем одно.
  const sp = $derived(store.spread)
  const sid = $derived(sp?.id ?? '')
  const rows = $derived(
    borderRows(
      targetsOf(
        sp,
        placesel.selected.map((b) => b.id),
      ),
      { missing: store.missing, nameOf: (lib) => decor.frameItem(lib)?.name },
    ),
  )
  const item = $derived(decor.frameItem(rows?.tile))
  // Плитка не рисуется, если рисовать нечего: вид новее программы или файл пропал (как у стикера).
  const blank = $derived(!!rows && (rows.newer || rows.lostLib + rows.lostFile > 0))
  // Своя рамочка цела, но каталог её не знает (скрыта или удалена): имя «Своя рамочка» и пояснение.
  const unlisted = $derived(!!rows && rows.own && !!rows.tile && !item && decor.framesReady && !blank)

  // Плитки каталога нужны, только если рамочка из плитки: имя и миниатюра.
  $effect(() => {
    if (rows?.tile || rows?.own) void decor.ensureFrames()
  })

  const choose = () => decor.show('frame', sid, 'spread')

  // Уход с листа или смена выделения: просмотр и сессия не остаются.
  const scope = $derived(`${sid}:${placesel.selected.map((b) => b.id).join(',')}`)
  $effect(() => {
    void scope
    return () => borderAct.end()
  })
</script>

<div class="bd" role="group" aria-label="Рамочка" data-guide="border">
  <div class="cap">Рамочка</div>
  {#if !rows}
    <div class="btns">
      <button onclick={choose} title="Рамочки из библиотеки: линии, паспарту, полароид, свои картинки">Выбрать…</button>
    </div>
  {:else}
    <div class="have" class:newer={rows.newer}>
      {#if item && !blank}
        <img class:chk={rows.own} src={tileSrc(item.url, 128)} alt="" draggable="false" />
      {:else}
        <span class="ph" aria-hidden="true"></span>
      {/if}
      <span class="name" title={rows.name}>{rows.name}</span>
    </div>
    {#if rows.newer}
      <p class="note">{BORDER_NEWER}</p>
    {/if}
    {#if rows.lostLib > 0}
      <p class="note warn">{LIB_MISSING_TEXT}</p>
    {/if}
    {#if rows.lostFile > 0}
      <p class="note warn">{BORDER_LOST_FILE}</p>
    {/if}
    {#if unlisted}
      <p class="note">{BORDER_UNLISTED}</p>
    {/if}
    {#if rows.inks.length > 0}
      <div class="row">
        <span class="lbl">{rows.inks.length > 1 ? 'Цвета' : 'Цвет'}</span>
        <div class="inks">
          {#each rows.inks as c, i (i)}
            <ColorPicker
              value={c}
              label="Цвет рамочки {i + 1}"
              onpick={(hex) => borderAct.ink(sid, rows.inkIds[i], { inks: inkPatch(rows, i, hex) })}
            />
          {/each}
        </div>
      </div>
    {/if}
    {#if rows.widthIds.length > 0}
      <TiltRow
        tilt={rows.width}
        mixed={rows.widthMixed}
        label="Толщина, мм"
        numLabel="Толщина рамочки, миллиметры"
        rangeLabel="Толщина рамочки"
        rangeTitle="Толщина рамочки, мм; двойной щелчок — как у выбранного вида"
        min={WIDTH.min}
        max={WIDTH.max}
        step={WIDTH.step}
        guide=""
        id="border-width"
        dflt={rows.defWidth}
        onlive={(v) => borderlive.set({ spread: sid, ids: rows.widthIds, width: v })}
        oncommit={(v) => borderAct.series(sid, 'width', rows.widthIds, { width: v })}
        onend={() => borderlive.release()}
        onleave={() => borderAct.session.end()}
      />
    {/if}
    {#if rows.sizeIds.length > 0}
      <TiltRow
        tilt={rows.size}
        mixed={rows.sizeMixed}
        label="Размер, %"
        numLabel="Размер рамочки, процент"
        rangeLabel="Размер рамочки"
        rangeTitle="Размер рамочки-картинки, %; двойной щелчок — как задумано"
        min={SIZE.min}
        max={SIZE.max}
        step={SIZE.step}
        guide=""
        id="border-size"
        dflt={SIZE.dflt}
        onlive={(v) => borderlive.set({ spread: sid, ids: rows.sizeIds, scale: v / 100 })}
        oncommit={(v) => borderAct.series(sid, 'size', rows.sizeIds, { scale: v / 100 })}
        onend={() => borderlive.release()}
        onleave={() => borderAct.session.end()}
      />
    {/if}
    <div class="btns">
      <button title="Другая рамочка из библиотеки" onclick={choose}>Сменить…</button>
      <button class="ghost" onclick={() => borderAct.clear(sid, rows.all)}>Убрать</button>
    </div>
  {/if}
</div>

<style>
  .bd {
    margin-top: 10px;
    padding: 6px;
    border: 1px solid var(--line);
    border-radius: 6px;
    font-size: 12px;
  }
  .cap {
    font-weight: 600;
  }
  .have {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 6px;
  }
  .have img,
  .ph {
    width: 44px;
    height: 44px;
    flex: none;
    object-fit: contain;
    border-radius: 4px;
    background: var(--panel2);
  }
  .have img.chk {
    background: repeating-conic-gradient(#d4d4d8 0 25%, #f4f4f5 0 50%) 0 0 / 14px 14px;
  }
  .ph {
    border: 1px dashed var(--line);
    background: transparent;
  }
  /* Пояснения — как в карточке стикера. */
  .note {
    margin: 6px 0 0;
    color: var(--muted);
  }
  .note.warn {
    color: #ff8a8a;
  }
  .newer .name {
    color: var(--muted);
  }
  .name {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-top: 6px;
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
</style>
