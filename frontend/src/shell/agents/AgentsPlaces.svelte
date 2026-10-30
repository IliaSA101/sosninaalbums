<script lang="ts">
  import { isMac } from '../../base/core/platform'
  import { agents } from './agents.svelte'
  import { KEY_LIFTED, liftGroups, placeRows } from './agentperms'
  import { LIFT_OK, PLACE_FORGET_ONE, SETUP_LABEL, liftNote, placeForgetLabel } from './agentstext'

  // Места съёмок агента (153, design §3) в блоке «Разрешения»: заметка о подъёме мест 0.34 (part = note, над группами, видна всегда)
  // и раскрытый список (part = list, под строкой мест). Вид — «Последние альбомы» стартового окна: путь, серым справа метка диска, ✕.
  let { part }: { part: 'note' | 'list' } = $props()

  const info = $derived(agents.info)
  const rows = $derived(placeRows(info.places, agents.placesNow, isMac()))
  const lifts = $derived(liftGroups(info.lifted))
</script>

{#if part === 'note' && lifts.length > 0}
  <div class="lift" role="note" data-row={KEY_LIFTED} class:flash={agents.flash === KEY_LIFTED}>
    <div class="grow">
      {#each lifts as g (g.to)}<p>{liftNote(g.to, g.from)}</p>{/each}
    </div>
    <button class="link" disabled={agents.busy} onclick={() => agents.dismissLifted()}>{LIFT_OK}</button>
    ·
    <button class="link" onclick={() => agents.seePlaces()}>{SETUP_LABEL}</button>
  </div>
{:else if part === 'list' && agents.placesOpen && rows.length > 0}
  <ul class="places" aria-label="Места съёмок">
    {#each rows as r (r.key || r.dir)}
      <li class:dim={r.dim}>
        <div class="line">
          <bdi class="dir">{r.dir}</bdi>
          {#if r.right}<span class="side">{r.right}</span>{/if}
        </div>
        {#if r.hints.length > 0 || r.bind}
          <small>
            {r.hints.join(' · ')}
            {#if r.bind}
              {r.hints.length > 0 ? '·' : ''}
              <button class="link" disabled={agents.busy} onclick={() => agents.bindPlace(r.key)}>{r.bind}</button>
            {/if}
          </small>
        {/if}
        <button class="forget" title={PLACE_FORGET_ONE} aria-label={placeForgetLabel(r.dir)} onclick={() => agents.forgetPlace(r.key)}
          >✕</button
        >
      </li>
    {/each}
  </ul>
{/if}

<style>
  p {
    margin: 0;
    line-height: 1.4;
  }
  .lift {
    display: flex;
    align-items: baseline;
    gap: 8px;
    margin: 0 0 8px;
    padding: 6px 10px;
    border: 1px solid var(--line);
    border-radius: 8px;
    font-size: 12px;
    overflow-wrap: anywhere;
  }
  .grow {
    flex: 1;
    min-width: 0;
  }
  .flash {
    outline: 2px solid var(--accent);
    outline-offset: -2px;
  }
  .places {
    margin: 0;
    padding: 2px 8px 6px 28px;
    list-style: none;
    border-top: 1px solid var(--line);
  }
  li {
    position: relative;
    padding: 3px 30px 3px 8px;
    font-size: 12px;
    border-radius: 4px;
  }
  li:hover {
    background: #38383c;
  }
  .dim .line {
    opacity: 0.45;
  }
  .line {
    display: flex;
    justify-content: space-between;
    gap: 12px;
  }
  .dir {
    min-width: 0;
    overflow-wrap: anywhere;
    font-family: Consolas, monospace;
  }
  .side {
    flex: none;
    color: var(--muted);
  }
  small {
    display: block;
    color: var(--muted);
    font-size: 11px;
  }
  .forget {
    position: absolute;
    top: 1px;
    right: 0;
    width: 26px;
    height: 26px;
    padding: 0;
    background: transparent;
    color: var(--muted);
  }
  .forget:hover {
    color: var(--text);
  }
  /* Ссылка-кнопка: как в блоке «Разрешения» (AgentsPerms) */
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
