"""Юнит-тесты чистых частей e2e (089): отчёт, пороги, журнал, сравнение, охрана аргументов, профили.

Без браузера и без стенда: `python -m unittest discover -s scripts/e2e`.
Временные файлы — в <корень>/.gotmp/e2e-test (не в корне диска и не в %TEMP% агента).
"""

import argparse
import contextlib
import io
import json
import os
import shutil
import tempfile
import unittest
from pathlib import Path
from unittest import mock

import big
import common
import driver
import report
import scalejs
from common import Refuse
from report import GATE, TARGET, Report


def make(slow: float = 1.0, **kw) -> Report:
    return Report("stand", "2026-10-20T08:00:00", slow=slow, **kw)


class TmpCase(unittest.TestCase):
    def tmp(self) -> Path:
        base = common.ROOT / ".gotmp" / "e2e-test"
        base.mkdir(parents=True, exist_ok=True)
        d = Path(tempfile.mkdtemp(dir=base))
        self.addCleanup(shutil.rmtree, d, ignore_errors=True)
        return d


class GateTargetTest(unittest.TestCase):
    def test_gate_fail_exit_1(self):
        r = make()
        r.add("list_total_ms", 1500, kind=GATE, limit=1000, time=True)
        r.add("dom_nodes", 100, kind=GATE, limit=4000)
        self.assertEqual(r.exit_code(), 1)
        self.assertEqual([m.id for m in r.failed()], ["list_total_ms"])
        md = report.to_md(r)
        self.assertIn("ПРОВАЛ", md)
        self.assertIn("list_total_ms", md)

    def test_gate_green_exit_0(self):
        r = make()
        r.add("dom_nodes", 4000, kind=GATE, limit=4000)  # граница — не провал (≤)
        self.assertEqual(r.exit_code(), 0)
        self.assertIn("gate зелёные", report.to_md(r))

    def test_target_not_failure(self):
        r = make()
        m = r.add("visible_wait_ms", 9000, kind=TARGET, limit=5000, time=True)
        self.assertFalse(m.ok)
        self.assertIn("цель не достигнута", m.note)
        self.assertEqual(r.exit_code(), 0)
        self.assertEqual(r.failed(), [])
        self.assertIn("цель не достигнута", report.to_md(r))

    def test_info_and_unmeasured_never_fail(self):
        r = make()
        r.add("list_ms", 99999)
        m = r.add("js_heap_mb", None, kind=GATE, limit=250)
        self.assertIsNone(m.ok)
        self.assertEqual(m.note, "не измерено")
        self.assertEqual(r.exit_code(), 0)

    def test_kind_validation(self):
        r = make()
        with self.assertRaises(ValueError):
            r.add("x", 1, kind="strict", limit=1)
        with self.assertRaises(ValueError):
            r.add("x", 1, kind=GATE)  # у gate нужен порог

    def test_slow_factor(self):
        r = make(slow=2.0)
        t = r.add("edit_frame_ms", 110, kind=GATE, limit=60, time=True)  # 60 × 2 = 120
        self.assertEqual(t.limit, 120)
        self.assertTrue(t.ok)
        n = r.add("dom_nodes", 4100, kind=GATE, limit=4000)  # не время — не множится
        self.assertEqual(n.limit, 4000)
        self.assertFalse(n.ok)
        self.assertEqual(common.slow_factor({}), 1.0)
        self.assertEqual(common.slow_factor({"SA_TEST_SLOW": ""}), 1.0)
        self.assertEqual(common.slow_factor({"SA_TEST_SLOW": "4"}), 4.0)
        for bad in ("x", "0.5", "-1"):
            with self.assertRaises(Refuse):
                common.slow_factor({"SA_TEST_SLOW": bad})

    def test_report_json_shape(self):
        r = make(count=600, machine={"cpu": "X", "threads": 12, "ramGB": 64})
        r.add("thumbs_per_photo_ms", 281.456, kind=GATE, limit=400, time=True)
        d = json.loads(json.dumps(r.as_json()))
        self.assertEqual(set(d), {"level", "started", "quick", "count", "slow", "machine", "metrics"})
        self.assertEqual(set(d["metrics"][0]), {"id", "value", "limit", "kind", "ok", "note"})
        self.assertEqual(d["metrics"][0]["value"], 281.46)
        self.assertEqual(d["count"], 600)

    def test_percentile(self):
        self.assertEqual(driver.percentile([], 95), 0.0)
        self.assertEqual(driver.percentile([5], 95), 5)
        xs = list(range(1, 101))
        self.assertEqual(driver.percentile(xs, 95), 95)
        self.assertEqual(driver.percentile(xs, 100), 100)
        self.assertEqual(driver.percentile([16.7] * 99 + [200.0], 95), 16.7)


class WriteTest(TmpCase):
    def test_write_report_files(self):
        out = self.tmp() / "run"
        r = make()
        r.add("dom_nodes", 10, kind=GATE, limit=4000)
        report.write_report(r, out)
        self.assertTrue((out / "report.json").is_file())
        self.assertIn("| dom_nodes |", (out / "report.md").read_text(encoding="utf-8"))
        self.assertEqual(report.load_report(out)["metrics"][0]["id"], "dom_nodes")
        self.assertEqual(report.load_report(out / "report.json")["level"], "stand")

    def test_load_report_rejects_garbage(self):
        d = self.tmp()
        (d / "report.json").write_text("{}", encoding="utf-8")
        with self.assertRaises(ValueError):
            report.load_report(d)


class LogParseTest(unittest.TestCase):
    LOG = "\n".join(
        [
            "2026/10/20 07:15:53 папок 1: 217 фото (обход 3ms, чтение 835ms, из памяти 0), пропущено 0",
            "2026/10/20 07:16:00 миниатюры: 600 за 2m3.5s",
            "2026/10/20 07:16:01 память: после прогрева — Go 1180 МБ, куча 310 МБ",
            "2026/10/20 07:16:02 прогрев раскладок: 150 разворотов за 4.2s",
            "2026/10/20 07:16:03 автосборка: 600 фото → 150 разворотов за 1.25s",
            "2026/10/20 07:16:04 экспорт: 10 разворотов за 30s",
            "2026/10/20 07:16:05 сообщение без события",
        ]
    )

    def test_log_parse(self):
        ev = report.parse_log(self.LOG)
        self.assertEqual(ev["folders"], [{"photos": 217, "walk_ms": 3.0, "read_ms": 835.0}])
        self.assertEqual(ev["thumbs"], [{"photos": 600, "ms": 123500.0, "cancelled": 0}])
        self.assertEqual(ev["memory"], [{"stage": "после прогрева", "go_mb": 1180, "heap_mb": 310}])
        self.assertEqual(ev["layouts"], [{"spreads": 150, "ms": 4200.0}])
        self.assertEqual(ev["autobuild"], [{"photos": 600, "spreads": 150, "ms": 1250.0}])
        self.assertEqual(ev["export"], [{"spreads": 10, "ms": 30000.0}])

    def test_log_parse_cancelled(self):
        # 090 G2: хвост со снятыми запросами не ломает время и даёт их число.
        ev = report.parse_log("2026/10/20 07:16:00 миниатюры: 600 за 1m2.5s, снято запросов 42\nминиатюры: 3 за 850ms")
        self.assertEqual(ev["thumbs"], [{"photos": 600, "ms": 62500.0, "cancelled": 42}, {"photos": 3, "ms": 850.0, "cancelled": 0}])

    def test_log_parse_empty(self):
        ev = report.parse_log("")
        self.assertTrue(all(v == [] for v in ev.values()))

    def test_parse_duration(self):
        cases = {"0s": 0, "0": 0, "850ms": 850, "1.234s": 1234, "1m3.5s": 63500, "1h2m": 3720000, "12µs": 0.012, "300ns": 0.0003}
        for s, ms in cases.items():
            self.assertAlmostEqual(report.parse_duration(s), ms, places=6, msg=s)
        for bad in ("", "abc", "5"):
            with self.assertRaises(ValueError):
                report.parse_duration(bad)

    def test_bad_lines(self):
        text = "всё хорошо\nПАНИКА в горутине\nошибка чтения\nне удалось открыть\nmedia: фото не найдено"
        self.assertEqual(len(report.bad_lines(text)), 3)
        self.assertEqual(report.bad_lines(""), [])


class CompareTest(TmpCase):
    @staticmethod
    def rep(**vals) -> dict:
        return {"level": "stand", "started": "t", "metrics": [{"id": k, "value": v, "kind": "info"} for k, v in vals.items()]}

    def test_compare_worse(self):
        old = self.rep(a=100, b=100, c=100, d=0, e=None, f=100)
        new = self.rep(a=150, b=149, c=40, d=50, e=9, g=500)
        worse = report.compare(old, new)
        self.assertEqual([w["id"] for w in worse], ["a"])  # 1,5 и больше; 1,49 нет; лучше нет; old=0, None и новые — пропуск
        self.assertEqual(worse[0]["ratio"], 1.5)
        self.assertIn("хуже  a", report.compare_text(old, new))

    def test_compare_none_worse(self):
        text = report.compare_text(self.rep(a=100), self.rep(a=101))
        self.assertIn("нет", text)

    def test_compare_cli_exit_0_even_when_worse(self):
        d = self.tmp()
        for name, v in (("old", 100), ("new", 300)):
            (d / name).mkdir()
            (d / name / "report.json").write_text(json.dumps(self.rep(a=v)), encoding="utf-8")
        buf = io.StringIO()
        with contextlib.redirect_stdout(buf):
            code = report.main(["--compare", str(d / "old"), str(d / "new")])
        self.assertEqual(code, 0)
        self.assertIn("a", buf.getvalue())

    def test_compare_cli_missing_file_exit_2(self):
        err = io.StringIO()
        with contextlib.redirect_stderr(err):
            code = report.main(["--compare", "нет-такого", "нет-такого-2"])
        self.assertEqual(code, 2)


class GuardTest(TmpCase):
    def test_profiles(self):
        p = common.PROFILES
        self.assertEqual((p["A"].w, p["A"].h, p["A"].dpr), (1280, 760, 1.0))
        self.assertEqual((p["B"].w, p["B"].h, p["B"].dpr), (1536, 785, 1.25))
        self.assertEqual((p["C"].w, p["C"].h, p["C"].dpr), (1280, 641, 1.5))
        self.assertEqual((p["D"].w, p["D"].h, p["D"].dpr), (1707, 881, 1.5))
        self.assertEqual([x.key for x in common.parse_profiles(common.DEFAULT_PROFILES)], ["A", "B", "C"])
        self.assertEqual([x.key for x in common.parse_profiles("c, a, c")], ["C", "A"])  # регистр, повтор
        for bad in ("", " , ", "Z", "A,Q"):
            with self.assertRaises(Refuse):
                common.parse_profiles(bad)

    def test_port_guard(self):
        for bad in (34115, 80, 34200, 0, -1):
            with self.assertRaises(Refuse, msg=str(bad)):
                common.check_port(bad)
        self.assertEqual(common.check_port(34116), 34116)
        self.assertEqual(common.check_port(34199), 34199)
        self.assertIn("владельца", str(self.refusal(common.check_port, 34115)))
        # весь путь big.run: отказ до браузера и сети, код 2, ничего не записано
        args = argparse.Namespace(port=34115, stand="e2e", out=None, quick=True, photos=None, headed=False, cold=False, warm_wait=1, album=None)
        err = io.StringIO()
        with contextlib.redirect_stderr(err):
            self.assertEqual(common.run_main(lambda: big.run(args)), 2)
        self.assertIn("34115", err.getvalue())

    @staticmethod
    def refusal(fn, *a) -> Refuse:
        try:
            fn(*a)
        except Refuse as e:
            return e
        raise AssertionError("отказа не было")

    def test_stand_name(self):
        self.assertEqual(common.check_stand("E2E"), "e2e")
        for bad in ("", "  ", "a b", "a_b", "x" * 17, "стенд"):
            with self.assertRaises(Refuse, msg=repr(bad)):
                common.check_stand(bad)

    def test_out_required(self):
        base = self.tmp() / "e2e"
        base.mkdir()
        inside = base / "run1"
        self.assertEqual(common.out_dir(str(inside), "stand", base=base), inside.resolve())
        self.assertEqual(common.out_dir(str(base), "stand", base=base), base.resolve())
        for bad in ("", "   ", "rel/path", str(base.parent), str(base.parent / "other"), str(base / ".." / "x")):
            with self.assertRaises(Refuse, msg=repr(bad)):
                common.out_dir(bad, "stand", base=base)
        import datetime as dt

        auto = common.out_dir(None, "scale", dt.datetime(2026, 10, 20, 8, 5, 9), base)
        self.assertEqual(auto, base.resolve() / "20261020-080509-scale")
        # корень по умолчанию — внутри проекта, а не в корне диска
        self.assertIn(".gotmp", common.E2E_OUT.parts)

    def test_empty_env_refused_not_cwd(self):
        with mock.patch.dict(os.environ, {"APPDATA": ""}):
            with self.assertRaises(Refuse):
                common.config_root()
            with self.assertRaises(Refuse):
                common.stand_dir("e2e")
        with mock.patch.dict(os.environ, {"APPDATA": "C:/Users/x/AppData/Roaming"}):
            self.assertEqual(common.stand_dir("E2E").name, "SosninaAlbums-stand-e2e")

    def test_photo_dirs(self):
        root = self.tmp()
        with self.assertRaises(Refuse):
            common.photo_dirs(None, True, root=root)  # набора нет
        (root / "a").mkdir()
        (root / "b").mkdir()
        self.assertEqual([p.name for p in common.photo_dirs(None, True, root=root)], ["a"])
        self.assertEqual([p.name for p in common.photo_dirs(None, False, root=root)], ["a", "b"])
        with self.assertRaises(Refuse):
            common.photo_dirs(["relative"], True)
        with self.assertRaises(Refuse):
            common.photo_dirs([""], True)

    def test_count_photos_and_tail(self):
        root = self.tmp()
        (root / "s").mkdir()
        for n in ("1.jpg", "2.JPEG", "3.png", "s/4.jpg"):
            (root / n).write_bytes(b"x")
        self.assertEqual(common.count_photos([root]), 3)
        log = root / "log.txt"
        self.assertEqual(common.tail_from(log, 0), ("", 0))  # файла нет
        log.write_bytes(b"one\n")  # байты: без перевода строк Windows
        text, off = common.tail_from(log, 0)
        log.write_bytes(b"one\ntwo\n")
        self.assertEqual(common.tail_from(log, off)[0], "two\n")
        log.write_bytes(b"x\n")  # журнал обрезали — читаем сначала
        self.assertEqual(common.tail_from(log, 100)[0], "x\n")

    def test_parse_int(self):
        self.assertEqual(common.parse_int("  12345\r\n"), 12345)
        self.assertIsNone(common.parse_int(""))
        self.assertIsNone(common.parse_int("нет"))
        self.assertIsNone(common.peak_working_set_mb(None))


if __name__ == "__main__":
    unittest.main()


class ExpectedPxTest(unittest.TestCase):
    def test_step_256(self):
        self.assertEqual(scalejs.expected_px(600, 1), 768)
        self.assertEqual(scalejs.expected_px(256, 1), 256)
        self.assertEqual(scalejs.expected_px(63, 1.5), 256)  # 95 px: тот же шаг, адрес не меняется
        self.assertEqual(scalejs.expected_px(300, 5), 1536)
