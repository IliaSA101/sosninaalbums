"""qa_report.py: сводка прогона win-qa из results.jsonl (спека 118, design §6). Только stdlib."""

import contextlib
import io
import json
import os
import shutil
import tempfile
import unittest

import qa_report as q

HERE = os.path.dirname(os.path.abspath(__file__))
UPDATE = os.path.join(HERE, "..", "..", "docs", "qa", "update.md")
ITEMS = [("В0-1", "первый пункт"), ("В4", "мастер по порядку"), ("В1", "Edge"), ("Г6", "код 6"), ("Г1", "лента")]


def main_quiet(argv):
    with contextlib.redirect_stdout(io.StringIO()):
        return q.main(argv)


class Report(unittest.TestCase):
    def setUp(self):
        self.dir = tempfile.mkdtemp(prefix="qa-report-")
        os.makedirs(os.path.join(self.dir, "win-qa-install", "png"))

    def tearDown(self):
        shutil.rmtree(self.dir, ignore_errors=True)

    def write(self, rows, shots=()):
        with open(os.path.join(self.dir, "win-qa-install", "results.jsonl"), "w", encoding="utf-8") as f:
            for r in rows:
                f.write(json.dumps(r, ensure_ascii=False) + "\n")
        for s in shots:
            with open(os.path.join(self.dir, "win-qa-install", s), "wb") as f:
                f.write(b"x")

    def run_report(self, blocks, items=ITEMS):
        rows = q.read_results(self.dir)
        return q.build(items, set(blocks), rows, q.read_aborts(self.dir))

    def verdicts(self, blocks, items=ITEMS):
        out, problems = self.run_report(blocks, items)
        return {e["id"]: e["verdict"] for e in out}, problems

    @unittest.skipUnless(os.path.isfile(UPDATE), "нет docs/qa/update.md (ветка winqa/* без кухни)")
    def test_parse_update_md_ids(self):
        ids = [i for i, _ in q.read_items(UPDATE)]
        self.assertGreaterEqual(len(ids), 70)
        for want in ("В0-1", "Г16", "Ж5"):
            self.assertIn(want, ids)

    @unittest.skipUnless(os.path.isfile(UPDATE), "нет docs/qa/update.md (ветка winqa/* без кухни)")
    def test_items_file_matches_update_md(self):
        self.assertEqual(q.read_items(UPDATE), q.read_items(""), "qa-items.txt устарел: обновите копией из update.md")

    def test_every_item_has_one_block(self):
        known = {i for i, _ in q.read_items("")}
        owned = [i for ids in q.BLOCKS.values() for i in ids] + q.ALWAYS_LAPTOP
        self.assertEqual(len(owned), len(set(owned)), "пункт в двух блоках")
        self.assertEqual(set(owned), known)

    def test_all_items_have_status(self):
        out, _ = self.run_report(["install"], q.read_items(""))
        self.assertEqual([e["id"] for e in out][: len(q.read_items(""))], [i for i, _ in q.read_items("")])

    def test_unknown_id_fails(self):
        self.write([{"id": "Я99", "status": "ok"}])
        _, problems = self.run_report(["install"])
        self.assertTrue(any("Я99" in p for p in problems))

    def test_fail_makes_exit_1(self):
        self.write([{"id": "В4", "status": "fail", "detail": "x"}])
        outdir = os.path.join(self.dir, "out")
        code = main_quiet(["--blocks", "install", "--in", self.dir, "--out", outdir])
        self.assertEqual(code, 1)
        self.assertTrue(os.path.isfile(os.path.join(outdir, "summary.md")))

    def test_shots_exist(self):
        self.write([{"id": "В4", "status": "eyes", "shots": ["png/V4-a.png"]}])
        v, problems = self.verdicts(["install"])
        self.assertEqual(v["В4"], q.FAIL)
        self.assertTrue(any("V4-a.png" in p for p in problems))
        self.write([{"id": "В4", "status": "eyes", "shots": ["png/V4-a.png"]}], shots=["png/V4-a.png"])
        v, problems = self.verdicts(["install"])
        self.assertEqual(v["В4"], q.EYES)
        self.assertEqual(problems, [])

    def test_unselected_block_is_skip(self):
        self.write([])
        v, _ = self.verdicts(["probe"])
        self.assertEqual(v["Г6"], q.SKIPPED)
        self.assertEqual(v["В4"], q.SKIPPED)

    def test_open_block_items_and_laptop_row(self):
        items = [("А1", "smartscreen"), ("Б7", "старая 0.24"), ("Д3", "открыть в")]
        self.write([{"id": "А1", "status": "eyes"}, {"id": "Б7", "status": "laptop"}])
        v, problems = self.verdicts(["open"], items)
        self.assertEqual(v["А1"], q.EYES)
        self.assertEqual(v["Б7"], q.LAPTOP)
        self.assertEqual(v["Д3"], q.UNCHECKED)

    def test_selected_block_without_row_is_unchecked(self):
        self.write([{"id": "В0-1", "status": "ok"}])
        v, _ = self.verdicts(["install"])
        self.assertEqual(v["В0-1"], q.OK)
        self.assertEqual(v["В4"], q.UNCHECKED)

    def test_laptop_items_not_fail(self):
        self.write([])
        v, _ = self.verdicts(["install"])
        self.assertEqual(v["В1"], q.LAPTOP)
        code = main_quiet(["--blocks", "probe", "--in", self.dir, "--out", os.path.join(self.dir, "o")])
        self.assertEqual(code, 1, "пункты probe без итогов красят прогон")

    def test_eyes_table_has_item_text(self):
        self.write([{"id": "В4", "status": "eyes", "shots": ["png/V4-a.png"]}], shots=["png/V4-a.png"])
        out = os.path.join(self.dir, "o")
        main_quiet(["--blocks", "install", "--in", self.dir, "--out", out])
        with open(os.path.join(out, "eyes.md"), encoding="utf-8") as f:
            text = f.read()
        self.assertIn("png/V4-a.png", text)
        self.assertIn("В4", text)

    def test_blocked_is_not_checked(self):
        self.write([{"id": "В6", "status": "skip", "detail": "нет В0"}])
        out, _ = self.run_report(["install"], q.read_items(""))
        e = [x for x in out if x["id"] == "В6"][0]
        self.assertEqual(e["verdict"], q.UNCHECKED)
        self.assertIn("нет В0", e["detail"])

    def test_aborted_block_explains_unchecked(self):
        # сторож снял блок: недошедшие пункты - "НЕ ПРОВЕРЕНО: блок прерван на <id> (<step>)"
        self.write([{"id": "В0-1", "status": "ok", "detail": "x"}])
        os.makedirs(os.path.join(self.dir, "win-qa-install", "logs"))
        with open(os.path.join(self.dir, "win-qa-install", "logs", "abort.txt"), "w", encoding="utf-8") as f:
            f.write("В2\nщелчок по файлу\n")
        out, _ = self.run_report(["install"], q.read_items(""))
        e = [x for x in out if x["id"] == "В4"][0]
        self.assertEqual(e["verdict"], q.UNCHECKED)
        self.assertIn("НЕ ПРОВЕРЕНО", e["detail"])
        self.assertIn("блок прерван на В2", e["detail"])
        self.assertIn("щелчок по файлу", e["detail"])


if __name__ == "__main__":
    unittest.main()
