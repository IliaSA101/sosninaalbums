// Последний компонент пути (папки), с любым разделителем.
export function folderName(f: string): string {
  return f.split(/[\\/]/).filter(Boolean).pop() ?? f
}
