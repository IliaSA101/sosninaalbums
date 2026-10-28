<script lang="ts">
  import { untrack } from 'svelte'
  import { screenDpr } from '../../base/ui/dpr.svelte'
  import { store } from '../../base/core/store.svelte'
  import { sheetIn, unitOf } from '../../base/geom/sheet'
  import Modal from '../../base/modal/Modal.svelte'
  import SideList from '../../base/tpl/SideList.svelte'
  import DecorTile from '../decorbrowse/DecorTile.svelte'
  import DecorPeek from '../decorbrowse/DecorPeek.svelte'
  import BrowseFilters from '../decorbrowse/BrowseFilters.svelte'
  import { browse, sideCounts, tileLabel } from '../decorbrowse/browse'
  import { peek } from '../decorbrowse/peek.svelte'
  import { holdSize, tileOf } from '../decorbrowse/tilesize'
  import { tileWheel } from '../decorbrowse/tilewheel'
  import { decor } from './decor.svelte'
  import BgTargets from './BgTargets.svelte'
  import { bgPickShown } from './bgtargets'
  import { ALBUM_BG } from '../albumbg/albumbg'
  import AlbumOwnAsk from '../albumbg/AlbumOwnAsk.svelte'
  import DecorHead from './DecorHead.svelte'
  import QrDialog from './QrDialog.svelte'
  import SpreadSave from './SpreadSave.svelte'
  import { FILTERED_EMPTY, saveScopes, tplSideItems, tplVisible } from './spreadtpl'
  import {
    ADD_OWN,
    DECOR_TITLE,
    MINE,
    NAME_MAX,
    NOUN,
    OWN_TIP_STICKER,
    deleteText,
    emptyText,
    ownKind,
    scopeIn,
    themeItems,
    uploadHint,
    whereItems,
  } from './decor'

  // Окно «Фоны» (048; те же плитки для «Стикеры» и «Развороты» 067): темы слева («источник слева», как в галерее шаблонов ⊞), справа плитки в пропорции листа,
  // сверху «Куда» и «Добавить свой…» (100-1). Щелчок по плитке ставит фон и закрывает окно; ✎ и × — у своих, как у «Мои шаблоны».

  const c = $derived(decor.cur)
  const sp = $derived(c ? sheetIn(store.view, c.spread) : undefined)
  const unit = $derived(unitOf(store.view.settings))
  const where = $derived(whereItems(sp, store.cover, unit.all))
  const scope = $derived(c ? scopeIn(where, c.scope) : 'spread')
  const sticker = $derived(c?.kind === 'sticker')
  const album = $derived(decor.albumNow()) // 098: плитка ставит фон альбома, а не фон листа
  const albumOnly = $derived(!!c?.album) // окно «Фон альбома»: строки «На» нет
  const frame = $derived(c?.kind === 'frame') // 079: «Куда» — выделенные места или весь лист, а не область листа
  const spreads = $derived(c?.kind === 'spread') // 067: плитки готовых разворотов, «Куда» отбирает их
  const themed = $derived(themeItems(decor.cat, decor.theme))
  const listed = $derived(spreads && sp ? tplVisible(themed, scope, sp) : themed)
  const readOnly = $derived(!!decor.cat?.readOnly)
  // 133: размер плиток — по виду окна (ползунок в шапке, Ctrl+колесо); px картинки от него не уменьшается, пока окно открыто
  const size = $derived(c ? tileOf(c.kind) : 0)
  let hold = $state(0)
  let holdKind = ''
  $effect.pre(() => {
    const k = c?.kind ?? ''
    const sz = size
    untrack(() => {
      if (k !== holdKind) {
        holdKind = k
        hold = 0
      }
      if (sz > hold) hold = sz
    })
  })
  const pxSize = $derived(holdSize(hold, size))
  // Плитка, что стоит в этой области сейчас: отмечена, как текущая раскладка в галерее ⊞
  const standing = $derived(decor.standing(scope))
  // 133: отбор «Набор · Вид · Стиль» после темы и «Куда»; стоящая плитка видна при любом отборе
  const br = $derived(browse(listed, decor.cat?.items ?? [], decor.filters, standing))
  const items = $derived(br.items)
  let tilesEl = $state<HTMLElement>()
  // Смена источника или отбора — к стоящей плитке, иначе в начало (образец — TplRow). Ключ — строка: пересчёт br, перечитывание
  // каталога, смена standing и открытие окна тех же значений прокрутку не трогают (стоящую при открытии показывает seenIf плитки).
  const scrollKey = $derived(`${decor.theme}|${br.sel.pack}|${br.sel.form}|${br.sel.style}`)
  let seenKey: string | null = null
  $effect(() => {
    const k = scrollKey
    if (seenKey === null || k === seenKey) {
      seenKey = k
      return
    }
    seenKey = k
    queueMicrotask(() => {
      if (!tilesEl) return
      const cur = tilesEl.querySelector<HTMLElement>('.current')
      if (cur) cur.scrollIntoView?.({ block: 'nearest' })
      else tilesEl.scrollTop = 0
    })
  })
  // 077: строка «на какие листы» под «Куда» — у «Фонов», не на обложке и не в альбоме из одного разворота
  const manyRow = $derived(c?.kind === 'bg' && !albumOnly && bgPickShown(sp, store.view.spreads.length))
  const bgPlan = $derived(decor.bgPlan())
  const noun = $derived(c ? NOUN[c.kind] : 'фон')
  const dpr = $derived(screenDpr.value)
  const emptyMsg = $derived(spreads && themed.length > 0 && !decor.loading ? FILTERED_EMPTY : emptyText(decor.theme, readOnly, c?.kind))

  // Лист пропал, пока окно открыто (удалён, смена альбома): закрываем. Область пропала из списка — первая доступная.
  $effect(() => {
    if (!c) return
    if (!sp) decor.close()
    else if (!frame && !album && scope !== c.scope) decor.setScope(scope)
  })

  // Enter в поле имени — как кнопка окна.
  function enter(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      e.preventDefault()
      void decor.commit()
    }
  }
  const selectAll = (e: FocusEvent) => (e.currentTarget as HTMLInputElement).select()
</script>

{#if c && sp}
  <Modal title={albumOnly ? ALBUM_BG : DECOR_TITLE[c.kind]} size="large" undo onclose={() => decor.close()}>
    <div class="body">
      <div class="side">
        <SideList
          items={spreads && sp
            ? tplSideItems(decor.cat, scope, sp, decor.filters, standing)
            : sideCounts(decor.cat, decor.filters, { keep: standing })}
          value={decor.theme}
          onpick={(id) => decor.setTheme(id)}
          label="Темы"
          scroll
        />
      </div>
      <div class="main">
        <DecorHead {c} {where} {scope} {album} />
        <BrowseFilters {br} onpick={(k, v) => decor.setFilter(k, v)} />
        {#if manyRow}
          <BgTargets
            bind:pick={decor.bgPick}
            bind:text={decor.bgText}
            selN={decor.selIds().length}
            total={store.view.spreads.length}
            {unit}
            {scope}
            n={bgPlan.targets.length}
            error={bgPlan.error}
            focus={decor.numsFocus}
          />
        {/if}
        {#if sticker && decor.theme === MINE}<p class="tip">{OWN_TIP_STICKER}</p>{/if}
        <div class="tiles" aria-busy={decor.loading} style:--tile="{size}px" use:tileWheel={c.kind} bind:this={tilesEl}>
          {#each items as it (it.id)}
            <DecorTile
              {it}
              label={tileLabel(it, br.sel.style)}
              kind={c.kind}
              {sp}
              {scope}
              current={it.id === standing}
              {readOnly}
              {noun}
              hold={pxSize}
              {dpr}
              onpeek={(t) =>
                peek.open(
                  items.map((i) => i.id),
                  t.id,
                )}
            />
          {/each}
          {#if decor.cat && items.length === 0}
            <div class="empty">
              {emptyMsg}
              {#if decor.theme === MINE && !readOnly && !decor.loading && ownKind(c.kind)}
                <!-- та же кнопка, что в строке выше, по центру пустой темы -->
                <div class="center">
                  <button class="primary" disabled={decor.busy} title={uploadHint(c.kind)} onclick={() => decor.upload()}
                    >{ADD_OWN[c.kind]}</button
                  >
                </div>
              {/if}
            </div>
          {/if}
        </div>
      </div>
    </div>
  </Modal>
  <DecorPeek {sp} {scope} hold={pxSize} {dpr} />
{/if}

{#if c && decor.ask?.kind === 'rename'}
  <Modal title="Переименовать {noun}" onclose={() => decor.dismiss()}>
    <input type="text" maxlength={NAME_MAX} data-autofocus bind:value={decor.name} onkeydown={enter} onfocus={selectAll} aria-label="Имя" />
    <div class="actions">
      <button class="ghost" onclick={() => decor.dismiss()}>Отмена</button>
      <button class="primary" onclick={() => decor.commit()}>Сохранить</button>
    </div>
  </Modal>
{:else if c && decor.ask?.kind === 'hide'}
  <Modal title="Убрать из библиотеки?" onclose={() => decor.dismiss()}>
    <p>{deleteText(decor.ask.name, c.kind)}</p>
    <div class="actions">
      <button class="ghost" onclick={() => decor.dismiss()}>Отмена</button>
      <button class="primary" onclick={() => decor.commit()}>Убрать</button>
    </div>
  </Modal>
{:else if c && decor.ask?.kind === 'put'}
  <!-- 067: вопрос перед постановкой разворота; «Отмена» возвращает в окно плиток -->
  <Modal title="Поставить разворот?" onclose={() => decor.dismiss()}>
    <p>{decor.ask.text}</p>
    <div class="actions">
      <button class="ghost" onclick={() => decor.dismiss()}>Отмена</button>
      <button class="primary" data-autofocus onclick={() => decor.confirmPut()}>Поставить</button>
    </div>
  </Modal>
{:else if c && decor.ask?.kind === 'bgMany'}
  <!-- 077: вопрос, если уходят фото фона; «Отмена» возвращает в окно плиток -->
  <Modal title="Поставить фон?" onclose={() => decor.dismiss()}>
    <p>{decor.ask.text}</p>
    <div class="actions">
      <button class="ghost" onclick={() => decor.dismiss()}>Отмена</button>
      <button class="primary" data-autofocus onclick={() => decor.confirmBgMany()}>Поставить</button>
    </div>
  </Modal>
{:else if c && decor.ask?.kind === 'albumOwn'}
  <!-- 115: у разворотов свой фон; × возвращает к плиткам, правки нет (как вопрос 077) -->
  <AlbumOwnAsk
    ask={decor.ask.own}
    onkeep={() => decor.answerAlbumOwn(false)}
    onall={() => decor.answerAlbumOwn(true)}
    onclose={() => decor.dismiss()}
  />
{/if}

<SpreadSave scopes={saveScopes(where)} />
<QrDialog />

<style>
  /* Высота задана окну, а не содержимому: темы и плитки прокручиваются каждые сами, шапка окна остаётся на месте */
  .body {
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
    padding: 5px 6px 6px 5px; /* место под рамку отмеченной плитки */
    overflow-y: auto;
  }
  .tip {
    flex: none;
    margin: 0;
    color: var(--muted);
    font-size: 12px;
  }
  .center {
    margin-top: 14px;
    text-align: center;
  }
  .empty {
    grid-column: 1 / -1;
    padding: 30px 0 0 8px;
    color: var(--muted);
    font-size: 13px;
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
