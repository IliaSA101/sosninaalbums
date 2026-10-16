<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import { sheetIn, unitOf } from '../../base/geom/sheet'
  import Modal from '../../base/ui/Modal.svelte'
  import Seg from '../../base/tpl/Seg.svelte'
  import SideList from '../../base/tpl/SideList.svelte'
  import TplItem from '../../base/tpl/TplItem.svelte'
  import { decor } from './decor.svelte'
  import {
    DECOR_TITLE,
    NAME_MAX,
    READ_ONLY_HINT,
    deleteText,
    emptyText,
    scopeIn,
    sideItems,
    standingId,
    standingSticker,
    themeItems,
    tilePx,
    tileSrc,
    uploadHint,
    whereItems,
  } from './decor'

  // Окно «Фоны» (048): темы слева («источник слева», как в галерее шаблонов ⊞), справа плитки в пропорции листа,
  // сверху «Куда» и «Загрузить своё…». Щелчок по плитке ставит фон и закрывает окно; ✎ и × — у своих, как у «Мои шаблоны».
  const TILE_W = 168 // CSS px: плитка с запасом в dpr берёт PNG шириной px

  const c = $derived(decor.cur)
  const sp = $derived(c ? sheetIn(store.view, c.spread) : undefined)
  const unit = $derived(unitOf(store.view.settings))
  const where = $derived(whereItems(sp, store.cover?.layout, unit.all))
  const scope = $derived(c ? scopeIn(where, c.scope) : 'spread')
  const items = $derived(themeItems(decor.cat, decor.theme))
  const readOnly = $derived(!!decor.cat?.readOnly)
  const px = $derived(tilePx(TILE_W, globalThis.devicePixelRatio || 1))
  // Плитка, что стоит в этой области сейчас: отмечена, как текущая раскладка в галерее ⊞
  const sticker = $derived(c?.kind === 'sticker')
  const standing = $derived(
    sticker ? standingSticker(sp, c?.replace, decor.cat?.items ?? []) : standingId(sp, scope, decor.cat?.items ?? []),
  )
  const noun = $derived(sticker ? 'стикер' : 'фон')

  // Лист пропал, пока окно открыто (удалён, смена альбома): закрываем. Область пропала из списка — первая доступная.
  $effect(() => {
    if (!c) return
    if (!sp) decor.close()
    else if (scope !== c.scope) decor.setScope(scope)
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
  <Modal title={DECOR_TITLE[c.kind]} size="large" onclose={() => decor.close()}>
    <div class="body">
      <div class="side">
        <SideList items={sideItems(decor.cat)} value={decor.theme} onpick={(id) => decor.setTheme(id)} label="Темы" scroll />
      </div>
      <div class="main">
        <div class="bar">
          {#if c.kind === 'bg' && where.length > 1}
            <span class="lbl">Куда</span>
            <Seg items={where} value={scope} onpick={(s) => decor.setScope(s)} label="Куда поставить фон" />
          {/if}
          <span class="grow"></span>
          <button
            class="ghost"
            disabled={decor.busy || readOnly}
            title={readOnly ? READ_ONLY_HINT : uploadHint(c.kind)}
            onclick={() => decor.upload()}>Загрузить своё…</button
          >
        </div>
        <div class="tiles" aria-busy={decor.loading}>
          {#each items as it (it.id)}
            <TplItem
              name={it.name}
              onrename={it.own && !readOnly ? () => decor.askRename(it) : undefined}
              ondelete={it.own && !readOnly ? () => decor.askHide(it) : undefined}
              renameLabel="Переименовать {noun}"
              deleteTitle="Убрать из библиотеки"
              deleteLabel="Убрать {noun} из библиотеки"
            >
              <button
                class="tile"
                class:current={it.id === standing}
                class:stk={sticker}
                aria-current={it.id === standing ? 'true' : undefined}
                use:seenIf={it.id === standing}
                style:aspect-ratio={sticker ? '1 / 1' : `${sp.w} / ${sp.h}`}
                style:background={it.color || undefined}
                title={it.name}
                onclick={() => decor.pick(it)}
              >
                <img src={tileSrc(it.url, px)} alt="" loading="lazy" draggable="false" />
              </button>
            </TplItem>
          {/each}
          {#if decor.cat && items.length === 0}
            <div class="empty">{emptyText(decor.theme, readOnly)}</div>
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
    <p>{deleteText(decor.ask.name)}</p>
    <div class="actions">
      <button class="ghost" onclick={() => decor.dismiss()}>Отмена</button>
      <button class="primary" onclick={() => decor.commit()}>Убрать</button>
    </div>
  </Modal>
{/if}

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
  .bar {
    flex: none;
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 30px;
  }
  .lbl {
    color: var(--muted);
    font-size: 12px;
  }
  .grow {
    flex: 1;
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
  /* Стикер: квадратная плитка, рисунок вписан целиком, под ним шашечка (видно прозрачность) */
  .tile.stk {
    background: repeating-conic-gradient(#d4d4d8 0 25%, #f4f4f5 0 50%) 0 0 / 14px 14px;
  }
  .tile.stk img {
    object-fit: contain;
    inset: 6px;
    width: calc(100% - 12px);
    height: calc(100% - 12px);
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
