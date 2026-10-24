<script lang="ts">
  import Modal from '../../base/modal/Modal.svelte'
  import { backupsUi } from './backups.svelte'
  import { countLabel, sizeLabel, whenLabel } from './backups'

  // Окно «Резервные копии» (спека 019 E): копии текущего альбома, новые первыми.
  // «Восстановить» заменяет альбом копией одним шагом правки — Ctrl+Z и «Вернуть» в тосте возвращают как было.
  const now = $derived(backupsUi.open ? Date.now() : 0)
</script>

{#if backupsUi.open}
  <Modal title="Резервные копии" onclose={() => backupsUi.close()} wide>
    {#if backupsUi.loading}
      <p class="note">Читаем список…</p>
    {:else if backupsUi.items.length === 0 && !backupsUi.error}
      <p class="note">Копий пока нет: они появляются при сохранении, не чаще раза в 10 минут.</p>
    {:else}
      <ul class="list" aria-label="Копии альбома">
        {#each backupsUi.items as b (b.name)}
          <li>
            <div class="info">
              <span class="when">{whenLabel(b.time, now)}</span>
              <span class="meta">{countLabel(b.spreads, b.photos)} · {sizeLabel(b.size)}</span>
            </div>
            <button disabled={backupsUi.busy} onclick={() => backupsUi.recover(b)}>Восстановить</button>
          </li>
        {/each}
      </ul>
    {/if}
    {#if backupsUi.error}<p class="err" role="alert">{backupsUi.error}</p>{/if}
    <footer>
      <button class="ghost" onclick={() => backupsUi.showFolder()}>Показать папку</button>
      <button data-autofocus onclick={() => backupsUi.close()}>Закрыть</button>
    </footer>
  </Modal>
{/if}

<style>
  .note {
    margin: 0 0 12px;
    color: var(--muted);
  }
  .list {
    list-style: none;
    margin: 0 0 12px;
    padding: 0;
    max-height: 320px;
    overflow: auto;
    border: 1px solid var(--line);
    border-radius: 6px;
  }
  li {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 6px 10px;
  }
  li + li {
    border-top: 1px solid var(--line);
  }
  .info {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .when {
    font-weight: 600;
  }
  .meta {
    color: var(--muted);
    font-size: 12px;
  }
  .err {
    margin: 0 0 12px;
    color: #ff8a8a;
  }
  footer {
    display: flex;
    justify-content: space-between;
    gap: 8px;
  }
</style>
