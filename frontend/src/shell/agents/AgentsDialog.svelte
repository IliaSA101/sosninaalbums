<script lang="ts">
  import Modal from '../../base/modal/Modal.svelte'
  import { agents } from './agents.svelte'
  import { callLine, hasNames, hasPics } from './agents'
  import {
    AGENTS_TITLE,
    ASK_NAMES,
    ASK_ON,
    ASK_PICS,
    CANCEL_LABEL,
    CLIENTS_NONE,
    CLOSE_LABEL,
    COPY_LABEL,
    DISABLE_HINT,
    DISABLE_LABEL,
    ENABLE_LABEL,
    LOG_CLEAR,
    LOG_CLEAR_HINT,
    LOG_EMPTY,
    LOG_TITLE,
    OFF_TEXT,
    ON_TEXT,
    PORTABLE_NOTE,
    SAVE_FILE_HINT,
    SAVE_FILE_LABEL,
    SEE_BASE,
    SEE_NAMES,
    SEE_PICS,
    SEE_TITLE,
    SETUP_NOTE,
    SETUP_TITLE,
    clientsLine,
    todayLine,
  } from './agentstext'

  // Окно «Файл ▸ ИИ-агенты (MCP)…» (143, У1). Образец — «Наборы библиотеки» (067): вопрос перед первым выходом наружу
  // («Отмена» слева, действие справа), флажок, строка состояния с «Выключить», «Закрыть» внизу.
  // Все решения (что видно на уровне, тексты настройки, пути) — у Go; здесь только рисуем и зовём.
  // Тексты клиентов, имена агентов и журнал — чужой/недоверенный текст: только как текст (не {@html}).
  $effect(() => agents.watch())

  const ASKS = { on: ASK_ON, names: ASK_NAMES, pics: ASK_PICS }
  const ask = $derived(agents.ask ? ASKS[agents.ask] : null)
  const info = $derived(agents.info)
  const names = $derived(agents.names)

  // Раздел «Как подключить»: свёрнут; предупреждение (hint) раскрывает его. Состояние своё, не `open={…}`: Svelte переписал бы
  // атрибут при каждом событии Go и захлопнул раскрытое пользователем.
  let setupOpen = $state(false)
  const hinted = $derived(!!info.hint)
  $effect(() => {
    if (hinted) setupOpen = true
  })

  // Состояние флажка — у Go: до ответа (или вопроса) вид прежний.
  function flag(e: Event & { currentTarget: HTMLInputElement }, now: boolean, act: (on: boolean) => void) {
    const on = e.currentTarget.checked
    e.currentTarget.checked = now
    act(on)
  }
</script>

{#if agents.open}
  <Modal title={AGENTS_TITLE} onclose={() => agents.close()} width={620}>
    <div class="wrap">
      <div class="scroll">
        <!-- Состояние: выключено — кнопка «Разрешить…», включено — «Включено» и «Выключить», ошибка — «Разрешить…» и «Выключить» -->
        {#if agents.on}
          <div class="state" role="status">
            <b>{ON_TEXT}</b>
            <span class="grow"></span>
            <button disabled={agents.busy} title={DISABLE_HINT} onclick={() => agents.disable()}>{DISABLE_LABEL}</button>
          </div>
        {:else}
          <p class="note">{OFF_TEXT}</p>
          {#if info.status === 'error' && info.error}<p class="err" role="alert">{info.error}</p>{/if}
          <!-- Ошибка запуска при включённом доступе: «Выключить» снимает его в настройке (Go не меняет её молча) -->
          <div class="state">
            <button disabled={agents.busy} onclick={() => agents.want('on')}>{ENABLE_LABEL}</button>
            {#if info.status === 'error'}
              <span class="grow"></span>
              <button disabled={agents.busy} title={DISABLE_HINT} onclick={() => agents.disable()}>{DISABLE_LABEL}</button>
            {/if}
          </div>
        {/if}

        {#if agents.on}
          <h3>{SEE_TITLE}</h3>
          <p class="note">{SEE_BASE}</p>
          <label class="chk">
            <input
              type="checkbox"
              checked={hasNames(info.level)}
              disabled={agents.busy}
              onchange={(e) => flag(e, hasNames(info.level), (on) => agents.toggleNames(on))}
            />
            {SEE_NAMES}
          </label>
          <label class="chk">
            <input
              type="checkbox"
              checked={hasPics(info.level)}
              disabled={agents.busy}
              onchange={(e) => flag(e, hasPics(info.level), (on) => agents.togglePics(on))}
            />
            {SEE_PICS}
          </label>

          <!-- Настройка клиентов нужна один раз: свёрнута, чтобы журнал был виден без прокрутки (образец — «Дополнительно» в автосборке).
               Предупреждение (путь к программе устарел) разворачивает её сразу: оно про подключение -->
          <details class="setup" bind:open={setupOpen}>
            <summary>{SETUP_TITLE}</summary>
            {#if info.hint}<p class="hint" role="note">{info.hint}</p>{/if}
            {#if info.setups.length > 0}
              <p class="note">{SETUP_NOTE}</p>
              <ul class="setups" aria-label={SETUP_TITLE}>
                {#each info.setups as s (s.id)}
                  <li>
                    <div class="head">
                      <b>{s.title}</b>{#if s.note}<span class="meta">{s.note}</span>{/if}
                    </div>
                    <pre class="code">{s.text}</pre>
                    <div class="row">
                      <button class="ghost" onclick={() => agents.copy(s)}>{COPY_LABEL}</button>
                      {#if s.file}
                        <button class="ghost" title={SAVE_FILE_HINT} onclick={() => agents.saveFile()}>{SAVE_FILE_LABEL}</button>
                      {/if}
                    </div>
                  </li>
                {/each}
              </ul>
              {#if info.portable}<p class="note">{PORTABLE_NOTE}</p>{/if}
            {/if}
          </details>

          <p class="clients">{names.length > 0 ? clientsLine(names) : CLIENTS_NONE}</p>

          <h3>{LOG_TITLE}</h3>
          {#if info.calls.length === 0}
            <p class="note">{LOG_EMPTY}</p>
          {:else}
            <ul class="log" aria-label={LOG_TITLE}>
              {#each info.calls as c, i (i)}
                <li class:bad={!c.ok}>{callLine(c)}</li>
              {/each}
            </ul>
          {/if}
          <div class="row">
            <span class="meta">{todayLine(info.today)}</span>
            <span class="grow"></span>
            <button class="link" disabled={agents.busy || info.calls.length === 0} title={LOG_CLEAR_HINT} onclick={() => agents.clearLog()}
              >{LOG_CLEAR}</button
            >
          </div>
        {/if}
      </div>
      <footer>
        <button data-autofocus onclick={() => agents.close()}>{CLOSE_LABEL}</button>
      </footer>
    </div>
  </Modal>
{/if}

{#if agents.open && ask}
  <!-- Вопрос перед включением доступа или уровня: «Отмена» слева, действие справа -->
  <Modal title={ask.title} onclose={() => agents.dismiss()} wide>
    <p class="body">{ask.text}</p>
    <div class="actions">
      <button class="ghost" onclick={() => agents.dismiss()}>{CANCEL_LABEL}</button>
      <button class="primary" data-autofocus onclick={() => agents.confirm()}>{ask.ok}</button>
    </div>
  </Modal>
{/if}

<style>
  /* Колонка окна: прокручивается содержимое, «Закрыть» на виду. 100px — шапка и поля Modal */
  .wrap {
    display: flex;
    flex-direction: column;
    max-height: calc(100vh - 100px);
  }
  .scroll {
    flex: 1 1 auto;
    min-height: 0;
    overflow: auto;
    margin: 0 0 10px;
  }
  footer {
    flex: none;
    display: flex;
    justify-content: flex-end;
  }
  h3 {
    margin: 12px 0 6px;
    font-size: 13px;
    font-weight: 600;
  }
  p {
    margin: 0 0 10px;
    line-height: 1.4;
  }
  .body {
    margin: 0 0 10px;
  }
  .note {
    color: var(--muted);
    font-size: 12px;
  }
  .err {
    color: #ff8a8a;
  }
  .setup {
    margin: 12px 0 8px;
  }
  .setup summary {
    cursor: pointer;
    font-size: 13px;
    font-weight: 600;
  }
  .setup[open] summary {
    margin-bottom: 6px;
  }
  .hint {
    padding: 6px 8px;
    border: 1px solid var(--accent);
    border-radius: 6px;
    font-size: 12px;
  }
  .state {
    display: flex;
    align-items: center;
    gap: 8px;
    margin: 0 0 6px;
  }
  .grow {
    flex: 1;
  }
  .chk {
    display: flex;
    align-items: center;
    gap: 6px;
    margin: 0 0 6px;
  }
  .setups,
  .log {
    list-style: none;
    margin: 0 0 10px;
    padding: 0;
  }
  .setups li {
    margin: 0 0 10px;
    padding: 8px 10px;
    border: 1px solid var(--line);
    border-radius: 6px;
  }
  .head {
    display: flex;
    align-items: baseline;
    gap: 8px;
    margin: 0 0 4px;
  }
  .meta {
    color: var(--muted);
    font-size: 12px;
    overflow-wrap: anywhere;
  }
  .code {
    margin: 0 0 6px;
    padding: 6px 8px;
    background: var(--panel2);
    border-radius: 4px;
    font-family: Consolas, monospace;
    font-size: 12px;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    user-select: text;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .clients {
    margin: 0 0 4px;
    color: var(--muted);
    font-size: 12px;
    overflow-wrap: anywhere;
  }
  .log {
    max-height: 150px;
    overflow: auto;
    font-size: 12px;
    color: var(--muted);
    user-select: text;
  }
  .log li {
    padding: 2px 0;
    overflow-wrap: anywhere;
  }
  .log li.bad {
    color: #ff8a8a;
  }
  /* Ссылка-кнопка: как «Не обращаться к сети» в окне «Наборы» */
  .link {
    all: unset;
    cursor: pointer;
    color: var(--muted);
    font-size: 12px;
    text-decoration: underline dotted;
    text-underline-offset: 3px;
  }
  .link:hover,
  .link:focus-visible {
    text-decoration-style: solid;
  }
  .link:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }
  .link:disabled {
    cursor: default;
    opacity: 0.5;
    text-decoration: none;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 12px;
  }
</style>
