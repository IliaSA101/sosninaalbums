// Тесты goenv: `node --test scripts/`.
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import {
  gclHave,
  gclWant,
  goCaches,
  mainRoot,
  toolDirs,
  withGoCaches,
} from "./goenv.mjs";

const here = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

test("mainRoot: корень основной копии, не worktree", () => {
  const m = mainRoot(here);
  assert.ok(path.isAbsolute(m));
  assert.ok(!m.includes(`${path.sep}.claude${path.sep}worktrees`), m);
});

test("mainRoot: не git — сама папка", () => {
  const root = path.parse(here).root;
  assert.equal(mainRoot(root), root);
});

test("goCaches: пути в .gotmp основной копии", () => {
  const c = goCaches(here);
  assert.equal(c.GOCACHE, path.join(mainRoot(here), ".gotmp", "cache-go"));
  assert.equal(
    c.GOLANGCI_LINT_CACHE,
    path.join(mainRoot(here), ".gotmp", "cache-golangci"),
  );
});

test("withGoCaches: перебивает заданный GOCACHE, остальное сохраняет", () => {
  const e = withGoCaches({ GOCACHE: "X:/user", FOO: "1" }, here);
  assert.equal(e.GOCACHE, goCaches(here).GOCACHE);
  assert.equal(e.FOO, "1");
});

test("gclWant: версия из шага golangci-lint-action, а не первый version: в файле", () => {
  const ci =
    "jobs:\n  a:\n    with:\n      version: v9.9.9\n  b:\n      - uses: golangci/golangci-lint-action@abc # v9.3.0\n        with:\n          version: v2.14.0\n";
  assert.equal(gclWant(ci), "v2.14.0");
  assert.equal(gclWant("jobs: {}"), "");
});

test("gclWant: в ci.yml проекта версия задана", () => {
  const ci = fs.readFileSync(
    path.join(here, ".github", "workflows", "ci.yml"),
    "utf8",
  );
  assert.match(gclWant(ci), /^v\d+\.\d+\.\d+$/);
});

test("gclHave: разбор вывода golangci-lint version", () => {
  assert.equal(
    gclHave(
      "golangci-lint has version 2.14.0 built with go1.26.10 from (unknown)",
    ),
    "v2.14.0",
  );
  assert.equal(gclHave("golangci-lint has version v2.15.0 built"), "v2.15.0");
  assert.equal(gclHave("command not found"), "");
});

test("toolDirs: SA_TOOLS первым, затем sosninaalbums-tools рядом с основной копией, затем GOBIN", () => {
  const d = toolDirs(here, { SA_TOOLS: "/t", GOBIN: "/g" });
  assert.deepEqual(d, [
    "/t",
    path.join(path.dirname(mainRoot(here)), "sosninaalbums-tools"),
    "/g",
  ]);
  assert.equal(toolDirs(here, {}).length, 1);
});
