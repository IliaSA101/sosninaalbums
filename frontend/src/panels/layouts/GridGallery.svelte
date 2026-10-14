<script lang="ts">
  import { store, Api } from '../../base/core/store.svelte'
  import { gallery } from './gallery.svelte'
  import { errText } from '../../base/core/errtext'
  import { plural } from '../../base/core/plural'
  import { applyChoice, tplCall } from './gridactions'
  import { cardBoxes, defaultN, dropTitle, fits, MAX_N, previewRects } from './gridgallery'
  import type { GridChoice, GridChoices, LayoutScope } from '../../base/core/types'
  import { isSheet, newSheetSize, sheetSize, unitOf } from '../../base/geom/sheet'
  import Modal from '../../base/ui/Modal.svelte'
  import TplRow from '../../base/ui/TplRow.svelte'
  import TplItem from '../../base/ui/TplItem.svelte'

  // Вкладка «Сетки» галереи (спека 007): встроенные сетки на n фото и «Мои шаблоны».
  // Доли boxes от Go — проценты карточки; интерфейс ничего не считает, кроме перевода в мм для предпросмотра.
  const H = 56 // высота карточки
  // g приходит от LayoutGallery: пока галерея уезжает, gallery.cur уже null.
  let { g }: { g: { spread: string; scope: LayoutScope } } = $props()
  const s = $derived(store.view.settings)
  const unit = $derived(unitOf(s))
  const sp = $derived(store.view.spreads.find((x) => x.id === g.spread))
  const scope = $derived(g.scope)
  // Размер листа и шов — из Go (sheet.ts); нет листа — по настройкам альбома.
  const sheet = $derived(sp ? sheetSize(sp) : newSheetSize(s))
  const cardW = $derived((H * (scope === 'spread' ? sheet.w : sheet.mid)) / sheet.h)

  // Фильтр: пока не выбран руками — по умолчанию (число фото или мест в области).
  let nPick = $state<number | null>(null)
  const nWant = $derived(nPick ?? (sp ? defaultN(sp, sheet.mid, scope) : 1))
  $effect(() => {
    void scope // другая область — снова фильтр по умолчанию
    nPick = null
  })

  // Ответ Go; ключ — для какого разворота, фильтра и области он посчитан.
  let c = $state<{ key: string; res: GridChoices } | null>(null)
  let loading = $state(false)
  let again = $state(0) // перезапрос после правки шаблонов
  let req = 0
  const key = $derived(`${g.spread}|${nWant}|${scope}`)
  $effect(() => {
    const k = key
    void [again, store.view.rev]
    const my = ++req
    loading = true
    Api.GridChoices(g.spread, nWant, scope)
      .then((res) => {
        if (my === req) c = { key: k, res: res as unknown as GridChoices }
      })
      .catch((e) => {
        if (my === req) store.notify(errText(e), 'error')
      })
      .finally(() => {
        if (my === req) loading = false
      })
  })
  const res = $derived(c && c.key === key ? c.res : null)
  const builtin = $derived(scope === 'spread' ? (res?.builtin ?? []) : [])
  const mine = $derived((res?.mine ?? []).filter((x) => fits(x, scope)))

  // У листа виньетки страниц нет (Go отказывает шаблонам по страницам): область одна, выбор прячется.
  const pages = $derived<{ scope: LayoutScope; label: string }[]>(
    isSheet(s)
      ? [{ scope: 'spread', label: 'Лист' }]
      : [
          { scope: 'spread', label: 'Разворот' },
          { scope: 'left', label: 'Левая' },
          { scope: 'right', label: 'Правая' },
        ],
  )
  function setScope(next: LayoutScope) {
    if (next !== scope) gallery.open(next, 'grids')
  }

  function hover(ch: GridChoice | null) {
    gallery.previewChoice(ch ? { key: ch.id, rects: previewRects(ch, scope, sheet) } : null)
  }
  function pick(ch: GridChoice) {
    const { spread } = g
    gallery.close()
    void applyChoice(spread, scope, ch)
  }

  // ---------- шаблоны: переименовать, удалить, сохранить ----------
  // Имена в полях — отдельное состояние: окно закрывается (rename/save = null) раньше, чем bind:value
  // отпустит поле, и привязка к свойству null падала.
  let rename = $state<GridChoice | null>(null)
  let renameName = $state('')
  let del = $state<GridChoice | null>(null)
  let save = $state<{ scope: LayoutScope } | null>(null)
  let saveName = $state('')

  async function doRename() {
    const r = rename
    if (!r) return
    rename = null
    if (await tplCall(Api.RenameGridTemplate(r.id, renameName))) again++
  }
  async function doDelete() {
    const d = del
    if (!d) return
    del = null
    if (await tplCall(Api.DeleteGridTemplate(d.id))) again++
  }
  async function doSave() {
    const v = save
    if (!v) return
    save = null
    gallery.preview = null
    if (await tplCall(Api.SaveGridTemplate(g.spread, v.scope, saveName.trim()))) {
      store.notify('Шаблон сохранён')
      again++
    }
  }
  // Enter в поле имени — как кнопка окна.
  const enter = (fn: () => void) => (e: KeyboardEvent) => {
    if (e.key === 'Enter') {
      e.preventDefault()
      fn()
    }
  }
</script>

<div class="bar">
  {#if pages.length > 1}
    <div class="seg" role="group" aria-label="Область">
      {#each pages as p (p.scope)}
        <button class:on={scope === p.scope} onclick={() => setScope(p.scope)}>{p.label}</button>
      {/each}
    </div>
  {/if}
  <div class="seg" role="group" aria-label="Число мест">
    {#each Array.from({ length: MAX_N }, (_, i) => i + 1) as n (n)}
      <button
        class:on={nWant === n}
        onclick={() => (nPick = n)}
        title={n === MAX_N ? 'Шаблоны на 7 мест и больше' : `Шаблоны на ${n} ${plural(n, ['место', 'места', 'мест'])}`}
        >{n === MAX_N ? '7+' : n}</button
      >
    {/each}
  </div>
  {#if loading}<span class="busy">Считаю…</span>{/if}
  <button
    class="text"
    onclick={() => ((saveName = ''), (save = { scope }))}
    title={isSheet(s) ? 'Запомнить места этого листа как шаблон' : 'Запомнить места этого разворота или страницы как шаблон'}
    >Сохранить как шаблон…</button
  >
</div>

{#snippet row(label: string, list: GridChoice[], empty: string, own: boolean)}
  <TplRow {label} count={list.length} empty={loading && !res ? '' : empty} tall={own && list.length > 0} inert={loading}>
    {#each list as ch (ch.id)}
      <TplItem
        name={own ? ch.name : undefined}
        onrename={own ? () => ((rename = ch), (renameName = ch.name), hover(null)) : undefined}
        ondelete={own ? () => ((del = ch), hover(null)) : undefined}
      >
        <button
          class="card"
          class:dim={loading}
          style:width="{cardW}px"
          style:height="{H}px"
          title={ch.name || undefined}
          onmouseenter={() => hover(ch)}
          onmouseleave={() => hover(null)}
          onclick={() => pick(ch)}
        >
          {#each cardBoxes(ch, scope) as r, i (i)}
            <span
              class="cell"
              style:left="{r.x * 100}%"
              style:top="{r.y * 100}%"
              style:width="{r.w * 100}%"
              style:height="{r.h * 100}%"
              style:transform={r.rot ? `rotate(${r.rot}deg)` : undefined}
            ></span>
          {/each}
          <span class="spine"></span>
          {#if ch.drop > 0}<span class="drop" title={dropTitle(ch.drop)}>−{ch.drop}</span>{/if}
        </button>
      </TplItem>
    {/each}
  </TplRow>
{/snippet}

{@render row(
  'Встроенные',
  builtin,
  scope === 'spread' ? 'Нет шаблонов на такое число мест' : `Встроенные шаблоны ставятся только на весь ${unit.one}`,
  false,
)}
{@render row(
  'Мои шаблоны',
  mine,
  `Нет своих шаблонов на ${nWant === MAX_N ? '7 мест и больше' : `${nWant} ${plural(nWant, ['место', 'места', 'мест'])}`}`,
  true,
)}

{#if rename}
  <Modal title="Переименовать шаблон" onclose={() => (rename = null)}>
    <input type="text" maxlength="60" data-autofocus bind:value={renameName} onkeydown={enter(doRename)} aria-label="Имя шаблона" />
    <div class="actions">
      <button class="ghost" onclick={() => (rename = null)}>Отмена</button>
      <button class="primary" onclick={doRename}>Сохранить</button>
    </div>
  </Modal>
{/if}

{#if del}
  <Modal title="Удалить шаблон?" onclose={() => (del = null)}>
    <p>Шаблон «{del.name}» будет удалён из списка. {unit.cap === 'Лист' ? 'Листы' : 'Развороты'}, где он уже применён, не изменятся.</p>
    <div class="actions">
      <button class="ghost" onclick={() => (del = null)}>Отмена</button>
      <button class="primary" onclick={doDelete}>Удалить</button>
    </div>
  </Modal>
{/if}

{#if save}
  <Modal title="Сохранить как шаблон" onclose={() => (save = null)}>
    <label class="field"
      >Название
      <input type="text" maxlength="60" data-autofocus placeholder="Шаблон N" bind:value={saveName} onkeydown={enter(doSave)} /></label
    >
    {#if pages.length > 1}
      <div class="field">
        Область
        <div class="seg" role="group" aria-label="Область шаблона">
          {#each pages as p (p.scope)}
            <button class:on={save.scope === p.scope} onclick={() => save && (save.scope = p.scope)}>{p.label}</button>
          {/each}
        </div>
      </div>
    {/if}
    <p class="hint">
      Запоминаются только места, без фото.{pages.length > 1 ? ' «Левая» и «Правая» дают шаблон страницы.' : ''}
    </p>
    <div class="actions">
      <button class="ghost" onclick={() => (save = null)}>Отмена</button>
      <button class="primary" onclick={doSave}>Сохранить</button>
    </div>
  </Modal>
{/if}

<style>
  .bar {
    display: flex;
    align-items: center;
    gap: 14px;
    height: 32px;
  }
  .seg {
    display: flex;
    gap: 1px;
    padding: 2px;
    background: #1b1b1d;
    border-radius: 7px;
  }
  .seg button {
    height: 24px;
    min-width: 28px;
    padding: 0 8px;
    font-size: 12px;
    background: transparent;
    border-radius: 5px;
  }
  .seg button.on {
    background: var(--accent);
    color: #1e1e1e;
    font-weight: 600;
  }
  .busy {
    color: var(--muted);
    font-size: 12px;
  }
  .bar > .text {
    margin-left: auto;
    height: 26px;
    font-size: 12px;
    padding: 0 10px;
  }
  .card {
    all: unset;
    display: block;
    position: relative;
    overflow: clip; /* повёрнутая коробка у края не вылезает из карточки */
    background: #fff;
    cursor: pointer;
    outline: 2px solid transparent;
    outline-offset: 2px;
    transition:
      outline-color 0.15s,
      opacity 0.15s;
  }
  .card:hover {
    outline-color: var(--accent);
  }
  .card.dim {
    opacity: 0.45;
  }
  .cell {
    position: absolute;
    background: #d8d8d8;
    box-shadow: inset 0 0 0 1px #a8a8a8;
  }
  .spine {
    position: absolute;
    left: 50%;
    top: 0;
    bottom: 0;
    width: 1px;
    background: rgba(0, 0, 0, 0.25);
  }
  /* «−N»: столько фото уйдёт в ленту при применении */
  .drop {
    position: absolute;
    right: 2px;
    bottom: 2px;
    padding: 0 4px;
    border-radius: 3px;
    background: #c0392b;
    color: #fff;
    font-size: 11px;
    font-weight: 700;
    line-height: 16px;
  }
  /* Окна */
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
  .field {
    display: flex;
    flex-direction: column;
    gap: 4px;
    margin-bottom: 10px;
    color: var(--muted);
    font-size: 12px;
  }
  .field .seg {
    align-self: flex-start;
  }
  p {
    margin: 0 0 10px;
    line-height: 1.4;
  }
  .hint {
    color: var(--muted);
    font-size: 12px;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 12px;
  }
</style>
