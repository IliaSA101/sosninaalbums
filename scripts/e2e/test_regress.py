"""Юнит-тесты чистых частей e2e-регресса (166): имена и отбор прогонов, порт, дерево, Linux-ветка config_root, стражи.

Без браузера и стенда: `python -m unittest discover -s scripts/e2e`. Тесты самих сценариев (regress/reg_*.py)
этот discover не видит (в regress/ нет __init__.py) — их запускает regress.py.
"""

import ast
import contextlib
import datetime as dt
import io
import json
import os
import shutil
import signal
import socket
import subprocess
import sys
import tempfile
import types
import unittest
from pathlib import Path
from unittest import mock

import common
import driver
import regress
import standproc
from common import Refuse

CASES = Path(__file__).resolve().parent / "regress"
if str(CASES) not in sys.path:
    sys.path.insert(0, str(CASES))
import base  # noqa: E402


class RunsTest(unittest.TestCase):
    def test_run_name_matches_pattern(self):
        n = standproc.run_name(dt.datetime(2026, 11, 4, 9, 5, 7))
        self.assertEqual(n, "20261104-090507")
        self.assertTrue(standproc.RUN_RX.match(n))

    def test_stale_runs_only_ours_keep_last(self):
        names = [f"2026110{i}-000000" for i in range(1, 8)] + ["notes", "20261104", "keepme-20261101-000000", "20261107-000000-2"]
        later = dt.datetime(2026, 12, 1)
        gone = standproc.stale_runs(names, keep=5, now=later)
        self.assertEqual(gone, ["20261101-000000", "20261102-000000", "20261103-000000"])
        self.assertEqual(standproc.stale_runs(names[:3], keep=5, now=later), [])

    def test_stale_runs_skips_alive_and_young(self):
        names = ["20261101-000000", "20261102-000000", "20261103-000000", "20261104-000000"]
        now = dt.datetime(2026, 11, 4, 1, 0, 0)  # 20261104 моложе 2 ч, остальные старше
        self.assertEqual(standproc.stale_runs(names, keep=0, now=now), names[:3])
        self.assertEqual(standproc.stale_runs(names, keep=0, now=now, alive=lambda n: n == "20261102-000000"), ["20261101-000000", "20261103-000000"])
        young = dt.datetime(2026, 11, 3, 1, 59, 0)  # от 20261103-000000 меньше 2 ч
        self.assertEqual(standproc.stale_runs(names[:3], keep=0, now=young), names[:2])

    def test_make_run_dir_suffix_same_second(self):
        with tempfile.TemporaryDirectory() as t:
            now = dt.datetime(2026, 11, 4, 9, 5, 7)
            a, b, c = (standproc.make_run_dir(Path(t), now) for _ in range(3))
            self.assertEqual([a.name, b.name, c.name], ["20261104-090507", "20261104-090507-2", "20261104-090507-3"])
            self.assertTrue(all(standproc.RUN_RX.match(n) for n in (a.name, b.name, c.name)))
            self.assertEqual(sorted([c.name, a.name, b.name]), [a.name, b.name, c.name])

    def test_prune_skips_live_pid(self):
        dead = subprocess.Popen([sys.executable, "-c", "pass"])
        dead.wait()
        with tempfile.TemporaryDirectory() as t:
            base_dir = Path(t)
            for n in ("20200101-000000", "20200102-000000", "20200103-000000", "20200104-000000"):
                (base_dir / n).mkdir()
            (base_dir / "20200101-000000" / "run.pid").write_text(str(os.getpid()))  # скрипт прогона жив
            (base_dir / "20200102-000000" / "stand.pid").write_text(str(os.getpid()))  # оставленный --keep стенд жив
            (base_dir / "20200103-000000" / "run.pid").write_text(str(dead.pid))  # процесс умер
            (base_dir / "notes").mkdir()
            gone = standproc.prune_runs(base_dir, keep=1, now=dt.datetime(2026, 11, 4))
            self.assertEqual(gone, ["20200103-000000"])
            self.assertEqual(sorted(p.name for p in base_dir.iterdir()), ["20200101-000000", "20200102-000000", "20200104-000000", "notes"])

    def test_pid_alive(self):
        self.assertTrue(standproc.pid_alive(os.getpid()))
        self.assertFalse(standproc.pid_alive(0))
        self.assertFalse(standproc.pid_alive(-5))

    def test_huge_pid_not_crash(self):
        self.assertFalse(standproc.pid_alive(2**40))
        self.assertFalse(standproc.pid_alive(2**31 + 5))  # OverflowError в os.kill
        with tempfile.TemporaryDirectory() as t:
            run = Path(t)
            (run / "run.pid").write_text(str(2**40))
            (run / "stand.pid").write_text("99999999999999999999999")
            self.assertFalse(standproc.run_alive(run))
            (run / "run.pid").write_text("мусор")
            self.assertFalse(standproc.run_alive(run))

    def fake_ctypes(self, handle, exit_code=None, last_error=0, got_code=True):
        box = types.SimpleNamespace(value=0)
        k32 = mock.MagicMock()
        k32.OpenProcess.return_value = handle

        def get_code(_h, _ref):
            box.value = exit_code
            return got_code

        k32.GetExitCodeProcess.side_effect = get_code
        ct = mock.MagicMock()
        ct.WinDLL.return_value = k32  # свой экземпляр kernel32, не общий windll
        ct.get_last_error.return_value = last_error
        wt = mock.MagicMock()
        wt.DWORD = lambda: box
        ct.wintypes = wt
        return ct, wt, k32

    def nt_alive(self, **kw):
        ct, wt, k32 = self.fake_ctypes(**kw)
        with mock.patch.object(standproc, "IS_NT", True), mock.patch.object(standproc.os, "kill", side_effect=AssertionError("os.kill на Windows = TerminateProcess")), \
                mock.patch.dict(sys.modules, {"ctypes": ct, "ctypes.wintypes": wt}):
            k32.ct = ct
            return standproc.pid_alive(4242), k32

    def test_pid_alive_windows_never_kills(self):
        alive, k32 = self.nt_alive(handle=77, exit_code=standproc.STILL_ACTIVE)
        self.assertTrue(alive)
        k32.CloseHandle.assert_called_once_with(77)  # дескриптор закрыт
        alive, k32 = self.nt_alive(handle=77, exit_code=1)  # процесс завершился
        self.assertFalse(alive)
        k32.CloseHandle.assert_called_once_with(77)
        self.assertFalse(self.nt_alive(handle=0, last_error=standproc.ERROR_INVALID_PARAMETER)[0])  # нет такого PID
        self.assertTrue(self.nt_alive(handle=0, last_error=5)[0])  # отказ в доступе: считаем живым
        _alive, k32 = self.nt_alive(handle=77, exit_code=standproc.STILL_ACTIVE)
        k32.ct.WinDLL.assert_called_once_with("kernel32", use_last_error=True)  # свой экземпляр с кодом ошибки потока
        self.assertEqual(len(k32.OpenProcess.argtypes), 3)  # типы заданы на этом экземпляре
        self.assertEqual(len(k32.GetExitCodeProcess.argtypes), 2)
        self.assertTrue(self.nt_alive(handle=77, exit_code=1, got_code=False)[0])  # код выхода не получен: живым
        with mock.patch.object(standproc, "IS_NT", True), mock.patch.dict(sys.modules, {"ctypes": None}):  # нет ctypes: живым
            self.assertTrue(standproc.pid_alive(4242))

    def test_tail(self):
        self.assertEqual(standproc.tail("a\nb\nc", 2), "b\nc")
        self.assertEqual(standproc.tail("", 5), "")


class PortTest(unittest.TestCase):
    def test_owner_port_refused(self):
        with self.assertRaises(Refuse):
            standproc.check_regress_port(34115)

    def test_busy_port_refused(self):
        with socket.socket() as s:
            s.bind(("127.0.0.1", 0))
            s.listen(1)
            port = s.getsockname()[1]
            with mock.patch.object(standproc, "check_port", lambda p: p):
                with self.assertRaises(Refuse):
                    standproc.check_regress_port(port)

    def test_default_port_in_range(self):
        self.assertEqual(common.check_port(standproc.DEFAULT_PORT), 34117)


class StandEnvTest(unittest.TestCase):
    def test_xdg_inside_run(self):
        run = standproc.REGRESS_OUT / "20261104-000000"
        with mock.patch.dict(os.environ, {"XDG_DATA_HOME": "/outside/data", "XDG_STATE_HOME": "/outside/state", "XDG_CONFIG_HOME": "/outside/cfg"}):
            env = standproc.stand_env(run, "Regress")
        # Писать стенд может только в *_HOME; XDG_*_DIRS и XDG_RUNTIME_DIR — системные пути поиска, их не подменяем.
        keys = [k for k in env if k.startswith("XDG_") and k.endswith("_HOME")]
        self.assertEqual(set(keys), {"XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME"})
        for k in keys:
            self.assertIn(run.resolve(), Path(env[k]).parents, f"{k}={env[k]} вне папки прогона")
        self.assertEqual(env["SA_STAND"], "regress")

    def test_empty_root_refused(self):
        for bad in (Path(""), Path("relative/run"), Path("/tmp/outside-run"), standproc.REGRESS_OUT):
            with self.assertRaises(Refuse, msg=str(bad)):
                standproc.stand_env(bad, "regress")

    def test_start_refuses_foreign_run_dir(self):
        with tempfile.TemporaryDirectory() as t:
            sp = standproc.StandProc(Path("bin"), Path("dist"), Path(t), 34120, "x")
            with self.assertRaises(Refuse):
                sp.start()  # до создания папок и запуска
            self.assertEqual(list(Path(t).iterdir()), [])
            self.assertIsNone(sp.proc)

    def test_run_dir_inside_gotmp(self):
        self.assertIn(common.ROOT / ".gotmp", standproc.REGRESS_OUT.parents)


class ToolsTest(unittest.TestCase):
    def fake_which(self, have):
        return mock.patch.object(shutil, "which", lambda t: f"/bin/{t}" if t in have else None)

    def test_missing_tools(self):
        with self.fake_which({"go"}), mock.patch.dict(sys.modules, {"playwright": mock.MagicMock(), "playwright.sync_api": mock.MagicMock()}):
            with self.assertRaises(Refuse) as cm:
                standproc.need_tools(Path("/nonexistent"))
            self.assertIn("xvfb-run", str(cm.exception))
        with self.fake_which({"go"}), mock.patch.dict(sys.modules, {"playwright": None, "playwright.sync_api": None}):
            with self.assertRaises(Refuse) as cm:
                standproc.need_tools(Path("/nonexistent"), attach=True)
            self.assertIn("Playwright", str(cm.exception))

    def test_attach_needs_only_go_and_playwright(self):
        with self.fake_which({"go"}), mock.patch.dict(sys.modules, {"playwright": mock.MagicMock(), "playwright.sync_api": mock.MagicMock()}):
            standproc.need_tools(Path("/nonexistent"), attach=True)  # ни xvfb, ни tar, ни node_modules, ни дерева
        with self.fake_which(set()), mock.patch.dict(sys.modules, {"playwright": mock.MagicMock(), "playwright.sync_api": mock.MagicMock()}):
            with self.assertRaises(Refuse):
                standproc.need_tools(Path("/nonexistent"), attach=True)


class ArgsTest(unittest.TestCase):
    def test_default_stand_name_per_port(self):
        self.assertEqual(regress.parse_args(["--up", "--port", "34120"]).stand, "regress34120")
        self.assertEqual(regress.parse_args(["--up"]).stand, f"regress{standproc.DEFAULT_PORT}")
        self.assertEqual(regress.parse_args(["--up", "--port", "34120", "--stand", "X1"]).stand, "x1")

    def test_attach_rules(self):
        self.assertEqual(regress.parse_args(["--attach", "--port", "34116", "--stand", "b"]).stand, "b")
        for argv in (["--attach"], ["--attach", "--port", "34116"], ["--attach", "--port", "34116", "--stand", "b", "--keep"],
                     ["--attach", "--port", "34116", "--stand", "b", "--tree", "/x"], ["--up", "--port", "34115"], ["--up", "--repeat", "0"]):
            with self.assertRaises(Refuse, msg=str(argv)):
                regress.parse_args(argv)


class RunTest(unittest.TestCase):
    def test_failed_command_refused_with_tail(self):
        with self.assertRaises(Refuse) as cm:
            standproc.run([sys.executable, "-c", "import sys; print('boom-line'); sys.exit(3)"], Path.cwd(), "сборка")
        self.assertIn("boom-line", str(cm.exception))
        self.assertIn("код 3", str(cm.exception))

    def test_tree_must_be_same_repo(self):
        self.assertIsNone(standproc.check_tree(None))
        with self.assertRaises(Refuse):
            standproc.check_tree("relative/path")
        with self.assertRaises(Refuse):
            standproc.check_tree("/")


@unittest.skipUnless(sys.platform.startswith("linux"), "ветка Linux")
class ConfigRootLinuxTest(unittest.TestCase):
    def test_xdg_then_home_then_refuse(self):
        with mock.patch.dict(os.environ, {"XDG_CONFIG_HOME": "/x/cfg", "HOME": "/h"}):
            self.assertEqual(common.config_root(), Path("/x/cfg"))
            self.assertEqual(common.stand_dir("Regress").name, "SosninaAlbums-stand-regress")
        with mock.patch.dict(os.environ, {"XDG_CONFIG_HOME": "", "HOME": "/h"}):
            self.assertEqual(common.config_root(), Path("/h/.config"))
        with mock.patch.dict(os.environ, {"XDG_CONFIG_HOME": " ", "HOME": ""}):
            with self.assertRaises(Refuse):
                common.config_root()


class SeedTest(unittest.TestCase):
    def test_seed_plan_hides_start_and_tours(self):
        plan = dict((m, a) for m, a in base.seed_prefs_plan() if m != "SetTour")
        self.assertEqual(plan["SetShowStart"], (False,))
        tours = [a for m, a in base.seed_prefs_plan() if m == "SetTour"]
        self.assertIn(("first", "done"), tours)
        self.assertIn(("spread", "done"), tours)

    def test_bug_passport(self):
        @base.bug(id="R0", fix="abc", src="s", red="r")
        class C(base.RegressCase):
            def test_x(self):
                pass

        self.assertEqual(C.BUG["id"], "R0")


def banned_in(source: str, banned: set[str]) -> list[tuple[int, str]]:
    """Запрещённые имена в исходнике: обращение, имя, импорт (в том числе под псевдонимом) и строка-имя для getattr."""
    found = []
    for node in ast.walk(ast.parse(source)):
        names = []
        if isinstance(node, ast.Attribute):
            names = [node.attr]
        elif isinstance(node, ast.Name):
            names = [node.id]
        elif isinstance(node, ast.alias):
            names = [node.name.split(".")[-1]]  # from time import sleep as s: имя до «as»
        elif isinstance(node, ast.Constant) and isinstance(node.value, str):
            names = [node.value]
        found += [(getattr(node, "lineno", 0), n) for n in names if n in banned]
    return found


def bug_ids(path: Path) -> list[tuple[str, str]]:
    """(класс, id из @bug(id=...)) для каждого класса файла."""
    out = []
    for node in ast.parse(path.read_text(encoding="utf-8")).body:
        if isinstance(node, ast.ClassDef):
            for d in node.decorator_list:
                if isinstance(d, ast.Call) and isinstance(d.func, ast.Name) and d.func.id == "bug":
                    out += [(node.name, k.value.value) for k in d.keywords if k.arg == "id" and isinstance(k.value, ast.Constant)]
    return out


class GuardTest(unittest.TestCase):
    """Страж: в сценариях нет sleep / wait_for_timeout; у каждого класса с тестами есть @bug; id багов не повторяются."""

    BANNED = {"sleep", "wait_for_timeout"}

    def files(self):
        return sorted(CASES.glob("reg_*.py"))

    def test_no_sleep_in_cases(self):
        for f in self.files():
            for line, name in banned_in(f.read_text(encoding="utf-8"), self.BANNED):
                self.fail(f"{f.name}:{line}: {name} в сценарии запрещён (design §4)")

    def test_every_case_class_has_bug(self):
        for f in self.files():
            for node in ast.parse(f.read_text(encoding="utf-8")).body:
                if isinstance(node, ast.ClassDef) and any(isinstance(n, ast.FunctionDef) and n.name.startswith("test_") for n in node.body):
                    decos = [d.func.id for d in node.decorator_list if isinstance(d, ast.Call) and isinstance(d.func, ast.Name)]
                    self.assertIn("bug", decos, f"{f.name}: класс {node.name} без @bug")

    def test_unique_ids(self):
        seen: dict[str, str] = {}
        for f in self.files():
            for cls, bid in bug_ids(f):
                self.assertNotIn(bid, seen, f"id {bid}: {f.name}.{cls} и {seen.get(bid)}")
                seen[bid] = f"{f.name}.{cls}"
        self.assertTrue({"R1", "R2", "R3", "R4", "R5", "R6", "R7", "R8"} <= set(seen), f"нет части R-тестов: {sorted(seen)}")

    def test_guard_catches_sleep_forms(self):
        for src in ("def test_a(self):\n    self.st.sleep(1)\n", "from time import sleep as s\ns(1)\n", "import time\ntime.sleep(1)\n",
                    "from time import sleep\n", "self.page.wait_for_timeout(5)\n", "getattr(st, 'sleep')(1)\n"):
            self.assertTrue(banned_in(src, self.BANNED), src)
        self.assertEqual(banned_in("from time import monotonic as m\nm()\n", self.BANNED), [])


def sub_fail_case():
    """Тест с провалом одного subTest; внутри функции, чтобы discover не принял его за настоящий."""

    class SubFail(unittest.TestCase):
        BUG = {"id": "RX", "src": "тест"}

        def test_x(self):
            for side in ("a", "b"):
                with self.subTest(side=side):
                    if side == "b":
                        self.fail("boom")

    return SubFail


def fine_case():
    class Fine(unittest.TestCase):
        def test_ok(self):
            pass

    return Fine


def run_case(cls) -> "regress.Timed":
    suite = unittest.TestLoader().loadTestsFromTestCase(cls)
    return unittest.TextTestRunner(resultclass=regress.Timed, stream=io.StringIO()).run(suite)


class ReportTest(unittest.TestCase):
    def test_codes(self):
        ok, bad = run_case(fine_case()), run_case(sub_fail_case())
        self.assertEqual(regress.exit_code([ok]), 0)
        self.assertEqual(regress.exit_code([ok, bad]), 1)  # провал любого повтора
        err = io.StringIO()
        with tempfile.TemporaryDirectory() as t, mock.patch.object(standproc, "REGRESS_OUT", Path(t) / "regress"):  # общую папку не трогаем
            with contextlib.redirect_stderr(err):
                self.assertEqual(common.run_main(lambda: regress.main(["--up", "--port", "34115"])), 2)  # порт владельца: отказ до записи
            self.assertEqual(list(Path(t).iterdir()), [])
        self.assertIn("34115", err.getvalue())

    def test_subtest_failure_in_report(self):
        res = run_case(sub_fail_case())
        self.assertFalse(res.wasSuccessful())
        rows = [r for r in res.rows if r["status"] == "FAIL"]
        self.assertEqual(len(rows), 1, res.rows)
        self.assertIn("side='b'", rows[0]["test"])
        self.assertEqual(rows[0]["bug"], "RX")  # паспорт подтеста — от его теста
        with tempfile.TemporaryDirectory() as t:
            regress.write_report(Path(t), [res], {"порт": 1})
            md = (Path(t) / "report.md").read_text(encoding="utf-8")
            data = json.loads((Path(t) / "report.json").read_text(encoding="utf-8"))
        self.assertIn("| FAIL |", md)
        self.assertEqual([r["status"] for r in data["rows"]], ["FAIL"])

    def test_prefs_trace(self):
        seed = {"level": "full", "showStart": False}
        self.assertEqual(base.prefs_trace(dict(seed), seed), [])
        self.assertEqual(base.prefs_trace({"level": "simple", "showStart": False, "extra": 1}, seed), ["extra", "level"])

        class FakeSt:
            prefs = {"level": "simple", "showStart": False}

            def call(self, name, *args):
                if name == "GetState":
                    return {"prefs": dict(self.prefs)}
                self.prefs = dict(seed)  # засев вернул настройки
                return None

        class C(base.RegressCase):
            def test_x(self):
                pass

        old = (base.RunEnv.st, base.RunEnv.seed)
        try:
            base.RunEnv.st, base.RunEnv.seed = FakeSt(), dict(seed)
            msgs = C("test_x")._restore_prefs()
            self.assertEqual(len(msgs), 1)
            self.assertIn("level", msgs[0])
            self.assertEqual(base.RunEnv.st.prefs, seed)  # возврат через API
            self.assertEqual(C("test_x")._restore_prefs(), [])  # второй тест после возврата чист
        finally:
            base.RunEnv.st, base.RunEnv.seed = old


class FakeStand:
    def __init__(self, calls, boom=False, resignal=False):
        self.calls, self.boom, self.resignal = calls, boom, resignal
        self.proc = types.SimpleNamespace(pid=999)

    def stop(self):
        self.calls.append("stop")
        if self.resignal:
            os.kill(os.getpid(), signal.SIGINT)  # второй Ctrl+C посреди остановки
            self.calls.append("stop-done")
        if self.boom:
            raise RuntimeError("не гасится")


@contextlib.contextmanager
def saved_signals():
    names = [n for n in regress.SIGNALS if hasattr(signal, n)]
    old = {n: signal.getsignal(getattr(signal, n)) for n in names}
    try:
        yield
    finally:
        for n, h in old.items():
            signal.signal(getattr(signal, n), h)


class SignalTest(unittest.TestCase):
    def fire(self, stand):
        calls = stand.calls if stand else []
        regress.LIVE["stand"] = stand
        try:
            with saved_signals():
                regress.install_signals()
                with mock.patch.object(regress.os, "_exit", lambda code: calls.append(f"exit{code}")), contextlib.redirect_stderr(io.StringIO()) as err:
                    regress._on_signal(signal.SIGINT, None)
        finally:
            regress.LIVE["stand"] = None
        return calls, err.getvalue()

    def test_signal_stops_stand_then_exits_2(self):
        calls, err = self.fire(FakeStand([]))
        self.assertEqual(calls, ["stop", "exit2"])
        self.assertIn("SIGINT", err)
        self.assertIsNone(regress.LIVE["stand"])

    def test_signal_without_stand_and_failing_stop(self):
        self.assertEqual(self.fire(None)[0], ["exit2"])
        calls, err = self.fire(FakeStand([], boom=True))
        self.assertEqual(calls, ["stop", "exit2"])
        self.assertIn("не погашен", err)

    def test_second_signal_does_not_cut_stop(self):
        calls, _ = self.fire(FakeStand([], resignal=True))
        self.assertEqual(calls, ["stop", "stop-done", "exit2"])  # остановка доведена до конца, выход один

    def test_second_signal_in_end_stand(self):
        calls: list[str] = []
        with saved_signals():
            regress.install_signals()
            with contextlib.redirect_stdout(io.StringIO()):
                regress.end_stand(FakeStand(calls, resignal=True), keep=False, port=34120)
        self.assertEqual(calls, ["stop", "stop-done"])  # без игнора обработчик вышел бы посреди stop()

    def test_sigint_is_handled(self):
        with saved_signals():
            regress.install_signals()
            for n in regress.SIGNALS:
                if hasattr(signal, n):
                    self.assertIs(signal.getsignal(getattr(signal, n)), regress._on_signal, n)

    def test_end_stand_keep_only_when_finished(self):
        calls: list[str] = []
        out = io.StringIO()
        with saved_signals(), contextlib.redirect_stdout(out):
            regress.end_stand(FakeStand(calls), keep=True, port=34120)  # --keep и тесты дошли до отчёта
            self.assertEqual(calls, [])
            regress.end_stand(FakeStand(calls), keep=False, port=34120)  # --keep, но прогон оборвался: гасим
            self.assertEqual(calls, ["stop"])
            regress.end_stand(None, keep=False, port=34120)
        self.assertIn("kill -TERM -- -999", out.getvalue())
        self.assertIn("стенд остановлен", out.getvalue())


class HangTest(unittest.TestCase):
    def test_hang_stops_stand_before_exit(self):
        calls: list[str] = []
        st = driver.Stand(34120, Path("unused"))
        st.on_hang = lambda: calls.append("stop")
        with mock.patch.object(driver.os, "_exit", lambda code: calls.append(f"exit{code}")), contextlib.redirect_stdout(io.StringIO()):
            st._hung()
        self.assertEqual(calls, ["stop", "exit2"])  # стенд гасится до выхода

    def test_failing_stop_still_exits(self):
        calls: list[str] = []
        st = driver.Stand(34120, Path("unused"))

        def boom():
            raise RuntimeError("не гасится")

        st.on_hang = boom
        with mock.patch.object(driver.os, "_exit", lambda code: calls.append(f"exit{code}")), contextlib.redirect_stdout(io.StringIO()) as out:
            st._hung()
        self.assertEqual(calls, ["exit2"])
        self.assertIn("не погашен", out.getvalue())


if __name__ == "__main__":
    unittest.main()
