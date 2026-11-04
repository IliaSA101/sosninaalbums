"""Юнит-тесты чистых частей e2e-регресса (166): имена и отбор прогонов, порт, дерево, Linux-ветка config_root, стражи.

Без браузера и стенда: `python -m unittest discover -s scripts/e2e`. Тесты самих сценариев (regress/reg_*.py)
этот discover не видит (в regress/ нет __init__.py) — их запускает regress.py.
"""

import ast
import datetime as dt
import os
import socket
import sys
import unittest
from pathlib import Path
from unittest import mock

import common
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
        names = [f"2026110{i}-000000" for i in range(1, 8)] + ["notes", "20261104", "keepme-20261101-000000"]
        gone = standproc.stale_runs(names, keep=5)
        self.assertEqual(gone, ["20261101-000000", "20261102-000000"])
        self.assertEqual(standproc.stale_runs(names[:3], keep=5), [])

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


class GuardTest(unittest.TestCase):
    """Страж: в сценариях нет sleep / wait_for_timeout; у каждого класса с тестами есть @bug."""

    BANNED = {"sleep", "wait_for_timeout"}

    def files(self):
        return sorted(CASES.glob("reg_*.py"))

    def test_no_sleep_in_cases(self):
        for f in self.files():
            for node in ast.walk(ast.parse(f.read_text(encoding="utf-8"))):
                name = node.attr if isinstance(node, ast.Attribute) else node.id if isinstance(node, ast.Name) else ""
                self.assertNotIn(name, self.BANNED, f"{f.name}:{getattr(node, 'lineno', '?')}: {name} в сценарии запрещён (design §4)")

    def test_every_case_class_has_bug(self):
        for f in self.files():
            for node in ast.parse(f.read_text(encoding="utf-8")).body:
                if isinstance(node, ast.ClassDef) and any(isinstance(n, ast.FunctionDef) and n.name.startswith("test_") for n in node.body):
                    decos = [d.func.id for d in node.decorator_list if isinstance(d, ast.Call) and isinstance(d.func, ast.Name)]
                    self.assertIn("bug", decos, f"{f.name}: класс {node.name} без @bug")

    def test_guard_catches_sleep(self):
        tree = ast.parse("def test_a(self):\n    self.st.sleep(1)\n")
        names = {n.attr for n in ast.walk(tree) if isinstance(n, ast.Attribute)}
        self.assertTrue(names & self.BANNED)


if __name__ == "__main__":
    unittest.main()
