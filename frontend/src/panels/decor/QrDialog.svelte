<script lang="ts">
  import Modal from '../../base/modal/Modal.svelte'
  import { decor } from './decor.svelte'
  import { READ_ONLY_HINT } from './decor'
  import { QR_TITLE, qrLines } from './qr'
  import { qrForm as f } from './qr.svelte'

  // «QR-код по ссылке…» (100 п. 2): по образцу «Сохранить разворот…» — поле с Enter, «Отмена · Поставить».
  // Справа живой образец от Go: тёмные модули одним контуром, белое поле по краю; края резкие при любом размере.
  const lines = $derived(qrLines(f.info, f.link))
  const readOnly = $derived(!!decor.cat?.readOnly)

  // Окно «Стикеры» закрылось (Esc, смена альбома) — форма вместе с ним, чтобы не всплыть при новом открытии.
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
  <Modal title={QR_TITLE} wide onclose={() => f.close()}>
    <div class="form">
      <div class="col">
        <label class="lbl" for="qr-link">Ссылка</label>
        <input
          id="qr-link"
          type="text"
          data-autofocus
          placeholder="site.ru/album"
          spellcheck="false"
          value={f.link}
          oninput={(e) => f.input(e.currentTarget.value)}
          onkeydown={enter}
        />
        {#if f.error}<p class="err" role="alert">{f.error}</p>{/if}
        {#each lines as l (l)}<p class="hint">{l}</p>{/each}
      </div>
      <div class="sample" aria-label="Образец кода">
        {#if f.info && !f.error}
          <svg
            viewBox="0 0 {f.info.side} {f.info.side}"
            width="160"
            height="160"
            shape-rendering="crispEdges"
            role="img"
            aria-label="QR-код"
          >
            <rect width={f.info.side} height={f.info.side} fill="#fff" />
            <path d={f.info.path} fill="#000" />
          </svg>
        {/if}
      </div>
    </div>
    <div class="actions">
      <button class="ghost" onclick={() => f.close()}>Отмена</button>
      <button class="primary" disabled={!f.canPut} title={readOnly ? READ_ONLY_HINT : undefined} onclick={() => f.commit()}
        >Поставить</button
      >
    </div>
  </Modal>
{/if}

<style>
  .form {
    display: flex;
    gap: 14px;
    align-items: flex-start;
  }
  .col {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .lbl {
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
  .sample {
    flex: none;
    width: 160px;
    height: 160px;
    background: #2a2a2d;
    border-radius: 4px;
    overflow: hidden;
  }
  svg {
    display: block;
  }
  .hint,
  .err {
    margin: 0;
    font-size: 12px;
    line-height: 1.35;
    overflow-wrap: anywhere;
  }
  .hint {
    color: var(--muted);
  }
  .err {
    color: #e5645a;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 12px;
  }
</style>
