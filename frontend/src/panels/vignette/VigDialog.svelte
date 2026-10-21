<script lang="ts">
  import { untrack } from 'svelte'
  import { store } from '../../base/core/store.svelte'
  import { shapeKey } from '../../base/geom/shape'
  import { sheetIn } from '../../base/geom/sheet'
  import { modes } from '../../base/prefs/view.svelte'
  import Modal from '../../base/modal/Modal.svelte'
  import Spread from '../../editor/spread/Spread.svelte'
  import VigStrip from './VigStrip.svelte'
  import { VigSession, vigDialog } from './vigsession.svelte'
  import { vigOwn } from './vigown.svelte'
  import { ownLabel } from './vigown'
  import { FORMS, FORM_FILTERS, curKey, pickTile, vigTiles, type Tile } from './vigform'
  import Seg from '../../base/tpl/Seg.svelte'
  import { activeFamily, familiesIn, type VigFamilyFilter } from '../../base/tpl/tplfilter'
  import { PREVIEW_RESERVE, VAR_FIELDS, VAR_MAX, leftLine, placesLine, previewBox, resetWarning, stripMax } from './vigdialog'
  import { MAX_PUPILS, MAX_TEACHERS, RATIOS, clampCount, isVignette, portraitText } from './vignette'

  // Окно «Настроить виньетку» (031 dialog §3): большой предпросмотр листа, сетка шаблонов, числа, подпись учителя
  // и поля шапки. Предпросмотр считает Go по каждому изменению формы; в проект до «Применить» ничего не пишется.
  // Каждое открытие — новая сессия: форма берётся из листа, правки листа извне её не сбрасывают.
  let ses = $state<VigSession | null>(null)

  $effect(() => {
    const id = vigDialog.id
    untrack(() => {
      const sp = id ? sheetIn(store.view, id) : undefined
      if (id && isVignette(sp)) ses = new VigSession(id, sp)
      else {
        ses = null
        if (id) vigDialog.close() // лист пропал или уже не виньетка
      }
    })
  })

  // Лист исчез, пока окно открыто (удалён, «Обычный разворот» мимо окна): закрываем.
  $effect(() => {
    const s = ses
    void store.view.rev
    if (s && !isVignette(sheetIn(store.view, s.id))) untrack(() => vigDialog.close())
  })

  // Предпросмотр: перезапрос при каждом изменении формы, полей шапки и ревизии (Undo из меню и т. п.).
  $effect(() => {
    void store.view.rev
    ses?.refreshPreview()
  })

  // Карточки шаблонов — отдельно: читают только числа и пропорцию, буквы «Заголовка» их не трогают.
  // Свои шаблоны переименовали или удалили (vigOwn.rev) — список перезапрашивается: событий у Go нет.
  $effect(() => {
    void store.view.rev
    void vigOwn.rev
    ses?.refreshChoices()
  })

  // Удалили × выбранную карточку — выбор возвращается к шаблону листа (после ответа Go).
  $effect(() => {
    void ses?.choices.value
    untrack(() => ses?.fixTpl())
  })

  const close = () => vigDialog.close()
  async function apply() {
    if (await ses?.apply()) close()
  }

  let colW = $state(560)
  let winH = $state(760)
  const sheet = $derived(ses ? sheetIn(store.view, ses.id) : undefined)

  // Фильтры вида (049) и формы (056): сетка и стрелки в ней идут по видимым плиткам; выбранная плитка видна всегда,
  // вид свои шаблоны не фильтрует. Сами фильтры только отбирают — форму мест выбирает щелчок по плитке.
  const families = $derived(familiesIn(ses?.items ?? []))
  const fam = $derived(activeFamily(ses?.family ?? 'all', families))
  const famItems = $derived<{ id: VigFamilyFilter; label: string }[]>([{ id: 'all', label: 'Все' }, ...families])
  const tiles = $derived(ses ? vigTiles(ses.items, fam, ses.formFilter, ses.form) : [])
  const hasCatalog = $derived((ses?.items ?? []).some((c) => !c.own))
  const filterRows = $derived((families.length > 0 ? 1 : 0) + (hasCatalog ? 1 : 0))
  // Ручная виньетка: «Применить» пересоберёт места и сбросит ручные — предупреждаем до нажатия
  const resetWarn = $derived(resetWarning(isVignette(sheet) && sheet.vig.manual, ses?.unchanged ?? true))
  const shown = $derived(ses?.preview.value?.spread ?? sheet)
  const box = $derived(sheet ? previewBox(sheet.w, sheet.h, colW, winH, PREVIEW_RESERVE) : { w: 0, h: 0 })

  // Число зажимается, и зажатое значение пишется обратно в поле: форма при том же числе не меняется.
  // Пока поле пустое, его не трогаем (набор), на выходе — всегда.
  const num =
    (set: (n: number) => void, cap: number, final = false) =>
    (e: Event) => {
      const el = e.currentTarget as HTMLInputElement
      const n = clampCount(el.value, cap)
      set(n)
      if ((final || el.value !== '') && el.value !== String(n)) el.value = String(n)
    }

  // Enter — «Применить» отовсюду, кроме подписи и кнопок: карточки шаблонов ловят его сами.
  function key(e: KeyboardEvent) {
    const t = e.target as HTMLElement
    if (e.key === 'Enter' && !t.closest('textarea, button, summary')) {
      e.preventDefault()
      void apply()
    }
  }

  // Плитка ставит шаблон и свою форму (скруглённые — 10 %); свой шаблон — ещё свой портрет, каталожная портрет не трогает.
  function pick(t: Tile) {
    if (ses) Object.assign(ses.form, pickTile(t, ses.form.ratio))
  }
</script>

<svelte:window bind:innerHeight={winH} />

{#if ses}
  {@const s = ses}
  <Modal title="Настроить виньетку" size="large" onclose={close} undo>
    <div class="body" role="presentation" onkeydown={key}>
      <div class="grid">
        <div class="left" bind:clientWidth={colW}>
          <div class="pv" style:height="{box.h}px" aria-busy={s.preview.pending} aria-label="Предпросмотр листа" role="img">
            {#if shown && box.w > 0}
              <Spread spread={shown} settings={store.view.settings} width={box.w} mini />
            {/if}
          </div>

          {#if families.length > 0}
            <div class="fam"><Seg items={famItems} value={fam} onpick={(f) => (s.family = f)} label="Вид шаблонов" /></div>
          {/if}
          {#if hasCatalog}
            <div class="fam"><Seg items={FORM_FILTERS} value={s.formFilter} onpick={(f) => (s.formFilter = f)} label="Форма мест" /></div>
          {/if}
          <VigStrip
            {tiles}
            cur={curKey(tiles, s.form)}
            sheetW={sheet?.w ?? 1}
            sheetH={sheet?.h ?? 1}
            maxH={stripMax(winH, box.h, filterRows)}
            onpick={pick}
            onapply={apply}
          />
        </div>

        <div class="right">
          <div class="nums">
            <label>
              Учителей
              <input
                type="number"
                min="0"
                max={MAX_TEACHERS}
                value={s.form.teachers}
                oninput={num((n) => (s.form.teachers = n), MAX_TEACHERS)}
                onchange={num((n) => (s.form.teachers = n), MAX_TEACHERS, true)}
              />
            </label>
            <label>
              Учеников
              <input
                type="number"
                min="0"
                max={MAX_PUPILS}
                value={s.form.pupils}
                data-autofocus
                oninput={num((n) => (s.form.pupils = n), MAX_PUPILS)}
                onchange={num((n) => (s.form.pupils = n), MAX_PUPILS, true)}
              />
            </label>
          </div>
          <div class="row">
            Портрет
            <div class="seg" role="group" aria-label="Пропорция портрета">
              {#each RATIOS as r (r)}
                <button type="button" class:on={s.form.ratio === r} aria-pressed={s.form.ratio === r} onclick={() => (s.form.ratio = r)}
                  >{r}</button
                >
              {/each}
            </div>
          </div>
          <label class="col">
            Подпись учителя
            <textarea rows="2" maxlength="200" placeholder={'{Фамилия}\n{Имя} {Отчество}'} bind:value={s.form.ttext}></textarea>
          </label>

          <div class="sect">Шапка</div>
          <div class="head">
            {#each VAR_FIELDS as f (f.k)}
              <label class="col" class:half={f.k === 'year' || f.k === 'lead'}>
                {f.label}
                <input type="text" maxlength={VAR_MAX} placeholder={f.hint} bind:value={s.vars[f.k]} />
              </label>
            {/each}
          </div>

          <div class="about" role="status">
            {#if s.now}
              <span>{ownLabel(s.now)}</span> · <span>{FORMS[shapeKey(s.form)].name}</span> ·
              <span class:small={s.now.small}>{portraitText(s.now)}</span>
            {/if}
            {#if s.preview.error && s.total <= s.max}<div class="err">{s.preview.error}</div>{/if}
          </div>
          {#if resetWarn}<div class="warn" role="status">{resetWarn}</div>{/if}
          <p class="gaps">
            Промежуток и поля — в <button
              type="button"
              class="link"
              onclick={() => {
                close()
                modes.openSection('settings')
              }}>настройках альбома</button
            >
          </p>
        </div>
      </div>

      <div class="bottom">
        <div class="info" role="status">
          <span class:bad={s.total > s.max}>{placesLine(s.total, s.max)}</span>
          {#if s.preview.value && s.preview.value.left > 0}<span class="left-ph">{leftLine(s.preview.value.left)}</span>{/if}
        </div>
        <div class="actions">
          <button type="button" class="ghost" onclick={close}>Отмена</button>
          <button type="button" class="primary" disabled={!!s.block} title={s.block ?? 'Применить: один шаг отмены'} onclick={apply}
            >Применить</button
          >
        </div>
      </div>
    </div>
  </Modal>
{/if}

<style>
  .body {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .grid {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 300px;
    gap: 20px;
    align-items: start;
  }
  .left {
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  /* Сегменты видов — по своей ширине, не на всю колонку */
  .fam {
    align-self: flex-start;
    max-width: 100%;
    overflow-x: auto;
    scrollbar-width: thin;
  }
  .pv {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 120px;
    background: var(--panel2);
    border-radius: 6px;
  }
  .right {
    display: flex;
    flex-direction: column;
    gap: 8px;
    min-width: 0;
  }
  .nums {
    display: grid;
    gap: 8px;
  }
  label,
  .row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 8px;
    font-size: 13px;
  }
  label.col {
    flex-direction: column;
    align-items: stretch;
    gap: 4px;
  }
  input[type='number'] {
    width: 90px;
  }
  input[type='text'] {
    width: 100%;
    box-sizing: border-box;
    min-width: 0;
  }
  textarea {
    box-sizing: border-box;
    width: 100%;
    min-width: 0;
    resize: none;
    padding: 6px 8px;
    font: inherit;
    font-size: 12px;
    background: #3a3a3e;
    color: var(--text);
    border: 1px solid var(--line);
    border-radius: 6px;
  }
  .seg {
    flex: 1 1 auto;
    min-width: 0;
    display: flex;
    gap: 1px;
    padding: 2px;
    background: #1b1b1d;
    border-radius: 7px;
  }
  .seg button {
    flex: 1 1 0;
    height: 24px;
    min-width: 0;
    padding: 0 6px;
    font-size: 12px;
    background: transparent;
    border-radius: 5px;
  }
  .seg button.on {
    background: var(--accent);
    color: #1e1e1e;
    font-weight: 600;
  }
  .sect {
    margin-top: 6px;
    padding-top: 8px;
    border-top: 1px solid var(--line);
    font-size: 13px;
    font-weight: 600;
  }
  /* Заголовок и подзаголовок — на всю ширину, год и руководитель — в одну строку */
  .head {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }
  .head label {
    flex: 1 1 100%;
  }
  .head label.half {
    flex: 1 1 calc(50% - 4px);
    min-width: 0;
  }
  .about {
    min-height: 16px;
    font-size: 12px;
    color: var(--muted);
  }
  .about .small {
    color: var(--accent);
  }
  .err {
    margin-top: 2px;
    color: #e0806f;
  }
  .warn {
    font-size: 12px;
    color: var(--accent);
  }
  .gaps {
    margin: 0;
    font-size: 12px;
    color: var(--muted);
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
  /* Низ всегда на виду: при низком окне прокручивается содержимое, а не он */
  .bottom {
    position: sticky;
    bottom: -14px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    margin: 0 -16px -14px;
    padding: 10px 16px 14px;
    background: var(--panel);
    border-top: 1px solid var(--line);
  }
  .info {
    display: flex;
    flex-wrap: wrap;
    gap: 4px 14px;
    font-size: 13px;
  }
  .info .bad {
    color: #e0806f;
  }
  .info .left-ph {
    color: var(--accent);
  }
  .actions {
    display: flex;
    gap: 8px;
  }
  @media (max-width: 760px) {
    .grid {
      grid-template-columns: minmax(0, 1fr);
    }
  }
</style>
