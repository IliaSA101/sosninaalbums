<script lang="ts">
  import Seg from '../../base/tpl/Seg.svelte'
  import type { Browsed, BrowseSel, FormSel } from './browse'

  // Ряд отбора под шапкой окна (133): «Набор [Все наборы ▾] · Вид · Стиль». Элемент есть, только если есть из чего выбирать;
  // нет ни одного — ряда нет. Образцы: «Вид» у виньеток (056) — Seg, «Тема» в «Сохранить разворот…» — <select>.
  let {
    br,
    onpick,
  }: {
    br: Pick<Browsed<unknown>, 'packs' | 'forms' | 'styles' | 'sel'>
    onpick: <K extends keyof BrowseSel>(key: K, value: BrowseSel[K]) => void
  } = $props()

  const ALL_ITEM = { id: '', label: 'Все' }
  const forms = $derived([ALL_ITEM, ...br.forms])
  const styles = $derived([ALL_ITEM, ...br.styles])
</script>

{#if br.packs.length || br.forms.length || br.styles.length}
  <div class="filters" role="group" aria-label="Отбор плиток">
    {#if br.packs.length}
      <div class="grp">
        <label class="lbl" for="decor-pack">Набор</label>
        <select id="decor-pack" value={br.sel.pack} onchange={(e) => onpick('pack', e.currentTarget.value)}>
          <option value="">Все наборы</option>
          {#each br.packs as p (p.id)}<option value={p.id}>{p.label}</option>{/each}
        </select>
      </div>
    {/if}
    {#if br.forms.length}
      <div class="grp">
        <span class="lbl">Вид</span>
        <Seg items={forms} value={br.sel.form} onpick={(f) => onpick('form', f as FormSel)} label="Вид стикеров" />
      </div>
    {/if}
    {#if br.styles.length}
      <div class="grp">
        <span class="lbl">Стиль</span>
        <Seg items={styles} value={br.sel.style} onpick={(s) => onpick('style', s)} label="Стиль стикеров" />
      </div>
    {/if}
  </div>
{/if}

<style>
  .filters {
    flex: none;
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px 10px;
  }
  /* Пара «подпись + элемент» переносится целиком, а не рвётся на границе строк */
  .grp {
    display: inline-flex;
    flex-wrap: nowrap;
    align-items: center;
    gap: 6px;
  }
  .lbl {
    color: var(--muted);
    font-size: 12px;
  }
  select {
    max-width: 200px;
  }
</style>
