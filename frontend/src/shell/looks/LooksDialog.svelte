<script lang="ts">
  import { untrack } from 'svelte'
  import { screenDpr } from '../../base/ui/dpr.svelte'
  import { store } from '../../base/core/store.svelte'
  import { COVER_ID } from '../../base/geom/sheet'
  import Modal from '../../base/modal/Modal.svelte'
  import SideList from '../../base/tpl/SideList.svelte'
  import { ALL, NAME_MAX, themeItems, tilePx } from '../../panels/decor/decor'
  import { packsUi } from '../packs/packs.svelte'
  import PackRun from '../packs/PackRun.svelte'
  import BrowseFilters from '../../panels/decorbrowse/BrowseFilters.svelte'
  import GetAsk from '../../panels/decorbrowse/GetAsk.svelte'
  import GetTile from '../../panels/decorbrowse/GetTile.svelte'
  import { browse, sideCounts } from '../../panels/decorbrowse/browse'
  import { isGetItem } from '../../panels/decorbrowse/decorget'
  import { getOffHint } from '../../panels/decorbrowse/getoff'
  import { holdSize, tileOf } from '../../panels/decorbrowse/tilesize'
  import { tileWheel } from '../../panels/decorbrowse/tilewheel'
  import { confirmLookGet } from './lookget.svelte'
  import {
    EMPTY_DEMO,
    EMPTY_LOOKS,
    EMPTY_MINE,
    EMPTY_THEME,
    LOOKS_TITLE,
    MORE_LOOKS,
    NO_PHOTOS_TIP,
    hideText,
    stepAskTitle,
    tileRatio,
  } from './looks'
  import { looks } from './looks.svelte'
  import LookApply from './LookApply.svelte'
  import LookHead from './LookHead.svelte'
  import LookPeek from './LookPeek.svelte'
  import LookTile from './LookTile.svelte'

  // Окно «Макеты альбома» (131 U1, 163 U2–U3): каркас окна «Развороты» (DecorDialog) — темы со счётчиками, строка над отбором
  // (размер миниатюр, «Наборы…», «Сохранить альбом как макет…»), ряд «Набор · Стиль», плитки. Щелчок по плитке ничего не ставит
  // (0.27-131-12): открывает крупный просмотр (LookPeek), а «Применить…» в нём — окно с формой (LookApply).
  const side = $derived(sideCounts(looks.cat, looks.filters, { kind: 'album' }))
  const none = $derived(!!looks.cat && looks.cat.items.length === 0 && !looks.cat.get?.length && !looks.loading)
  const themed = $derived(themeItems(looks.cat, looks.theme))
  const br = $derived(browse(themed, looks.cat?.items ?? [], looks.filters, '', 'album'))
  const items = $derived(br.items)
  const readOnly = $derived(!!looks.cat?.readOnly)
  const cover = $derived(store.cover?.sheet)
  // 144: плитки «Можно скачать» серые, пока идёт скачивание и в библиотеке «только чтение»
  const getOff = $derived(getOffHint(packsUi.net, readOnly))
  const getBox = $derived(tileRatio(store.view.settings, { cover: false })) // как у стоящей плитки разворота
  // 133: размер плиток — ползунок в шапке и Ctrl+колесо; px картинки не уменьшается, пока окно открыто (протяжка вниз и назад не перезапрашивает)
  const size = $derived(tileOf('album'))
  let hold = $state(0)
  $effect.pre(() => {
    const sz = size
    if (!looks.open) hold = 0
    else untrack(() => (hold = Math.max(hold, sz)))
  })
  const px = $derived(tilePx(holdSize(hold, size), screenDpr.value))
  let tilesEl = $state<HTMLElement>()
  // Смена темы или отбора — плитки с начала (у макетов выбранной плитки нет); первое значение прокрутку не трогает
  const scrollKey = $derived(`${looks.theme}|${br.sel.pack}|${br.sel.style}`)
  let seenKey: string | null = null
  $effect(() => {
    const k = scrollKey
    if (seenKey === null || k === seenKey) {
      seenKey = k
      return
    }
    seenKey = k
    queueMicrotask(() => {
      if (tilesEl) tilesEl.scrollTop = 0
    })
  })

  // Правка альбома под открытым окном (формат в панели деталей, событие Go): числа и план пересчитать.
  $effect(() => {
    const rev = store.view.rev
    untrack(() => looks.sync(rev))
  })

  function enter(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      e.preventDefault()
      void looks.commit()
    }
  }
  const selectAll = (e: FocusEvent) => (e.currentTarget as HTMLInputElement).select()
</script>

{#if looks.open}
  <Modal title={LOOKS_TITLE} size="large" onclose={() => looks.close()}>
    {#if none}
      <div class="none">
        {looks.demoOffer ? EMPTY_DEMO : EMPTY_LOOKS} ·
        <button type="button" class="link" onclick={() => packsUi.show()}>Наборы…</button>
      </div>
    {:else}
      <div class="body" role="group" aria-label="Макеты" tabindex="-1" data-autofocus>
        <div class="side">
          <SideList items={side} value={looks.theme} onpick={(id) => looks.setTheme(id)} label="Темы" scroll />
        </div>
        <div class="main">
          <LookHead />
          <BrowseFilters {br} kind="album" onpick={(k, v) => looks.setFilter(k, v)} />
          {#if store.photos.length === 0}<p class="tip">{NO_PHOTOS_TIP}</p>{/if}
          <PackRun net={packsUi.net} />
          <div class="tiles" aria-busy={looks.loading} style:--tile="{size}px" use:tileWheel={'album'} bind:this={tilesEl}>
            {#each items as it (it.id)}
              {#if isGetItem(it)}
                <GetTile {it} kind="album" box={getBox} off={getOff} onget={(t) => looks.askGet(t)} />
              {:else}
                <LookTile
                  {it}
                  {px}
                  r={tileRatio(store.view.settings, it, cover)}
                  ph={looks.ph}
                  {readOnly}
                  onpeek={(t) =>
                    looks.openPeek(
                      items.map((i) => i.id),
                      t.id,
                    )}
                  onrename={(t) => looks.askRename(t)}
                  onhide={(t) => looks.askHide(t)}
                />
              {/if}
            {/each}
            {#if looks.theme === ALL && looks.cat?.invite}
              <!-- 144: списка наборов ещё не было — приглашение в конце «Все» -->
              <GetTile kind="album" oninvite={() => packsUi.show()} />
            {/if}
            {#if looks.cat && items.length === 0}
              <div class="empty">{looks.theme === 'mine' ? EMPTY_MINE : EMPTY_THEME}</div>
            {/if}
          </div>
          {#if looks.cat?.demoOnly}
            <!-- Все макеты окна — из примера (142): дальше их приносят наборы; образец — пустое состояние -->
            <div class="more">
              {MORE_LOOKS} ·
              <button type="button" class="link" onclick={() => packsUi.show()}>Наборы…</button>
            </div>
          {/if}
        </div>
      </div>
    {/if}
  </Modal>
  <!-- Рядом с окном, а не в нём: просмотр и «Применить» лежат в стеке окон выше (как DecorPeek у DecorDialog) -->
  <LookPeek {px} />
{/if}

{#if looks.open && looks.ask?.kind === 'rename'}
  <Modal title="Переименовать макет" onclose={() => looks.dismiss()}>
    <input type="text" maxlength={NAME_MAX} data-autofocus bind:value={looks.name} onkeydown={enter} onfocus={selectAll} aria-label="Имя" />
    <div class="actions">
      <button class="ghost" onclick={() => looks.dismiss()}>Отмена</button>
      <button class="primary" onclick={() => looks.commit()}>Сохранить</button>
    </div>
  </Modal>
{:else if looks.open && looks.ask?.kind === 'hide'}
  <Modal title="Убрать из библиотеки?" onclose={() => looks.dismiss()}>
    <p>{hideText(looks.ask.name)}</p>
    <div class="actions">
      <button class="ghost" onclick={() => looks.dismiss()}>Отмена</button>
      <button class="primary" onclick={() => looks.commit()}>Убрать</button>
    </div>
  </Modal>
{:else if looks.open && looks.ask?.kind === 'form'}
  <LookApply />
{:else if looks.open && looks.ask?.kind === 'get'}
  <!-- 144: образец макета из нескачанного набора; «Отмена» возвращает к плиткам -->
  <GetAsk tile={looks.ask.tile} kind="album" onconfirm={() => confirmLookGet()} oncancel={() => looks.dismiss()} />
{:else if looks.open && looks.ask?.kind === 'step'}
  <!-- «Поставить на разворот N»: тот же вопрос, что у «Развороты» (067) -->
  <Modal title={stepAskTitle(looks.ask.spread === COVER_ID)} onclose={() => looks.dismiss()}>
    <p>{looks.ask.text}</p>
    <div class="actions">
      <button class="ghost" onclick={() => looks.dismiss()}>Отмена</button>
      <button class="primary" data-autofocus onclick={() => looks.confirmStep()}>Поставить</button>
    </div>
  </Modal>
{/if}

<style>
  /* Высота задана окну, а не содержимому: темы и плитки прокручиваются каждые сами */
  .body {
    outline: none;
    display: flex;
    gap: 12px;
    height: min(560px, calc(100vh - 150px));
  }
  .side {
    flex: none;
    width: 150px;
    min-height: 0;
  }
  .main {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .tiles {
    flex: 1;
    min-height: 0;
    display: grid;
    grid-template-columns: repeat(auto-fill, var(--tile));
    align-content: start;
    gap: 12px 10px;
    padding: 5px 6px 6px 5px; /* место под рамку при наведении */
    overflow-y: auto;
  }
  .tip {
    flex: none;
    margin: 0;
    color: var(--muted);
    font-size: 12px;
  }
  .empty {
    grid-column: 1 / -1;
    padding: 30px 0 0 8px;
    color: var(--muted);
    font-size: 13px;
  }
  .none {
    padding: 40px 0;
    text-align: center;
    color: var(--muted);
    font-size: 13px;
  }
  .more {
    flex: none;
    color: var(--muted);
    font-size: 12px;
    text-align: center;
  }
  .link {
    all: unset;
    color: var(--accent);
    cursor: pointer;
    text-decoration: underline;
  }
  .link:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }
  input[type='text'] {
    width: 100%;
    height: 30px;
    padding: 0 8px;
    font: inherit;
    background: #3a3a3e;
    color: var(--text);
    border: 1px solid var(--line);
    border-radius: 6px;
    user-select: text;
  }
  p {
    margin: 0 0 10px;
    line-height: 1.4;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 12px;
  }
</style>
