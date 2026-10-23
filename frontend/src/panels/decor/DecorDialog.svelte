<script lang="ts">
  import { screenDpr } from '../../base/ui/dpr.svelte'
  import { store } from '../../base/core/store.svelte'
  import { sheetIn, unitOf } from '../../base/geom/sheet'
  import Modal from '../../base/modal/Modal.svelte'
  import SideList from '../../base/tpl/SideList.svelte'
  import TplItem from '../../base/tpl/TplItem.svelte'
  import { decor } from './decor.svelte'
  import BgTargets from './BgTargets.svelte'
  import { bgPickShown } from './bgtargets'
  import { ALBUM_BG } from './albumbg'
  import DecorHead from './DecorHead.svelte'
  import QrDialog from './QrDialog.svelte'
  import SpreadSave from './SpreadSave.svelte'
  import { FILTERED_EMPTY, saveScopes, tileLook, tplNote, tplSideItems, tplSrc, tplTitle, tplVisible } from './spreadtpl'
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
    sideItems,
    themeItems,
    tilePx,
    tileSrc,
    uploadHint,
    whereItems,
  } from './decor'

  // Окно «Фоны» (048; те же плитки для «Стикеры» и «Развороты» 067): темы слева («источник слева», как в галерее шаблонов ⊞), справа плитки в пропорции листа,
  // сверху «Куда» и «Добавить свой…» (100-1). Щелчок по плитке ставит фон и закрывает окно; ✎ и × — у своих, как у «Мои шаблоны».
  const TILE_W = 168 // CSS px: плитка с запасом в dpr берёт PNG шириной px

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
  const items = $derived(spreads && sp ? tplVisible(themed, scope, sp) : themed)
  const readOnly = $derived(!!decor.cat?.readOnly)
  const px = $derived(tilePx(TILE_W, screenDpr.value))
  // Плитка, что стоит в этой области сейчас: отмечена, как текущая раскладка в галерее ⊞
  const standing = $derived(decor.standing(scope))
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

  // Отмеченная плитка при открытии окна не должна прятаться за краем прокрутки.
  function seenIf(el: HTMLElement, on: boolean) {
    if (on) el.scrollIntoView?.({ block: 'nearest' })
    return { update: (v: boolean) => v && el.scrollIntoView?.({ block: 'nearest' }) }
  }

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
          items={spreads && sp ? tplSideItems(decor.cat, scope, sp) : sideItems(decor.cat)}
          value={decor.theme}
          onpick={(id) => decor.setTheme(id)}
          label="Темы"
          scroll
        />
      </div>
      <div class="main">
        <DecorHead {c} {where} {scope} {album} />
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
        <div class="tiles" aria-busy={decor.loading}>
          {#each items as it (it.id)}
            {@const look = spreads ? tileLook(it, sp, scope, TILE_W, dpr) : null}
            <TplItem
              name={it.name}
              note={spreads ? tplNote(it) : undefined}
              onrename={it.own && !readOnly ? () => decor.askRename(it) : undefined}
              ondelete={it.own && !readOnly ? () => decor.askHide(it) : undefined}
              renameLabel="Переименовать {noun}"
              deleteTitle="Убрать из библиотеки"
              deleteLabel="Убрать {noun} из библиотеки"
            >
              {#if look}
                <!-- 067: лист одной пропорции на всё окно; страница в «Весь разворот» стоит по центру на своей -->
                <button
                  class="tile tpl"
                  style:aspect-ratio={look.box}
                  title={tplTitle(it)}
                  disabled={decor.busy}
                  onclick={() => decor.pick(it)}
                >
                  <img src={tplSrc(it.url, look.px, look.r)} alt="" loading="lazy" draggable="false" />
                </button>
              {:else}
                <button
                  class="tile"
                  class:current={it.id === standing}
                  class:stk={sticker}
                  class:frm={frame}
                  class:chk={frame && it.own}
                  aria-current={it.id === standing ? 'true' : undefined}
                  use:seenIf={it.id === standing}
                  style:aspect-ratio={sticker || frame ? '1 / 1' : `${sp.w} / ${sp.h}`}
                  style:background={it.color || undefined}
                  title={tplTitle(it)}
                  onclick={() => decor.pick(it)}
                >
                  <img src={tileSrc(it.url, px)} alt="" loading="lazy" draggable="false" />
                </button>
              {/if}
            </TplItem>
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
    grid-template-columns: repeat(auto-fill, 168px);
    align-content: start;
    gap: 12px 10px;
    padding: 5px 6px 6px 5px; /* место под рамку отмеченной плитки */
    overflow-y: auto;
  }
  .tile {
    display: block;
    position: relative;
    width: 168px;
    height: auto;
    padding: 0;
    overflow: hidden;
    border: 1px solid var(--line);
    border-radius: 4px;
    background: var(--panel2);
  }
  .tile:hover:not(:disabled) {
    border-color: var(--accent);
  }
  /* Стоящий сейчас — рамка, как у текущей раскладки в галерее ⊞ */
  .tile.current {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }
  .tile img {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    object-fit: cover;
    pointer-events: none;
  }
  /* Готовый разворот: картинка вписана в кнопку (страница уже листа — по краям подложка).
     Рамочка: квадратная плитка, рамка вокруг силуэта вписана целиком */
  .tile.tpl img,
  .tile.frm img {
    object-fit: contain;
  }
  /* Стикер: квадратная плитка, рисунок вписан целиком, под ним шашечка (видно прозрачность);
     у рамочки шашечка — только у своих */
  .tile.stk,
  .tile.chk {
    background: repeating-conic-gradient(#d4d4d8 0 25%, #f4f4f5 0 50%) 0 0 / 14px 14px;
  }
  .tile.stk img {
    object-fit: contain;
    inset: 6px;
    width: calc(100% - 12px);
    height: calc(100% - 12px);
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
