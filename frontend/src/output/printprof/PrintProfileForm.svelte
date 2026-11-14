<script lang="ts">
  import { untrack } from 'svelte'
  import Seg from '../../base/tpl/Seg.svelte'
  import PrintFormats from './PrintFormats.svelte'
  import { printEdit as ui } from './printedit.svelte'
  import { printList } from './printlist.svelte'
  import { BASE_HINT, BASE_NONE, NO_DPI_LINE, baseGroups, fieldErr, formErr, isHitLine, namesLine } from './printwords'

  // Поля окна «Своя типография» (макет 123 Б3). Любая правка поля — живой расчёт у Go: мм, ошибки под полями, «Сейчас альбом даёт…».
  const groups = $derived(baseGroups(printList.infos))
  const bad = (k: string) => !!fieldErr(ui.check, k)
  const lines = $derived(ui.check?.lines ?? [])
  const general = $derived(formErr(ui.check))

  // Выбор основы: при вопросе (были правки) список сразу возвращается на прежнюю основу — отказ её не меняет.
  function pickBase(sel: HTMLSelectElement) {
    void ui.setBase(sel.value)
    if (ui.ask) sel.value = ui.e.base
  }

  // Следим за всеми полями: JSON читает каждое. Запрос — вне отслеживания, номер запроса отсекает устаревший ответ.
  $effect(() => {
    void JSON.stringify(ui.e)
    untrack(() => void ui.recheck())
  })
</script>

{#if ui.made}
  <div class="made" role="status">
    <b>{ui.made.title}</b>
    {#each ui.made.lines as l (l)}<div>{l}</div>{/each}
  </div>
{/if}
<div class="two">
  <div class="col">
    <label class="lbl2" for="pe-name">Название</label>
    <input id="pe-name" type="text" class:bad={bad('name')} maxlength="60" spellcheck="false" data-autofocus bind:value={ui.e.name} />
    <p class="errt" role="alert">{fieldErr(ui.check, 'name')}</p>
  </div>
  <div class="col">
    <label class="lbl2" for="pe-base">Основа</label>
    <select id="pe-base" class:bad={bad('base')} value={ui.e.base} onchange={(e) => pickBase(e.currentTarget)}>
      {#each groups as g (g.label)}
        <optgroup label={g.label}>
          {#each g.items as i (i.id)}<option value={i.id}>{i.name}</option>{/each}
        </optgroup>
      {/each}
      <option value="">{BASE_NONE}</option>
    </select>
    <p class="hint">{BASE_HINT}</p>
    <p class="errt" role="alert">{fieldErr(ui.check, 'base')}</p>
  </div>
</div>
<div class="nums">
  <div class="col">
    <label class="lbl2" for="pe-dpi">Плотность, dpi</label>
    <input id="pe-dpi" type="number" class:bad={bad('dpi')} min="72" max="1200" step="1" bind:value={ui.e.dpi} />
    <p class="errt" role="alert">{fieldErr(ui.check, 'dpi')}</p>
  </div>
  <div class="col">
    <label class="lbl2" for="pe-bleed">Обрез, мм</label>
    <input id="pe-bleed" type="number" class:bad={bad('bleed')} min="0" max="10" step="0.5" bind:value={ui.e.bleed} />
    <p class="errt" role="alert">{fieldErr(ui.check, 'bleed')}</p>
  </div>
  <div class="col">
    <label class="lbl2" for="pe-safe">Безопасная зона, мм</label>
    <input id="pe-safe" type="number" class:bad={bad('safe')} min="0" max="30" step="0.5" bind:value={ui.e.safe} />
    <p class="errt" role="alert">{fieldErr(ui.check, 'safe')}</p>
  </div>
  <label class="chk" title="Разворот раскрывается плоско"><input type="checkbox" bind:checked={ui.e.layflat} />Layflat-альбом</label>
</div>

<PrintFormats />

<p class="sec">Файлы выгрузки</p>
<div class="files">
  <div class="col">
    <label class="lbl2" for="pe-prefix">Начало имени файла</label>
    <input
      id="pe-prefix"
      type="text"
      class:bad={bad('prefix')}
      maxlength="12"
      spellcheck="false"
      placeholder="1-"
      bind:value={ui.e.prefix}
    />
    <p class="errt" role="alert">{fieldErr(ui.check, 'prefix')}</p>
  </div>
  <div class="col">
    <span class="lbl2">Качество</span>
    <div class="segw">
      <Seg
        look="card"
        label="Качество"
        items={[
          { id: 100, label: '100' },
          { id: 95, label: '95' },
        ]}
        value={ui.e.quality}
        onpick={(q) => ui.setQuality(q)}
      />
    </div>
    <p class="errt" role="alert">{fieldErr(ui.check, 'quality')}</p>
  </div>
  <div class="col">
    <label class="lbl2" for="pe-url">Сайт типографии</label>
    <input id="pe-url" type="text" class:bad={bad('url')} spellcheck="false" placeholder="https://" bind:value={ui.e.url} />
    <p class="errt" role="alert">{fieldErr(ui.check, 'url')}</p>
  </div>
</div>
{#if ui.check?.names}<p class="hint names">{namesLine(ui.check.names)}</p>{/if}
{#if general}<p class="errt form" role="alert">{general}</p>{/if}
<div class="check">
  {#each lines as l, i (l)}<div class="t" class:f={isHitLine(ui.check, i)}>{l}</div>{/each}
  {#if ui.check && lines.length === 0}<div class="t">{NO_DPI_LINE}</div>{/if}
</div>

<style>
  .two {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 14px;
    margin-bottom: 10px;
  }
  .nums {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-start;
    gap: 14px;
    margin-bottom: 12px;
  }
  .nums .col input {
    width: 92px;
  }
  .col {
    display: flex;
    flex-direction: column;
    gap: 6px;
    min-width: 0;
  }
  .lbl2 {
    color: var(--muted);
    font-size: 12px;
  }
  .chk {
    display: flex;
    align-items: center;
    gap: 6px;
    height: 30px;
    margin-top: 18px;
  }
  input[type='text'] {
    width: 100%;
    font: inherit;
    height: 30px;
    background: #3a3a3e;
    color: var(--text);
    border: 1px solid var(--line);
    border-radius: 6px;
    padding: 0 8px;
    user-select: text;
  }
  select {
    width: 100%;
  }
  input.bad,
  select.bad {
    border-color: #e5645a;
  }
  .errt {
    margin: 0;
    font-size: 12px;
    line-height: 1.35;
    color: #e5645a;
    overflow-wrap: anywhere;
  }
  .errt:empty {
    display: none;
  }
  .errt.form {
    margin-bottom: 8px;
  }
  .hint {
    margin: 0;
    font-size: 12px;
    line-height: 1.35;
    color: var(--muted);
  }
  .hint.names {
    margin: -6px 0 12px;
  }
  .sec {
    font-weight: 600;
    margin: 0 0 6px;
  }
  .files {
    display: grid;
    grid-template-columns: 290px auto 1fr;
    gap: 14px;
    align-items: start;
    margin-bottom: 12px;
  }
  .segw :global(button) {
    min-width: 52px;
    height: 30px;
    padding: 0 12px;
  }
  .check {
    border: 1px solid var(--line);
    border-radius: 6px;
    background: var(--panel2);
    padding: 8px 10px;
    margin-bottom: 12px;
    display: flex;
    flex-direction: column;
    gap: 2px;
    font-size: 12px;
  }
  .check .t {
    color: var(--text);
  }
  .check .t.f {
    color: #6cc08b;
  }
  .made {
    border: 1px solid var(--line);
    border-radius: 6px;
    background: var(--panel2);
    padding: 8px 10px;
    margin-bottom: 12px;
    font-size: 12px;
  }
  .made b {
    font-size: 13px;
  }
  .made div {
    color: var(--muted);
  }
</style>
