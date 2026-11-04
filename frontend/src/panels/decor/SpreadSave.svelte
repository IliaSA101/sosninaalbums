<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import { BORDER_NOTE, hasBorders, sheetIn } from '../../base/geom/sheet'
  import Modal from '../../base/modal/Modal.svelte'
  import Seg, { type SegItem } from '../../base/tpl/Seg.svelte'
  import type { BackdropScope } from '../../base/core/types'
  import { NAME_MAX } from './decor'
  import { decor } from './decor.svelte'
  import { SAVE_HINT, SAVE_TITLE, saveThemes } from './spreadtpl'
  import { spreadSave as f } from './tplsave.svelte'

  // «Сохранить разворот…» (067): по образцу «Сохранить как шаблон» — поле имени с Enter, «Отмена · Сохранить».
  // scopes — «Что» из окна «Развороты»: один пункт (лист без шва) — выбор прячется, как «Куда».
  let { scopes }: { scopes: SegItem<BackdropScope>[] } = $props()

  const themes = $derived(saveThemes(decor.cat))
  const empty = $derived(f.name.trim() === '')

  // Окно разворотов закрылось (Esc, смена альбома) — форма вместе с ним, чтобы не всплыть при новом открытии.
  $effect(() => {
    if (!decor.cur) f.close()
  })

  function enter(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      e.preventDefault()
      void f.commit()
    }
  }
</script>

{#if f.open && decor.cur}
  <Modal title={SAVE_TITLE} wide onclose={() => f.close()}>
    <div class="form">
      <label class="lbl" for="sp-save-name">Имя</label>
      <input
        id="sp-save-name"
        type="text"
        maxlength={NAME_MAX}
        data-autofocus
        placeholder="Разворот N"
        bind:value={f.name}
        onkeydown={enter}
      />
      {#if scopes.length > 1}
        <span class="lbl">Что</span>
        <div class="what"><Seg items={scopes} value={f.scope} onpick={(s) => (f.scope = s)} label="Что сохранить" /></div>
      {/if}
      <label class="lbl" for="sp-save-theme">Тема</label>
      <select id="sp-save-theme" bind:value={f.theme}>
        {#each themes as t (t.id)}<option value={t.id}>{t.name}</option>{/each}
      </select>
      <span></span>
      <label class="chk"><input type="checkbox" bind:checked={f.places} />Места для фото</label>
      <span></span>
      <label class="chk"><input type="checkbox" bind:checked={f.texts} />Надписи</label>
    </div>
    <p class="hint">{SAVE_HINT}</p>
    {#if f.places && hasBorders(sheetIn(store.view, decor.cur.spread), f.scope)}<p class="hint">{BORDER_NOTE}</p>{/if}
    <div class="actions">
      <button class="ghost" onclick={() => f.close()}>Отмена</button>
      <button class="primary" disabled={empty || f.busy} onclick={() => f.commit()}>Сохранить</button>
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
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 12px;
  }
</style>
