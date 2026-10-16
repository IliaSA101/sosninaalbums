import js from '@eslint/js'
import ts from 'typescript-eslint'
import svelte from 'eslint-plugin-svelte'
import globals from 'globals'
import { globSync } from 'node:fs'

// --- Пределы и границы областей (решение владельца 2026-10-09; docs/STRUCTURE.md «Правила») ---

// Как считает check.ps1: пустые строки и комментарии входят в счёт
const LINES = { skipBlankLines: false, skipComments: false }

// Долг: функции длиннее 80 строк. Потолок = текущий максимум файла: расти нельзя, новые файлы — по 80.
const FN_DEBT = {}

// Долг: сложность выше 15. Потолок = текущий максимум файла.
const CX_DEBT = {}

const limits = [
  {
    files: ['src/**/*.{ts,svelte}'],
    rules: {
      'max-lines': ['error', { max: 600, ...LINES }],
      'max-lines-per-function': ['error', { max: 80, ...LINES }],
      complexity: ['error', 15],
    },
  },
  {
    // В тестах describe() оборачивает все it(): длина функции там ничего не значит
    files: ['src/**/*.test.ts'],
    rules: { 'max-lines-per-function': 'off' },
  },
  ...Object.entries(FN_DEBT).map(([file, max]) => ({
    files: [file],
    rules: { 'max-lines-per-function': ['error', { max, ...LINES }] },
  })),
  ...Object.entries(CX_DEBT).map(([file, max]) => ({
    files: [file],
    rules: { complexity: ['error', max] },
  })),
]

// Границы областей: no-restricted-imports с regex по тексту импорта.
// Правило в конфиге заменяется целиком, поэтому каждый набор перечисляет свои шаблоны полностью.
const STATE = {
  regex: '\\.svelte(\\.ts)?$',
  message: 'Чистый модуль не импортирует состояние и компоненты (*.svelte, *.svelte.ts, core/store): передайте данные аргументами.',
}
const restrict = (...patterns) => ({ 'no-restricted-imports': ['error', { patterns }] })

const boundaries = [
  {
    // Чистые модули отметок и зума (файлы с .svelte в имени — состояние, им можно)
    files: ['src/panels/marks/*.ts', 'src/editor/zoom/*.ts'],
    ignores: ['**/*.svelte.ts', '**/*.svelte.test.ts'],
    rules: restrict(STATE),
  },
  {
    // geom/: только типы ядра, свои файлы и text/textedit (хвост: textbox → textedit, убрать при правке)
    files: ['src/base/geom/**/*.ts'],
    rules: restrict(STATE, {
      regex: '^\\.\\./(?!core/types$|geom/|\\.\\./editor/text/textedit$|(\\.\\./)+testdata/)',
      message: 'geom/ зависит только от core/types, своих файлов и text/textedit (docs/STRUCTURE.md, правило 6).',
    }),
  },
  {
    // groupgeom/: то же плюс geom/
    files: ['src/base/groupgeom/**/*.ts'],
    rules: restrict(STATE, {
      regex: '^\\.\\./(?!core/types$|geom/|groupgeom/|\\.\\./editor/text/textedit$|(\\.\\./)+testdata/)',
      message: 'groupgeom/ зависит только от core/types, geom/, своих файлов и text/textedit (docs/STRUCTURE.md, правило 6).',
    }),
  },
  {
    // ui/ — общие виджеты: ни областей, ни состояния
    files: ['src/base/ui/**/*.{ts,svelte}'],
    rules: restrict({
      regex: '^\\.\\./(?!core/types$)',
      message: 'ui/ — общие виджеты: не импортирует области; данные и действия приходят пропсами.',
    }),
  },
  {
    // Хвост: FixedTip читает drag и timeline/tipLeft — убрать при правке
    files: ['src/base/ui/FixedTip.svelte'],
    rules: restrict({
      regex: '^\\.\\./(?!core/types$|drag/drag\\.svelte$|\\.\\./panels/timeline/timeline$)',
      message: 'ui/FixedTip: допустимы только drag/drag.svelte и timeline/timeline (долг).',
    }),
  },
  {
    // Основа не зовёт верхние группы (решение владельца 2026-10-09). geom, groupgeom и ui закрыты блоками выше
    // строже; здесь — остальная основа. Блоки с одним правилом не сливаются (действует последний), поэтому
    // файлы наборов не пересекаются. Долга нет (2026-10-10): верх подключается к основе хуками в shell/boot/wire.ts.
    files: ['src/base/{core,prefs,drag,types,words}/**/*.{ts,svelte}'],
    rules: restrict({
      regex: '^(\\.\\./){2,}(editor|panels|shell)/',
      message: 'base/ не импортирует editor/, panels/, shell/: зависимость вверх — передайте данные или колбэк (docs/STRUCTURE.md).',
    }),
  },
]

// Самопроверка: путь долга или границы, не совпавший ни с одним файлом, молча выключает проверку (так было бы
// после переноса в группы). Конфиг падает сразу — путь надо поправить вместе с переносом.
const here = import.meta.dirname
for (const pattern of [
  ...Object.keys(FN_DEBT),
  ...Object.keys(CX_DEBT),
  ...boundaries.flatMap((b) => [...b.files, ...(b.ignores ?? []).filter((p) => !p.startsWith('**'))]),
]) {
  if (globSync(pattern, { cwd: here }).length === 0) {
    throw new Error(`eslint.config.js: путь «${pattern}» не совпал ни с одним файлом — поправьте его после переноса`)
  }
}

export default ts.config(
  { ignores: ['wailsjs/**', 'dist/**', 'node_modules/**'] },
  js.configs.recommended,
  ...ts.configs.recommended,
  ...svelte.configs['flat/recommended'],
  { languageOptions: { globals: { ...globals.browser } } },
  {
    // Svelte-файлы (и .svelte.ts со рунами): TS-парсер для <script lang="ts">
    files: ['**/*.svelte', '**/*.svelte.ts'],
    languageOptions: { parserOptions: { parser: ts.parser } },
  },
  {
    // Svelte-переходы получают узел первым аргументом, даже если он не нужен: `_` — «не используется»
    rules: { '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_' }] },
  },
  {
    files: ['*.config.{js,ts}', 'vite.config.ts'],
    languageOptions: { globals: { ...globals.node } },
  },
  ...limits,
  ...boundaries,
)
