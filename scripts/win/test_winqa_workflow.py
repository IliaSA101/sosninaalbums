"""win-qa.yml (спека 118): триггеры, задания и права, закреплённые действия, срок артефактов. Нужен PyYAML: вне CI без него тест пропускается, в CI (переменная CI) - провал."""

import os
import re
import unittest

try:
    import yaml
except ImportError:  # pragma: no cover
    yaml = None

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.abspath(os.path.join(HERE, "..", ".."))
WORKFLOW = os.path.join(ROOT, ".github", "workflows", "win-qa.yml")


def steps(job):
    return job.get("steps", [])


# Как makensis в test_installer: в CI PyYAML обязателен (scripts/win/requirements.txt), иначе страж молча ничего не проверял бы
@unittest.skipUnless(yaml is not None or os.environ.get("CI"), "нужен PyYAML (pip install -r scripts/win/requirements.txt)")
@unittest.skipUnless(os.path.isfile(WORKFLOW), "нет .github/workflows/win-qa.yml")
class WinQaWorkflow(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        if yaml is None:  # в CI провал даёт setUp; вне CI класс уже пропущен
            return
        with open(WORKFLOW, encoding="utf-8") as f:
            cls.text = f.read()
        cls.doc = yaml.safe_load(cls.text)
        cls.on = cls.doc.get("on", cls.doc.get(True))

    def setUp(self):
        self.assertIsNotNone(yaml, "в CI PyYAML обязателен (pip install --require-hashes -r scripts/win/requirements.txt)")

    def test_triggers_only_winqa_branches_and_dispatch(self):
        self.assertEqual(set(self.on), {"push", "workflow_dispatch"})
        self.assertEqual(self.on["push"]["branches"], ["winqa/**"])
        self.assertNotIn("pull_request", self.on)
        self.assertNotIn("pull_request_target", self.on)

    def test_dispatch_inputs(self):
        inputs = self.on["workflow_dispatch"]["inputs"]
        self.assertTrue({"blocks", "video", "dpi", "runner", "sabotage"} <= set(inputs))

    def test_minimal_permissions_no_secrets(self):
        self.assertEqual(self.doc["permissions"], {"contents": "read"})
        self.assertNotIn("secrets.", self.text)
        for job in self.doc["jobs"].values():
            self.assertNotIn("permissions", job)

    def test_jobs_and_needs(self):
        jobs = self.doc["jobs"]
        self.assertEqual(set(jobs), {"plan", "replay", "kit", "scenario", "report"})
        self.assertEqual(jobs["kit"]["needs"], "plan")
        self.assertEqual(set(jobs["scenario"]["needs"]), {"plan", "kit"})
        self.assertEqual(set(jobs["report"]["needs"]), {"plan", "kit", "scenario"})
        self.assertIn("always()", jobs["report"]["if"])

    def test_scenario_is_windows_powershell_matrix(self):
        sc = self.doc["jobs"]["scenario"]
        self.assertIn("needs.plan.outputs.runner", sc["runs-on"])
        self.assertEqual(sc["defaults"]["run"]["shell"], "powershell")
        self.assertIs(sc["strategy"]["fail-fast"], False)

    def test_actions_pinned_by_sha(self):
        for job in self.doc["jobs"].values():
            for s in steps(job):
                if "uses" in s:
                    self.assertRegex(s["uses"], r"@[0-9a-f]{40}( |$)", s["uses"])

    def test_artifacts_kept_three_days(self):
        n = 0
        for job in self.doc["jobs"].values():
            for s in steps(job):
                if s.get("uses", "").startswith("actions/upload-artifact@"):
                    n += 1
                    self.assertEqual(s["with"]["retention-days"], 3)
        self.assertGreaterEqual(n, 3)

    def test_checkout_without_credentials(self):
        for job in self.doc["jobs"].values():
            for s in steps(job):
                if s.get("uses", "").startswith("actions/checkout@"):
                    self.assertIs(s["with"]["persist-credentials"], False)

    def test_windows_run_blocks_are_ascii(self):
        # Windows PowerShell 5.1 читает временный .ps1 без BOM как ANSI: кириллица в run-блоке ломается
        for s in steps(self.doc["jobs"]["scenario"]):
            if "run" in s:
                self.assertTrue(s["run"].isascii(), s.get("name"))

    def test_scenario_step_timeout_below_job(self):
        # шаг сценария должен кончиться раньше задания, чтобы сводка и выгрузка артефактов успели (сторож 118)
        sc = self.doc["jobs"]["scenario"]
        step = [s for s in steps(sc) if "run.ps1" in s.get("run", "")]
        self.assertEqual(len(step), 1)
        self.assertIn("timeout-minutes", step[0])
        self.assertLessEqual(step[0]["timeout-minutes"], sc["timeout-minutes"] - 5)

    def test_scripts_exist(self):
        for rel in re.findall(r"scripts[/\\]win[/\\][\w./\\-]+\.(?:ps1|sh|py)", self.text):
            self.assertTrue(os.path.isfile(os.path.join(ROOT, rel.replace("\\", "/"))), rel)


class ClickModes(unittest.TestCase):
    def test_click_modes_known(self):
        # Invoke-MouseClick молча делал одиночный щелчок на 'double': допустимы только click, dbl, right
        # у $C.Act режим - третий аргумент после подписи любой вложенности, поэтому режим там - единственный строчный латинский литерал
        pat = re.compile(r"(?:Invoke-HitClick\s+\S+|Invoke-MouseClick)\s+'([^']*)'")
        act = re.compile(r"\$C\.Act\b")
        word = re.compile(r"'([a-z]+)'")
        found = 0
        for base, _, names in os.walk(HERE):
            for n in names:
                if not n.endswith((".ps1", ".psm1")):
                    continue
                with open(os.path.join(base, n), encoding="utf-8-sig") as f:
                    for no, line in enumerate(f, 1):
                        modes = [m.group(1) for m in pat.finditer(line)]
                        if act.search(line):
                            modes += word.findall(line)
                        for mode in modes:
                            found += 1
                            self.assertIn(mode, {"click", "dbl", "right"}, "%s:%d" % (n, no))
        self.assertGreater(found, 5)


class PublishExclude(unittest.TestCase):
    def test_workflow_is_published(self):
        # решение 118-8 (пересмотр): прогоны в публичном репозитории, workflow уходит в main вместе со скриптами
        path = os.path.join(ROOT, "scripts", "publish-exclude.txt")
        with open(path, encoding="utf-8") as f:
            lines = [ln.strip() for ln in f if ln.strip() and not ln.startswith("#")]
        self.assertNotIn(".github/workflows/win-qa.yml", lines)
        self.assertNotIn("scripts/win/qa/", lines)


if __name__ == "__main__":
    unittest.main()
