<script lang="ts">
  import Modal from '../../base/modal/Modal.svelte'
  import Seg from '../../base/tpl/Seg.svelte'
  import { plural } from '../../base/words/plural'
  import { NAME_MAX } from '../../panels/decor/decor'
  import { NO_COVER, NO_REPEAT, SAVE_LOOK_HINT, SAVE_LOOK_TITLE, scopeItems } from './savelook'
  import { saveLookUi as f } from './savelook.svelte'

  // «Сохранить альбом как макет…» (131 U3, сц. 12): по образцу «Сохранить разворот…» — поле имени с Enter, «Отмена · Сохранить».
  // Слияние одинаковых разворотов и предел шагов — в Go: отказ заранее (PlanSaveLook) — причиной над кнопками, при записи — тостом.
  const items = $derived(scopeItems(f.selected))

  // Число вне пределов или пусто — в поле остаётся принятое.
  function from(e: Event) {
    const el = e.currentTarget as HTMLInputElement
    f.setFrom(el.valueAsNumber)
    el.value = String(f.from)
  }
  function to(e: Event) {
    const el = e.currentTarget as HTMLInputElement
    f.setTo(el.valueAsNumber)
    el.value = String(f.to)
  }
  // Enter в поле числа сначала принимает набранное (onchange до blur не приходит), потом сохраняет.
  function enter(e: KeyboardEvent, accept?: (e: Event) => void) {
    if (e.key !== 'Enter') return
    e.preventDefault()
    accept?.(e)
    void f.commit()
  }
</script>

{#if f.open}
  <Modal title={SAVE_LOOK_TITLE} wide onclose={() => f.close()}>
    <div class="form">
      <label class="lbl" for="sl-name">Имя</label>
      <input id="sl-name" type="text" maxlength={NAME_MAX} data-autofocus placeholder="Мой макет" bind:value={f.name} onkeydown={enter} />
      <label class="lbl" for="sl-theme">Тема</label>
      <select id="sl-theme" bind:value={f.theme}>
        {#each f.themes as t (t.id)}<option value={t.id}>{t.name}</option>{/each}
      </select>
      <span class="lbl">Развороты</span>
      <div class="what"><Seg {items} value={f.scope} onpick={(s) => f.setScope(s)} label="Какие развороты сохранить" /></div>
      <span class="lbl">Повторяемые</span>
      <div class="rep">
        с
        <input
          type="number"
          min="1"
          max={f.n}
          value={f.from}
          onchange={from}
          onkeydown={(e) => enter(e, from)}
          aria-label="Первый повторяемый разворот"
        />
        по
        <input
          type="number"
          min="0"
          max={f.n}
          value={f.to}
          onchange={to}
          onkeydown={(e) => enter(e, to)}
          aria-label="Последний повторяемый разворот"
        />
        <span class="of">из {f.n} {plural(f.n, ['разворота', 'разворотов', 'разворотов'])}</span>
      </div>
      <span></span>
      <label class="chk"
        ><input type="checkbox" checked={f.places} onchange={(e) => f.setFlags('places', e.currentTarget.checked)} />Места для фото</label
      >
      <span></span>
      <label class="chk"
        ><input type="checkbox" checked={f.drafts} onchange={(e) => f.setFlags('drafts', e.currentTarget.checked)} />Надписи — заготовками</label
      >
      <span></span>
      <label class="chk" title={f.hasCover ? undefined : NO_COVER}>
        <input
          type="checkbox"
          checked={f.cover}
          onchange={(e) => f.setFlags('cover', e.currentTarget.checked)}
          disabled={!f.hasCover}
        />Обложка
      </label>
    </div>
    {#if !f.repeat}<p class="hint">{NO_REPEAT}</p>{/if}
    <p class="hint">{SAVE_LOOK_HINT}</p>
    {#if f.refusal}<p class="warn">{f.refusal}</p>{/if}
    <div class="actions">
      <button class="ghost" onclick={() => f.close()}>Отмена</button>
      <button class="primary" disabled={!f.ready} onclick={() => f.commit()}>Сохранить</button>
    </div>
  </Modal>
{/if}

<style>
  .form {
    display: grid;
    grid-template-columns: 96px 1fr;
    gap: 8px 10px;
    align-items: center;
  }
  .lbl {
    color: var(--muted);
    font-size: 12px;
    text-align: right;
  }
  .what {
    justify-self: start;
  }
  .rep {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 13px;
  }
  .of {
    color: var(--muted);
    font-size: 12px;
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
  input[type='number'] {
    width: 72px;
  }
  select {
    width: 100%;
  }
  .chk {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 13px;
  }
  .hint {
    margin: 8px 0 0;
    color: var(--muted);
    font-size: 12px;
    line-height: 1.35;
  }
  /* Причина серой «Сохранить» — как предупреждение окна «Экспорт» */
  .warn {
    margin: 8px 0 0;
    color: var(--text);
    font-size: 12px;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 12px;
  }
</style>
