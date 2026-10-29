<script lang="ts">
  import Modal from '../../base/modal/Modal.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import type { Level } from '../../base/prefs/levels'
  import LevelMatrix from './LevelMatrix.svelte'

  // Окно «Уровни и функции» (спеки 010, 011): та же матрица, что в стартовом окне, плюс «Выбрать».
  // Настроить «Свой» посреди работы, не открывая стартовое окно.
  const close = () => (prefs.compare = false)

  function pick(l: Level) {
    close()
    void prefs.set(l)
  }
</script>

{#if prefs.compare}
  <Modal title="Уровни и функции" onclose={close} wide>
    <p class="lead lvdlg">Уровень прячет инструменты, а не данные: всё сделанное выше остаётся на разворотах, в печати и в экспорте.</p>
    <LevelMatrix pick onpick={pick} />
  </Modal>
{/if}

<style>
  /* Окно шире обычного: функция, три столбца уровней и «Свой» */
  :global(.box.wide):has(.lvdlg) {
    width: 860px;
  }
  .lead {
    margin: 0 0 8px;
    color: var(--muted);
  }
</style>
