<script lang="ts">
  import Modal from '../../base/ui/Modal.svelte'
  import { GROUPS, visibleHotkeys } from './hotkeys'
  import { help } from './help.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'

  // Таблица та же, по которой работают клавиши: справка не расходится с поведением. Только клавиши уровня.
  const groups = $derived.by(() => {
    const rows = visibleHotkeys(prefs.can)
    return GROUPS.map((g) => ({ ...g, rows: rows.filter((h) => h.group === g.id) })).filter((g) => g.rows.length)
  })
</script>

{#if help.open}
  <Modal title="Горячие клавиши" onclose={() => (help.open = false)} wide>
    <div class="cols">
      {#each groups as g (g.id)}
        <section>
          <h3>{g.title}</h3>
          <table>
            <tbody>
              {#each g.rows as r, i (i)}
                <tr>
                  <td class="key">{r.show}</td>
                  <td>{r.label}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </section>
      {/each}
    </div>
  </Modal>
{/if}

<style>
  /* Окно шире обычного: две колонки разделов */
  :global(.box.wide):has(.cols) {
    width: 760px;
  }
  .cols {
    columns: 2 340px;
    column-gap: 24px;
  }
  section {
    break-inside: avoid;
    margin-bottom: 12px;
  }
  h3 {
    margin: 0 0 4px;
    font-size: 12px;
    font-weight: 600;
    color: var(--accent);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }
  table {
    width: 100%;
    border-collapse: collapse;
  }
  td {
    padding: 3px 0;
    vertical-align: top;
    border-bottom: 1px solid var(--line);
    line-height: 1.35;
  }
  .key {
    width: 42%;
    padding-right: 10px;
    color: var(--text);
    font-weight: 600;
    white-space: normal;
  }
  td:not(.key) {
    color: var(--muted);
  }
</style>
