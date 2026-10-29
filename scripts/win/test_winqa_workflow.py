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
        self.assertEqual(set(jobs), {"plan", "replay", "kit", "scenario", "agents", "library", "videos-site", "videos-win", "videos-mac", "report"})
        self.assertEqual(jobs["kit"]["needs"], "plan")
        self.assertEqual(set(jobs["scenario"]["needs"]), {"plan", "kit"})
        self.assertEqual(set(jobs["report"]["needs"]), {"plan", "kit", "scenario"})
        self.assertEqual(set(jobs["agents"]["needs"]), {"plan", "kit"})
        self.assertEqual(set(jobs["library"]["needs"]), {"plan", "kit"})
        self.assertEqual(jobs["videos-site"]["needs"], "plan")
        self.assertEqual(set(jobs["videos-win"]["needs"]), {"plan", "kit", "videos-site"})
        self.assertEqual(set(jobs["videos-mac"]["needs"]), {"plan", "videos-site"})
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

    def test_agents_job_windows_powershell_ascii_and_artifact(self):
        # агенты-exe (143 Т1): тот же раннер и PowerShell 5.1; артефакт не под win-qa-*, иначе qa_report.py примет его за блок
        ag = self.doc["jobs"]["agents"]
        self.assertIn("needs.plan.outputs.runner", ag["runs-on"])
        self.assertEqual(ag["defaults"]["run"]["shell"], "powershell")
        for s in steps(ag):
            if "run" in s:
                self.assertTrue(s["run"].isascii(), s.get("name"))
        runs = " ".join(s.get("run", "") for s in steps(ag))
        self.assertIn("agents_exe.py", runs)
        names = [s["with"]["name"] for s in steps(ag) if s.get("uses", "").startswith("actions/upload-artifact@")]
        self.assertEqual(names, ["agents-exe-qa"])

    def test_library_job_windows_powershell_ascii_and_artifact(self):
        # образцы наборов в exe (144): тот же раннер и PowerShell 5.1; артефакт не под win-qa-*; шаг короче задания
        lib = self.doc["jobs"]["library"]
        self.assertIn("needs.plan.outputs.runner", lib["runs-on"])
        self.assertEqual(lib["defaults"]["run"]["shell"], "powershell")
        for s in steps(lib):
            if "run" in s:
                self.assertTrue(s["run"].isascii(), s.get("name"))
        run = [s for s in steps(lib) if "library_exe.py" in s.get("run", "")]
        self.assertEqual(len(run), 1)
        self.assertLessEqual(run[0]["timeout-minutes"], lib["timeout-minutes"] - 5)
        names = [s["with"]["name"] for s in steps(lib) if s.get("uses", "").startswith("actions/upload-artifact@")]
        self.assertEqual(names, ["library-exe-qa"])

    def test_videos_jobs_win_ascii_mac_builds_qa_and_guards(self):
        # ролики F1 (140 Т7): Windows - как library; macOS - своя сборка с тегом qa и обратный страж; артефакты не под win-qa-*
        win = self.doc["jobs"]["videos-win"]
        self.assertIn("needs.plan.outputs.runner", win["runs-on"])
        self.assertEqual(win["defaults"]["run"]["shell"], "powershell")
        for s in steps(win):
            if "run" in s:
                self.assertTrue(s["run"].isascii(), s.get("name"))
        run = [s for s in steps(win) if "videos_exe.py" in s.get("run", "")]
        self.assertEqual(len(run), 1)
        self.assertLessEqual(run[0]["timeout-minutes"], win["timeout-minutes"] - 5)
        mac = self.doc["jobs"]["videos-mac"]
        self.assertEqual(mac["runs-on"], "macos-latest")
        runs = [s.get("run", "") for s in steps(mac)]
        builds = [r for r in runs if "wails build" in r]
        self.assertEqual(len(builds), 1)
        self.assertRegex(builds[0], r"-tags qa\b")
        guard = [r for r in runs if "SA_QA_PROBE SA_LIBRARY_URL" in r]
        self.assertEqual(len(guard), 1)
        self.assertIn("|| {", guard[0])  # отсутствие строки в бинарнике - провал, обратно релизному стражу
        mrun = [s for s in steps(mac) if "videos_exe.py" in s.get("run", "")]
        self.assertEqual(len(mrun), 1)
        self.assertLessEqual(mrun[0]["timeout-minutes"], mac["timeout-minutes"] - 5)
        names = [s["with"]["name"] for j in (win, mac) for s in steps(j) if s.get("uses", "").startswith("actions/upload-artifact@")]
        self.assertEqual(names, ["videos-win-exe-qa", "videos-mac-app-qa"])
        site = self.doc["jobs"]["videos-site"]
        self.assertIn("library-testvideos.mjs", " ".join(s.get("run", "") for s in steps(site)))

    def test_replay_peercred_steps(self):
        # 148: чужая учётная запись на Windows - создание пользователя, два разбора, удаление последним и всегда; run-блоки ASCII;
        # пакет peercred входит в шаг -v; пароль скрыт add-mask и не печатается
        rp = steps(self.doc["jobs"]["replay"])
        names = [s.get("name", "") for s in rp]
        idx = [i for i, n in enumerate(names) if n.startswith("peercred")]
        self.assertEqual(len(idx), 4, names)
        self.assertEqual(idx, list(range(idx[0], idx[0] + 4)))
        create, foreign, elevated, cleanup = (rp[i] for i in idx)
        self.assertEqual(idx[-1], len(rp) - 1)  # удаление - последним
        for s in (create, foreign, elevated, cleanup):
            self.assertEqual(s["if"], "always()", s["name"])
            self.assertTrue(s["run"].isascii(), s["name"])
        self.assertIn("::add-mask::$pw", create["run"])
        self.assertIn("SA_PEER_FOREIGN=cred", create["run"])
        self.assertLessEqual(len("Aa1!") + 10, 14)  # пароль до 14 знаков: длиннее net user спрашивает Y/N
        self.assertRegex(create["run"], r"GetBytes\(\$b\)")
        self.assertRegex(foreign["run"], r"peer-foreign\.mjs TestForeign TestForeignClient TestForeignServer$")
        self.assertNotIn("--allow-skip", foreign["run"])  # пропуск - провал
        # Г-в подтверждена (29.10, 148-11): пропуск TestElevatedServerWindows - тоже провал, PASS обязателен
        self.assertNotIn("--allow-skip", elevated["run"])
        for name in ("TestPeerPIDQuery", "TestElevatedServerWindows", "TestListenerHandoffWindows", "TestSelfProbeReal"):
            self.assertRegex(elevated["run"], rf"'[^']*\b{name}\b[^']*' .*\b{name}\b", name)  # в шаблоне -run и в обязательных
        self.assertIn("Remove-LocalUser", cleanup["run"])
        self.assertIn("Win32_UserProfile", cleanup["run"])
        verbose = [s for s in rp if "-v" in s.get("run", "") and "./internal/peercred" in s["run"]]
        self.assertEqual(len(verbose), 1)
        self.assertTrue(any(s.get("uses", "").startswith("actions/setup-node@") for s in rp))  # node для peer-foreign.mjs

    def test_replay_mcp_dir_steps(self):
        # 148-11: подтесты с точкой соединения (mklink /J) в CI не пропускаются; разбор - peer-foreign.mjs по своему пакету
        rp = steps(self.doc["jobs"]["replay"])
        found = {s["name"]: s for s in rp if s.get("name", "").startswith("mcp - ")}
        self.assertEqual(len(found), 2, list(found))
        for s in found.values():
            self.assertEqual(s["if"], "always()", s["name"])
            self.assertTrue(s["run"].isascii(), s["name"])
            self.assertNotIn("--allow-skip", s["run"], s["name"])
            self.assertIn("peer-foreign.mjs", s["run"], s["name"])
        runs = " ".join(s["run"] for s in found.values())
        self.assertIn("--pkg=./internal/app/agents", runs)
        self.assertIn("--pkg=./internal/mcpbridge", runs)
        for name in ("TestSocketDACL", "TestSocketDACLHandle", "TestSockDirHandleWindows", "TestSockDirHeldWindows", "TestBridgeHoldsDirDuringPeerWindows", "TestSockDirWindows"):
            self.assertIn(name, runs)
        names = [s.get("name", "") for s in rp]
        self.assertLess(max(names.index(n) for n in found), names.index("peercred - создать пользователя qapeer"))
        self.assertTrue(any(s.get("uses", "").startswith("actions/setup-node@") for s in rp[: names.index(list(found)[0])]))  # node до разбора

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


CI = os.path.join(ROOT, ".github", "workflows", "ci.yml")


@unittest.skipUnless(yaml is not None or os.environ.get("CI"), "нужен PyYAML (pip install -r scripts/win/requirements.txt)")
@unittest.skipUnless(os.path.isfile(CI), "нет .github/workflows/ci.yml")
class CiPeercred(unittest.TestCase):
    """148: чужая учётная запись на Linux и macOS - отдельный шаг с SA_PEER_FOREIGN=sudo и разбором peer-foreign.mjs."""

    def setUp(self):
        self.assertIsNotNone(yaml, "в CI PyYAML обязателен")
        with open(CI, encoding="utf-8") as f:
            self.jobs = yaml.safe_load(f)["jobs"]

    def test_linux_and_mac_foreign_steps(self):
        for job in ("go-linux", "go-mac"):
            found = [s for s in steps(self.jobs[job]) if "peer-foreign.mjs" in s.get("run", "")]
            self.assertEqual(len(found), 1, job)
            self.assertEqual(found[0]["env"]["SA_PEER_FOREIGN"], "sudo", job)
            self.assertIn("TestForeignClient TestForeignServer", found[0]["run"], job)
            self.assertNotIn("--allow-skip", found[0]["run"], job)
            self.assertNotIn("continue-on-error", found[0], job)
        mac_v = [s for s in steps(self.jobs["go-mac"]) if "-v" in s.get("run", "") and "./internal/peercred" in s["run"]]
        self.assertEqual(len(mac_v), 1)  # peercred в блокирующем шаге macOS
        self.assertNotIn("continue-on-error", mac_v[0])

    def test_linux_step_after_go_test(self):
        names = [s.get("name", "") for s in steps(self.jobs["go-linux"])]
        self.assertLess(names.index("go test"), [i for i, n in enumerate(names) if n.startswith("peercred")][0])


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


RELEASE = os.path.join(ROOT, ".github", "workflows", "release.yml")


@unittest.skipUnless(os.path.isfile(RELEASE), "нет .github/workflows/release.yml")
class ReleaseNoQaTags(unittest.TestCase):
    """Выпуск собирается без тегов qa и dev (144, 0.31-144-59): зонд win-qa и стенд библиотеки в бинарник не попадают."""

    def setUp(self):
        with open(RELEASE, encoding="utf-8") as f:
            self.text = f.read()

    def test_builds_have_no_qa_or_dev_tags(self):
        builds = [ln for ln in self.text.splitlines() if re.search(r"\b(wails build|build\.sh|build\.ps1)\b", ln) and "run:" in ln]
        self.assertGreaterEqual(len(builds), 3)  # windows, macos, linux
        for ln in builds:
            for m in re.finditer(r"-{1,2}tags[=\s]+[\"']?([\w,+ ]+)", ln):
                tags = re.split(r"[,\s+]+", m.group(1).strip())
                self.assertFalse({"qa", "dev"} & set(tags), ln)
            self.assertNotRegex(ln, r"--tags")
        for bad in ("wails build -tags=qa", "wails build -tags qa,webkit2_41", 'wails build -tags "dev"', "wails build -tags=webkit2_41,qa"):
            m = re.search(r"-{1,2}tags[=\s]+[\"']?([\w,+ ]+)", bad)
            self.assertTrue({"qa", "dev"} & set(re.split(r"[,\s+]+", m.group(1).strip())), bad)  # сам разбор ловит такие формы
        self.assertTrue(all(g in self.text for g in ("нет бинарника build/bin/sosninaalbums.exe", "нет бинарника build/bin/sosninaalbums\"")))

    def test_every_build_is_followed_by_tag_guard(self):
        # после каждой сборки программы - шаг, ищущий SA_QA_PROBE и SA_LIBRARY_URL в бинарнике
        self.assertEqual(self.text.count("Страж тегов QA"), self.text.count("wails build -"))
        self.assertEqual(self.text.count("SA_QA_PROBE SA_LIBRARY_URL"), self.text.count("Страж тегов QA"))


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
