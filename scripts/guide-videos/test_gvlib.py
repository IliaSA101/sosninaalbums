"""Тесты чистых частей роликов F1 (140, Т6): пути, охрана, окно обрезки, спутник, хеш. Без браузера и стенда.

Запуск: python3 -m unittest discover -s scripts/guide-videos -v
Временные файлы — в <корень>/.gotmp/gv-test (не в корне диска и не в %TEMP%).
"""

from __future__ import annotations

import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import gvlib  # noqa: E402
from gvlib import Refuse  # noqa: E402


class TmpCase(unittest.TestCase):
    def tmp(self) -> Path:
        base = gvlib.ROOT / ".gotmp" / "gv-test"
        base.mkdir(parents=True, exist_ok=True)
        d = Path(tempfile.mkdtemp(dir=base))
        self.addCleanup(shutil.rmtree, d, ignore_errors=True)
        return d


class IdsPortTest(unittest.TestCase):
    def test_all_and_list(self):
        self.assertEqual(gvlib.check_ids("all"), list(gvlib.IDS))
        self.assertEqual(gvlib.check_ids("cover, autobuild,cover"), ["cover", "autobuild"])

    def test_bad_ids(self):
        for spec in ("", " , ", "export", "autobuild,nope"):
            with self.assertRaises(Refuse, msg=spec):
                gvlib.check_ids(spec)

    def test_port_guard(self):
        self.assertEqual(gvlib.check_port(34117), 34117)
        for port in (34115, 34200, 0, 80):
            with self.assertRaises(Refuse, msg=str(port)):
                gvlib.check_port(port)


class OutTest(TmpCase):
    def test_main_root_of_plain_repo(self):
        d = self.tmp()
        (d / ".git").mkdir()
        self.assertEqual(gvlib.main_root(d), d)

    def test_main_root_of_worktree(self):
        d = self.tmp()
        (d / ".git").write_text("gitdir: /srv/proj/.git/worktrees/agent-1\n", encoding="utf-8")
        self.assertEqual(gvlib.main_root(d), Path("/srv/proj"))

    def test_main_root_without_git(self):
        d = self.tmp()
        self.assertEqual(gvlib.main_root(d), d)

    def test_default_out(self):
        d = self.tmp()
        (d / ".git").mkdir()
        self.assertEqual(gvlib.default_out(d), d / "docs" / "private" / "mockups" / "0.30" / "videos")

    def test_check_out_refusals(self):
        d = self.tmp()
        (d / ".git").mkdir()
        for arg in ("", "   ", "rel/dir", "/tmp", str(d / "docs")):
            with self.assertRaises(Refuse, msg=arg):
                gvlib.check_out(arg, d)

    def test_check_out_allowed(self):
        d = self.tmp()
        (d / ".git").mkdir()
        inside = d / "docs" / "private" / "mockups" / "0.30" / "videos" / "x"
        self.assertEqual(gvlib.check_out(str(inside), d), inside.resolve())
        gt = d / ".gotmp" / "gv"
        self.assertEqual(gvlib.check_out(str(gt), d), gt.resolve())
        self.assertEqual(gvlib.check_out(None, d), gvlib.default_out(d))

    def test_default_photos_missing(self):
        d = self.tmp()
        (d / ".git").mkdir()
        with self.assertRaises(Refuse):
            gvlib.default_photos(d)


class TrimTest(unittest.TestCase):
    def test_window(self):
        w = gvlib.trim_window(100.0, 103.0, 127.0)
        self.assertEqual(w["start"], 2.7)  # 3 с до метки минус запас LEAD
        self.assertEqual(w["seconds"], 24.0 + 0.3 + 0.4)

    def test_start_not_negative(self):
        w = gvlib.trim_window(100.0, 100.1, 110.0)
        self.assertEqual(w["start"], 0.0)
        self.assertAlmostEqual(w["seconds"], 9.9 + 0.1 + 0.4)  # лид — только 0,1 с до метки

    def test_bad_order(self):
        for args in ((10, 5, 20), (10, 12, 12), (10, 12, 11)):
            with self.assertRaises(ValueError, msg=str(args)):
                gvlib.trim_window(*args)

    def test_length_note(self):
        self.assertEqual(gvlib.length_note(24.0), "")
        self.assertEqual(gvlib.length_note(20.0), "")
        self.assertEqual(gvlib.length_note(40.0), "")
        self.assertIn("короче", gvlib.length_note(19.9))
        self.assertIn("длиннее", gvlib.length_note(40.1))
        self.assertIn("короче", gvlib.length_note(0))


class SidecarTest(TmpCase):
    def test_roundtrip(self):
        webm = self.tmp() / "a.webm"
        gvlib.write_sidecar(webm, {"id": "autobuild", "start": 2.7, "seconds": 24.7, "note": "«Автосборка»"})
        self.assertEqual(gvlib.read_sidecar(webm)["note"], "«Автосборка»")

    def test_missing_is_empty(self):
        self.assertEqual(gvlib.read_sidecar(self.tmp() / "none.webm"), {})

    def test_broken_and_wrong_type(self):
        d = self.tmp()
        for text in ("{", "", "[1]", "null"):
            (d / "b.json").write_text(text, encoding="utf-8")
            with self.assertRaises(Refuse, msg=text):
                gvlib.read_sidecar(d / "b.webm")


class MergeRunTest(TmpCase):
    def test_keeps_other_articles(self):
        old = {"ok": {"autobuild": {"id": "autobuild"}, "manual": {"id": "manual", "v": 1}}, "failed": {}}
        got = gvlib.merge_run(old, {"manual": {"id": "manual", "v": 2}, "cover": {"id": "cover"}}, {})
        self.assertEqual(sorted(got["ok"]), ["autobuild", "cover", "manual"])
        self.assertEqual(got["ok"]["manual"]["v"], 2)

    def test_failed_keeps_old_ok_and_success_clears_failed(self):
        old = {"ok": {"manual": {"id": "manual"}}, "failed": {"cover": "старый провал"}}
        got = gvlib.merge_run(old, {"cover": {"id": "cover"}}, {"manual": "ПРОВАЛ"})
        self.assertIn("manual", got["ok"])
        self.assertEqual(got["failed"], {"manual": "ПРОВАЛ"})
        self.assertIn("cover", got["ok"])

    def test_broken_old(self):
        for old in (None, [], "x", {"ok": [], "failed": 3}):
            self.assertEqual(gvlib.merge_run(old, {"a": {}}, {"b": "e"}), {"ok": {"a": {}}, "failed": {"b": "e"}})

    def test_read_run_missing_and_broken(self):
        d = self.tmp()
        self.assertEqual(gvlib.read_run(d / "run.json"), {})
        (d / "run.json").write_text("{не json", encoding="utf-8")
        self.assertEqual(gvlib.read_run(d / "run.json"), {})

    def test_file_info(self):
        f = self.tmp() / "a.bin"
        f.write_bytes(b"abc")
        self.assertEqual(
            gvlib.file_info(f, "u"),
            {"size": 3, "sha256": "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad", "url": "u"},
        )
        self.assertNotIn("url", gvlib.file_info(f))


class HashTest(unittest.TestCase):
    def test_fnv_vectors(self):
        # Эталонные значения FNV-1a 32 (draft-eastlake-fnv).
        self.assertEqual(gvlib.fnv1a32(""), 0x811C9DC5)
        self.assertEqual(gvlib.fnv1a32("a"), 0xE40C292C)
        self.assertEqual(gvlib.fnv1a32("foobar"), 0xBF9CF968)

    def test_article_hash_empty_fields(self):
        self.assertEqual(gvlib.article_hash("T"), f"{gvlib.fnv1a32('T' + chr(10) * 3):08x}")
        self.assertEqual(len(gvlib.article_hash("")), 8)
        self.assertNotEqual(gvlib.article_hash("T", "w"), gvlib.article_hash("T", "", "w"))

    def test_matches_node(self):
        """articlehash.mjs и Python считают одинаково (на кириллице и пустых полях): расхождение — стражу не верить."""
        node = shutil.which("node")
        if node is None:
            self.skipTest("нет node")
        code = (
            "import {articleHash} from './articlehash.mjs';"
            "const a=[{title:'Автосборка',what:'что',where:'где',how:'как'},{title:'Т'},{title:'ё',how:'х'}];"
            "console.log(a.map(articleHash).join(' '))"
        )
        r = subprocess.run([node, "--input-type=module", "-e", code], cwd=gvlib.HERE, capture_output=True, text=True, timeout=60)
        self.assertEqual(r.returncode, 0, r.stderr)
        want = [
            gvlib.article_hash("Автосборка", "что", "где", "как"),
            gvlib.article_hash("Т"),
            gvlib.article_hash("ё", "", "", "х"),
        ]
        self.assertEqual(r.stdout.split(), want)


class InsideTest(unittest.TestCase):
    VIEW = {"x": 0, "y": 100, "width": 200, "height": 300}

    def test_inside_and_edges(self):
        self.assertTrue(gvlib.inside({"x": 10, "y": 110, "width": 50, "height": 30}, self.VIEW))
        self.assertTrue(gvlib.inside({"x": 0, "y": 100, "width": 200, "height": 300}, self.VIEW))  # впритык
        self.assertFalse(gvlib.inside({"x": 10, "y": 380, "width": 50, "height": 30}, self.VIEW))  # ниже края
        self.assertFalse(gvlib.inside({"x": -1, "y": 110, "width": 50, "height": 30}, self.VIEW))

    def test_pad(self):
        box = {"x": 0, "y": 100, "width": 200, "height": 300}
        self.assertFalse(gvlib.inside(box, self.VIEW, pad=4.0))  # отступ от краёв
        self.assertTrue(gvlib.inside({"x": 4, "y": 104, "width": 192, "height": 292}, self.VIEW, pad=4.0))


if __name__ == "__main__":
    unittest.main()
