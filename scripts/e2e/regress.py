"""e2e-регресс (166): найденные сценарии как автотесты в Chromium против настоящего Go.

  python3 scripts/e2e/regress.py --up [--port 34117] [--stand regress<порт>] [--tree <копия>] [-k имя] [--repeat N] [--keep] [--headed]
  python3 scripts/e2e/regress.py --attach --port 34116 --stand b      # стенд уже поднят stand.sh

--up собирает фронт и dev-программу из чистого среза HEAD (или из --tree), запускает её в xvfb со своими папками
данных и останавливает в конце (--keep: стенд остаётся работать для отладки). Всё лежит в .gotmp/e2e/regress/<время>/.
--attach ничего не собирает: нужны только go и Playwright. Код выхода: 0 — зелёные, 1 — провал, 2 — отказ.
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
    ap.add_argument("--stand", default=None, help="имя стенда (SA_STAND); для --up по умолчанию regress<порт>: два прогона с одним именем мешали бы друг другу (замок второго экземпляра)")
    ap.add_argument("--tree", default=None, help="рабочая копия этого репозитория вместо среза HEAD")
    ap.add_argument("-k", dest="pattern", default=None, help="только тесты, в имени которых есть эта подстрока")
    ap.add_argument("--repeat", type=int, default=1, help="сколько раз подряд прогнать набор на одном стенде")
    ap.add_argument("--keep", action="store_true", help="только с --up: не гасить стенд после прогона (порт и группа процессов печатаются)")
    ap.add_argument("--headed", action="store_true", help="показать окно браузера")
    a = ap.parse_args(argv)
    if a.repeat < 1:
        raise Refuse("--repeat: нужно число не меньше 1")
    if a.attach and a.port is None:
        raise Refuse("--attach требует --port")
    if a.attach and a.keep:
        raise Refuse("--keep относится к --up: чужой стенд скрипт не гасит")
    if a.attach and a.tree:
        raise Refuse("--tree относится к --up: к поднятому стенду дерево не применяется")
    if a.attach and a.stand is None:
        raise Refuse("--attach требует --stand: по имени берётся log.txt стенда")
    a.port = check_port(a.port if a.port is not None else standproc.DEFAULT_PORT)
    a.stand = check_stand(a.stand if a.stand is not None else f"regress{a.port}")
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
        bug = getattr(getattr(test, "test_case", test), "BUG", None) or {}  # у subTest паспорт на самом тесте
        self.rows.append({"test": test.id().split(".", 1)[-1], "bug": bug.get("id", ""), "src": bug.get("src", ""),
                          "status": status, "sec": round(time.monotonic() - self._t0, 2)})

    def addSubTest(self, test, subtest, err):
        super().addSubTest(test, subtest, err)
        if err is not None:  # зелёные подтесты отдельной строки не получают: строка теста идёт по его итогу
            self._row(subtest, "FAIL" if issubclass(err[0], test.failureException) else "ERROR")

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


def exit_code(runs: list[Timed]) -> int:
    """0 — все повторы зелёные (включая подтесты), 1 — есть провал."""
    return 0 if all(r.wasSuccessful() for r in runs) else 1


def commit_of(path: Path) -> str:
    r = subprocess.run(["git", "-C", str(path), "rev-parse", "--short", "HEAD"], capture_output=True, text=True, check=False)
    return r.stdout.strip() or "?"


def phase(name: str, t0: float) -> float:
    now = time.monotonic()
    print(f"[{now - t0:6.1f} с] {name}", flush=True)
    return now


LIVE: dict = {"stand": None}  # стенд, который гасит обработчик сигнала


SIGNALS = ("SIGINT", "SIGTERM", "SIGHUP")  # SIGHUP нет на Windows


def ignore_signals() -> None:
    """Перед остановкой стенда: повторный Ctrl+C не должен обрывать stop() (теряются SIGKILL-добивка и проверка порта).
    Остановка ограничена ~20 с, поэтому игнор безопасен."""
    for name in SIGNALS:
        if hasattr(signal, name):
            signal.signal(getattr(signal, name), signal.SIG_IGN)


def _on_signal(signum, _frame):
    """SIGINT/SIGTERM/SIGHUP: гасим стенд и выходим сразу. Исключение из обработчика посреди вызова Playwright оставляло
    его синхронный цикл в разрыве: закрытие браузера в finally зависало (проверено SIGHUP при живом стенде)."""
    ignore_signals()
    print(f"e2e: прервано сигналом {signal.Signals(signum).name}", file=sys.stderr, flush=True)
    st, LIVE["stand"] = LIVE["stand"], None
    if st is not None:
        try:
            st.stop()
        except Exception as e:  # noqa: BLE001 — выход всё равно нужен
            print(f"e2e: стенд не погашен: {e}", file=sys.stderr, flush=True)
    os._exit(2)


def install_signals() -> None:
    """Ctrl+C и закрытие терминала не должны оставлять стенд и xvfb (SIGHUP нет на Windows)."""
    for name in SIGNALS:
        if hasattr(signal, name):
            signal.signal(getattr(signal, name), _on_signal)


def end_stand(stand, keep: bool, port: int) -> None:
    """Конец прогона: --keep оставляет стенд, только если тесты дошли до отчёта; иначе стенд гасится."""
    if stand is None:
        return
    if keep:
        print(f"стенд оставлен: http://localhost:{port}, группа процессов {stand.proc.pid}; погасить: kill -TERM -- -{stand.proc.pid}", flush=True)
    else:
        ignore_signals()
        stand.stop()
        print("стенд остановлен, порт свободен", flush=True)


def main(argv: list[str] | None = None) -> int:
    a = parse_args(argv)
    tree = standproc.check_tree(a.tree)
    if a.attach:
        check_attach_free(a.port)
    else:
        standproc.check_regress_port(a.port)
    standproc.need_tools(tree or common.ROOT, attach=a.attach)
    slow = slow_factor()
    run_dir = standproc.make_run_dir(standproc.REGRESS_OUT)
    (run_dir / "run.pid").write_text(str(os.getpid()), encoding="utf-8")  # чистка чужих прогонов пропускает папки с живым PID
    standproc.prune_runs(standproc.REGRESS_OUT)
    stand = None
    finished = False
    install_signals()
    t0 = time.monotonic()
    print(f"прогон: {run_dir}", flush=True)
    try:
        if a.up:
            src = standproc.prepare_tree(run_dir, tree)
            binp, dist = standproc.build(src, run_dir)
            phase("сборка (фронт и программа)", t0)
            data = standproc.seed_data(src, run_dir)
            stand = LIVE["stand"] = standproc.StandProc(binp, dist, run_dir, a.port, a.stand)
            stand.start()
            phase("стенд поднят", t0)
        else:
            data = standproc.seed_data(common.ROOT, run_dir)  # тесты читают путь: стенд может быть из другой копии, данные от нашего qa-albums
        base.RunEnv.run_dir, base.RunEnv.data, base.RunEnv.slow = run_dir, data, slow
        base.RunEnv.log_root = run_dir / "home" if a.up else common.stand_dir(a.stand)
        with base.RegressStand(a.port, run_dir, headed=a.headed, wait_s=60.0 * slow) as st:
            if stand is not None:
                st.on_hang = stand.stop  # зависшая страница: выход с кодом 2 не оставляет стенд и xvfb
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
        env = {"коммит": commit_of(tree or common.ROOT), "дерево": str(tree) if tree else "срез HEAD" if a.up else "стенд пользователя", "SA_TEST_SLOW": slow, "порт": a.port}
        write_report(run_dir, runs, env)
        finished = True
        return exit_code(runs)
    finally:
        if stand is not None and not (a.keep and finished):
            ignore_signals()  # до LIVE=None: сигнал в этом окне иначе вышел бы, не погасив стенд
        LIVE["stand"] = None
        base.RunEnv.st = None
        end_stand(stand, a.keep and finished, a.port)


def check_attach_free(port: int) -> None:
    """--attach: на порту должен кто-то слушать (иначе отказ понятным словом, а не таймаутом Playwright)."""
    import socket

    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.settimeout(2)
        if s.connect_ex(("127.0.0.1", port)) != 0:
            raise Refuse(f"на порту {port} никто не слушает: поднимите стенд (scripts/stand.sh) или возьмите --up")


if __name__ == "__main__":
    sys.exit(common.run_main(main))
