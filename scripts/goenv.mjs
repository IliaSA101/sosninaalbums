// Кэши Go и golangci-lint — в .gotmp основной копии (общие для всех worktree).
// Почему: пользовательская переменная GOCACHE перебивала env из .claude/settings.json,
// и кэш проекта за 19–20.10 ушёл в D:\gocache (+36 ГБ) мимо .gotmp (20.10).
import { spawnSync } from "node:child_process";
import path from "node:path";

// Корень основной копии: рядом с общим .git; не git — сама папка here.
export function mainRoot(here) {
  const common = spawnSync(
    "git",
    ["rev-parse", "--path-format=absolute", "--git-common-dir"],
    { cwd: here, encoding: "utf8" },
  );
  return common.status === 0 ? path.dirname(common.stdout.trim()) : here;
}

export function goCaches(here) {
  const main = mainRoot(here);
  return {
    GOCACHE: path.join(main, ".gotmp", "cache-go"),
    GOLANGCI_LINT_CACHE: path.join(main, ".gotmp", "cache-golangci"),
  };
}

// Поверх любых пользовательских значений.
export const withGoCaches = (base, here) => ({ ...base, ...goCaches(here) });

// Версия golangci-lint из CI (задание golangci-lint-action в ci.yml), как gcl_want в scripts/env.sh:
// у разных версий разные замечания (04.11: gocognit 2.14 строже 2.15 — precheck пропустил, CI упал).
export function gclWant(ciText) {
  const i = ciText.indexOf("golangci-lint-action");
  if (i < 0) return "";
  const m = /^\s*version:\s*(v[0-9.]+)/m.exec(ciText.slice(i));
  return m ? m[1] : "";
}

// Версия из вывода `golangci-lint version` («... has version 2.14.0 built ...») — с «v», как в ci.yml.
export function gclHave(out) {
  const m = /version v?([0-9][0-9.]*)/.exec(out);
  return m ? `v${m[1]}` : "";
}

// Папки инструментов Go проекта в порядке scripts/env.sh: SA_TOOLS, sosninaalbums-tools рядом с основной копией,
// GOBIN. Раньше PATH: в ~/go/bin бывают версии соседних проектов.
export function toolDirs(here, e = process.env) {
  return [
    e.SA_TOOLS,
    path.join(path.dirname(mainRoot(here)), "sosninaalbums-tools"),
    e.GOBIN,
  ].filter(Boolean);
}
