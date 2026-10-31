<script lang="ts">
  import { tick } from 'svelte'
  import Modal from '../../base/modal/Modal.svelte'
  import Seg from '../../base/tpl/Seg.svelte'
  import AgentsPlaces from './AgentsPlaces.svelte'
  import { agents } from './agents.svelte'
  import {
    FOLDER_TOOLS,
    allowMissing,
    askOnly,
    KEY_LIFTED,
    KEY_PLACES,
    commonMode,
    groupCan,
    isMode,
    noteShown,
    placesShown,
    rowKey,
    rowsOf,
    segItems,
  } from './agentperms'
  import {
    ASK_EDIT,
    ASK_ONLY_NOTE,
    ASK_FOLDER,
    CANCEL_LABEL,
    EDIT_ELEVATED,
    GROUP_EDIT,
    GROUP_MIXED,
    GROUP_OTHER_NOTE,
    GROUP_READ,
    MUTED_LABEL,
    MUTED_TEXT,
    PERMS_NOTE,
    PERMS_TITLE,
    PLACES_FORGET,
    PLACES_HEAD,
    PLACES_NONE,
    placesList,
    PLACE_CHOOSE,
    READ_WHY_TAIL,
    placesMore,
  } from './agentstext'

  // Блок «Разрешения» окна «ИИ-агенты» (150, design §7.1) вместо флажка «Правки» 0.33. Группы «Только чтение» (свёрнута) и «Правки»;
  // у группы и строки — «Всегда · Спрашивать · Запретить» (Seg). Что допустимо, решает Go (AgentPerm.can). Образец — `<details>`
  // «Как подключить» и сегменты панелей. consent — тот же компонент рисует согласие (вопрос «Отмена · Разрешить») отдельным окном.
  let { consent = false }: { consent?: boolean } = $props()

  const info = $derived(agents.info)
  const blocked = $derived(info.editBlocked === 'elevated')
  const groups = $derived([
    { id: 'read' as const, title: GROUP_READ, rows: rowsOf(info.perms, 'read'), extra: [] },
    { id: 'edit' as const, title: GROUP_EDIT, rows: rowsOf(info.perms, 'edit'), extra: rowsOf(info.perms, 'other') },
  ])
  const quietOn = $derived(info.perms.some((p) => FOLDER_TOOLS.includes(p.tool) && p.mode === 'allow'))
  const pl = $derived(placesShown(info.places))

  // Состояние группы своё, не `open={…}`: Svelte переписал бы атрибут при каждом событии Go и захлопнул бы раскрытое человеком.
  let open = $state({ read: false, edit: true })
  let root = $state<HTMLElement>()

  // Окно открыли «на строке» (тост «Настроить», ссылки вопроса): раскрыть её группу и прокрутить.
  $effect(() => {
    const key = agents.flash
    if (!key || consent) return
    const p = info.perms.find((x) => rowKey(x.tool) === key)
    if (p) open[p.group === 'read' ? 'read' : 'edit'] = true
    else if (key === KEY_PLACES) open.edit = true
    else if (key !== KEY_LIFTED) return
    void tick().then(() => {
      for (const el of root?.querySelectorAll<HTMLElement>('[data-row]') ?? [])
        if (el.dataset.row === key) el.scrollIntoView?.({ block: 'nearest' })
    })
  })
</script>

<!-- Места съёмок в строке: до двух путей, «и ещё N», кнопка «Папка со съёмками…». none — слова, если мест нет -->
{#snippet places(none: string)}
  {#if pl.shown.length > 0}
    {#each pl.shown as d, i (d)}{i > 0 ? ', ' : ' '}<bdi>{d}</bdi>{/each}{pl.more > 0 ? placesMore(pl.more) : ''}
  {:else if none}
    {' ' + none}
  {/if}
  ·
  <button class="link" disabled={agents.busy} onclick={() => agents.choosePlace()}>{PLACE_CHOOSE}</button>
{/snippet}

{#if consent}
  {#if agents.ask === 'perm'}
    {@const c = agents.consent}
    <!-- Согласие: «Отмена» слева, «Разрешить» справа. Оба абзаца сразу — когда группа правок уходит из «Запретить» в «Всегда» -->
    <Modal title={c.edit ? ASK_EDIT.title : ASK_FOLDER.title} onclose={() => agents.dismiss()} wide>
      {#if c.edit}<p class="body">{ASK_EDIT.text}</p>{/if}
      {#if c.folder}
        <p class="body">{ASK_FOLDER.lead}</p>
        <p class="body places">{ASK_FOLDER.placesHead}{@render places(ASK_FOLDER.placesNone)}{ASK_FOLDER.placesTail}</p>
        <p class="body">{ASK_FOLDER.cloud}</p>
      {/if}
      <div class="actions">
        <button class="ghost" onclick={() => agents.dismiss()}>{CANCEL_LABEL}</button>
        <button class="primary" data-autofocus onclick={() => agents.confirm()}>{ASK_FOLDER.ok}</button>
      </div>
    </Modal>
  {/if}
{:else}
  <div bind:this={root}>
    <h3>{PERMS_TITLE}</h3>
    <p class="note">{PERMS_NOTE}</p>
    {#if blocked}<p class="note" role="note">{EDIT_ELEVATED}</p>{/if}
    <AgentsPlaces part="note" />
    {#each groups as g (g.id)}
      {@const off = blocked && g.id === 'edit'}
      {@const common = commonMode(g.rows)}
      <details class="grp" bind:open={open[g.id]}>
        <summary>
          <span class="tw">▸</span>
          <span>{g.title}</span>
          <span class="count">{g.rows.length + g.extra.length}</span>
          <span class="grow"></span>
          {#if !common}<span class="mixed">{GROUP_MIXED}</span>{/if}
          <Seg items={segItems(groupCan(g.rows), off)} value={common} onpick={(m) => agents.pickGroup(g.id, m)} label={g.title} />
        </summary>
        {#if agents.whyFor[`group:${g.id}`]}
          <p class="note why">{g.rows[0]?.why}{g.id === 'read' ? READ_WHY_TAIL : ''}</p>
        {/if}
        {#each [...g.rows, ...g.extra] as p, i (p.tool)}
          {#if g.extra.length > 0 && i === g.rows.length}<p class="note other">{GROUP_OTHER_NOTE}</p>{/if}
          <div class="row" data-row={rowKey(p.tool)} class:flash={agents.flash === rowKey(p.tool)}>
            <div class="t">
              {p.title}
              {#if noteShown(p)}<small>{p.note}</small>{/if}
              {#if askOnly(p)}<small>{ASK_ONLY_NOTE}</small>{/if}
              {#if agents.whyFor[p.tool] && p.why}<small class="why">{p.why}{p.group === 'read' ? READ_WHY_TAIL : ''}</small>{/if}
            </div>
            <Seg
              items={segItems(p.can, off && p.group === 'edit', allowMissing(p))}
              value={isMode(p.mode) ? p.mode : undefined}
              onpick={(m) => agents.pickTool(p.tool, m)}
              label={p.title}
            />
          </div>
        {/each}
        {#if g.id === 'edit' && (quietOn || info.lifted.length > 0)}
          <div class="places-row" data-row={KEY_PLACES} class:flash={agents.flash === KEY_PLACES}>
            {#if pl.shown.length > 0}{PLACES_HEAD}{:else}{PLACES_NONE}{/if}{@render places('')}
            {#if info.places.length > 0}
              ·
              <button class="link" disabled={agents.busy} onclick={() => agents.forgetPlaces()}>{PLACES_FORGET}</button>
              ·
              <button class="link" aria-expanded={agents.placesOpen} onclick={() => agents.togglePlaces()}>
                {placesList(info.places.length, agents.placesOpen)}
              </button>
            {/if}
          </div>
          <AgentsPlaces part="list" />
        {/if}
      </details>
    {/each}
    {#if info.asksMuted}
      <div class="mute">
        <span class="note grow">{MUTED_TEXT}</span>
        <button disabled={agents.busy} onclick={() => agents.allowAsks()}>{MUTED_LABEL}</button>
      </div>
    {/if}
  </div>
{/if}

<style>
  h3 {
    margin: 12px 0 6px;
    font-size: 13px;
    font-weight: 600;
  }
  p {
    margin: 0 0 10px;
    line-height: 1.4;
  }
  .note {
    color: var(--muted);
    font-size: 12px;
  }
  .grp {
    margin: 0 0 8px;
    border: 1px solid var(--line);
    border-radius: 8px;
  }
  summary {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 8px 6px 10px;
    cursor: pointer;
    font-size: 13px;
    font-weight: 600;
    list-style: none;
  }
  summary::-webkit-details-marker {
    display: none;
  }
  .tw {
    display: inline-block;
    width: 10px;
    color: var(--muted);
    transition: transform 0.15s;
  }
  .grp[open] > summary .tw {
    transform: rotate(90deg);
  }
  .count {
    padding: 0 6px;
    border: 1px solid var(--line);
    border-radius: 9px;
    color: var(--muted);
    font-size: 11px;
    font-weight: 400;
  }
  .mixed {
    color: var(--muted);
    font-size: 11px;
    font-weight: 400;
  }
  .grow {
    flex: 1;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 8px 6px 28px;
    border-top: 1px solid var(--line);
  }
  .t {
    flex: 1;
    min-width: 0;
  }
  small {
    display: block;
    margin-top: 1px;
    color: var(--muted);
    font-size: 11px;
  }
  small.why {
    color: var(--accent);
  }
  .why {
    padding: 0 10px 6px 28px;
  }
  .other {
    margin: 0;
    padding: 6px 10px 2px 28px;
    border-top: 1px solid var(--line);
  }
  .places-row {
    padding: 6px 10px 8px 28px;
    border-top: 1px solid var(--line);
    color: var(--muted);
    font-size: 12px;
    overflow-wrap: anywhere;
  }
  bdi {
    color: var(--text);
    font-family: Consolas, monospace;
  }
  .flash {
    outline: 2px solid var(--accent);
    outline-offset: -2px;
    border-radius: 4px;
  }
  .mute {
    display: flex;
    align-items: center;
    gap: 8px;
    margin: 0 0 6px;
  }
  .mute .note {
    margin: 0;
  }
  .body {
    margin: 0 0 10px;
    font-size: 14px;
  }
  .places {
    overflow-wrap: anywhere;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 12px;
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
</style>
