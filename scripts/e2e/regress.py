"""e2e-регресс (166): найденные сценарии как автотесты в Chromium против настоящего Go.

  python3 scripts/e2e/regress.py --up [--port 34117] [--stand regress] [--tree <копия>] [-k имя] [--repeat N] [--keep] [--headed]
  python3 scripts/e2e/regress.py --attach --port 34116 --stand b      # стенд уже поднят stand.sh

--up собирает фронт и dev-программу из чистого среза HEAD (или из --tree), запускает её в xvfb со своими папками
данных и останавливает в конце. Всё лежит в .gotmp/e2e/regress/<время>/. Код выхода: 0 — зелёные, 1 — провал, 2 — отказ.
"""

from __future__ import annotations

import json
import os
import signal
import subprocess
import sys
import time
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
CASES = HERE / "regress"
for p in (str(HERE), str(CASES)):  # python -I не добавляет папку скрипта
    if p not in sys.path:
        sys.path.insert(0, p)

import base  # noqa: E402
import common  # noqa: E402
import standproc  # noqa: E402
from common import PORT_MAX, PORT_MIN, Refuse, check_port, check_stand, slow_factor  # noqa: E402


def parse_args(argv: list[str] | None = None):
    import argparse

    ap = argparse.ArgumentParser(prog="regress.py", description="e2e-регресс (166)")
    mode = ap.add_mutually_exclusive_group(required=True)
    mode.add_argument("--up", action="store_true", help="собрать и поднять свой стенд (Linux)")
    mode.add_argument("--attach", action="store_true", help="работать с уже поднятым стендом (нужен --port)")
    ap.add_argument("--port", type=int, default=None, help=f"порт {PORT_MIN}–{PORT_MAX}; для --up по умолчанию {standproc.DEFAULT_PORT}")
    ap.add_argument("--stand", default="regress", help="имя стенда (SA_STAND)")
    ap.add_argument("--tree", default=None, help="рабочая копия этого репозитория вместо среза HEAD")
    ap.add_argument("-k", dest="pattern", default=None, help="только тесты, в имени которых есть эта подстрока")
    ap.add_argument("--repeat", type=int, default=1, help="сколько раз подряд прогнать набор на одном стенде")
    ap.add_argument("--keep", action="store_true", help="не удалять старые папки прогонов")
    ap.add_argument("--headed", action="store_true", help="показать окно браузера")
    a = ap.parse_args(argv)
    if a.repeat < 1:
        raise Refuse("--repeat: нужно число не меньше 1")
    if a.attach and a.port is None:
        raise Refuse("--attach требует --port")
    a.port = check_port(a.port if a.port is not None else standproc.DEFAULT_PORT)
    a.stand = check_stand(a.stand)
    return a


def load_suite(pattern: str | None) -> unittest.TestSuite:
    loader = unittest.TestLoader()
    if pattern:
        loader.testNamePatterns = [f"*{pattern}*"]
    return loader.discover(str(CASES), pattern="reg_*.py", top_level_dir=str(CASES))


class Timed(unittest.TextTestResult):
    """Результат с временем на тест — для report.md / report.json."""

    def __init__(self, *a, **kw):
        super().__init__(*a, **kw)
        self.rows: list[dict] = []
        self._t0 = 0.0

    def startTest(self, test):
        self._t0 = time.monotonic()
        super().startTest(test)

    def _row(self, test, status):
        bug = getattr(test, "BUG", None) or {}
        self.rows.append({"test": test.id().split(".", 1)[-1], "bug": bug.get("id", ""), "src": bug.get("src", ""),
                          "status": status, "sec": round(time.monotonic() - self._t0, 2)})

    def addSuccess(self, test):
        super().addSuccess(test)
        self._row(test, "OK")

    def addFailure(self, test, err):
        super().addFailure(test, err)
        self._row(test, "FAIL")

    def addError(self, test, err):
        super().addError(test, err)
        self._row(test, "ERROR")

    def addSkip(self, test, reason):
        super().addSkip(test, reason)
        self._row(test, "SKIP")


def run_suite(pattern: str | None) -> Timed:
    suite = load_suite(pattern)
    if suite.countTestCases() == 0:
        raise Refuse("нет тестов" + (f" по -k {pattern!r}" if pattern else ""))
    runner = unittest.TextTestRunner(resultclass=Timed, verbosity=2, stream=sys.stdout)
    return runner.run(suite)


def write_report(run_dir: Path, runs: list[Timed], env: dict) -> None:
    rows = [dict(r, run=i + 1) for i, res in enumerate(runs) for r in res.rows]
    (run_dir / "report.json").write_text(json.dumps({"env": env, "rows": rows}, ensure_ascii=False, indent=1), encoding="utf-8")
    md = ["# e2e-регресс", "", "; ".join(f"{k}: {v}" for k, v in env.items()), "", "| Повтор | Тест | Баг | Источник | Итог | Время, с |", "| --- | --- | --- | --- | --- | --- |"]
    md += [f"| {r['run']} | {r['test']} | {r['bug']} | {r['src']} | {r['status']} | {r['sec']} |" for r in rows]
    (run_dir / "report.md").write_text("\n".join(md) + "\n", encoding="utf-8")


def commit_of(path: Path) -> str:
    r = subprocess.run(["git", "-C", str(path), "rev-parse", "--short", "HEAD"], capture_output=True, text=True, check=False)
    return r.stdout.strip() or "?"


def phase(name: str, t0: float) -> float:
    now = time.monotonic()
    print(f"[{now - t0:6.1f} с] {name}", flush=True)
    return now


def main(argv: list[str] | None = None) -> int:
    a = parse_args(argv)
    tree = standproc.check_tree(a.tree)
    if a.attach:
        check_attach_free(a.port)
    else:
        standproc.check_regress_port(a.port)
    standproc.need_tools(tree or common.ROOT)
    slow = slow_factor()
    run_dir = standproc.REGRESS_OUT / standproc.run_name()
    run_dir.mkdir(parents=True)
    if not a.keep:
        standproc.prune_runs(standproc.REGRESS_OUT)
    stand = None
    signal.signal(signal.SIGTERM, lambda *_: (_ for _ in ()).throw(KeyboardInterrupt()))
    t0 = time.monotonic()
    print(f"прогон: {run_dir}", flush=True)
    try:
        src = standproc.prepare_tree(run_dir, tree)
        if a.up:
            binp, dist = standproc.build(src, run_dir)
            phase("сборка (фронт и программа)", t0)
        data = standproc.seed_data(src, run_dir)
        if a.up:
            stand = standproc.StandProc(binp, dist, run_dir, a.port, a.stand)
            stand.start()
            phase("стенд поднят", t0)
        base.RunEnv.run_dir, base.RunEnv.data, base.RunEnv.slow = run_dir, data, slow
        with base.RegressStand(a.port, run_dir, headed=a.headed, wait_s=60.0 * slow) as st:
            base.RunEnv.st = st
            base.RunEnv.seed = base.apply_seed(st)
            runs = []
            for i in range(a.repeat):
                t1 = time.monotonic()
                res = run_suite(a.pattern)
                ok = res.wasSuccessful()
                print(f"повтор {i + 1}/{a.repeat}: {'OK' if ok else 'ПРОВАЛ'}, тестов {res.testsRun}, {time.monotonic() - t1:.1f} с", flush=True)
                runs.append(res)
            phase("тесты", t0)
        env = {"коммит": commit_of(tree or common.ROOT), "дерево": str(tree) if tree else "срез HEAD", "SA_TEST_SLOW": slow, "порт": a.port}
        write_report(run_dir, runs, env)
        return 0 if all(r.wasSuccessful() for r in runs) else 1
    finally:
        if stand is not None:
            stand.stop()
            print("стенд остановлен, порт свободен", flush=True)
        base.RunEnv.st = None


def check_attach_free(port: int) -> None:
    """--attach: на порту должен кто-то слушать (иначе отказ понятным словом, а не таймаутом Playwright)."""
    import socket

    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.settimeout(2)
        if s.connect_ex(("127.0.0.1", port)) != 0:
            raise Refuse(f"на порту {port} никто не слушает: поднимите стенд (scripts/stand.sh) или возьмите --up")


if __name__ == "__main__":
    sys.exit(common.run_main(main))
