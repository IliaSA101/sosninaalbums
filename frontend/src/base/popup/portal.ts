// Узел уходит в body: у предка своя обрезка или stacking context (панель, холст, лента), и всплывающее окно
// перекрывалось бы или срезалось. Действие Svelte: use:portal.
export function portal(node: HTMLElement) {
  document.body.appendChild(node)
  return { destroy: () => node.remove() }
}
