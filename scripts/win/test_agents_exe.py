"""Логика agents_exe.py (win-qa, 143 Т1) без Windows: клиент посредника на поддельном `--mcp`, итоги, config стенда.
Сам прогон на установленном exe - только в задании agents win-qa.yml."""

import importlib.util
import json
import os
import stat
import sys
import tempfile
import textwrap
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
SPEC = importlib.util.spec_from_file_location("agents_exe", os.path.join(HERE, "qa", "agents_exe.py"))
ax = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(ax)

FAKE = textwrap.dedent(
    """\
    #!{py}
    import json, sys
    assert sys.argv[1:] == ["--mcp"], sys.argv
    for raw in sys.stdin:
        m = json.loads(raw)
        if "id" not in m:
            continue
        print(json.dumps({{"jsonrpc": "2.0", "method": "notifications/tools/list_changed"}}), flush=True)
        if m["method"] == "initialize":
            res = {{"protocolVersion": "2025-06-18", "serverInfo": {{"name": "sosnina-albums", "version": "bridge"}}}}
        elif m["method"] == "tools/list":
            res = {{"tools": [{{"name": "program_status"}}, {{"name": "start_program"}}]}}
        elif m["method"] == "hang":
            continue
        else:
            res = {{}}
        print(json.dumps({{"jsonrpc": "2.0", "id": m["id"], "result": res}}), flush=True)
    sys.exit(0)
    """
)


@unittest.skipIf(os.name == "nt", "поддельный посредник - скрипт с shebang")
class MediatorClient(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.exe = os.path.join(self.tmp.name, "fake-mcp")
        with open(self.exe, "w", encoding="utf-8") as f:
            f.write(FAKE.format(py=sys.executable))
        os.chmod(self.exe, os.stat(self.exe).st_mode | stat.S_IXUSR)
        self.med = ax.Mediator(self.exe, dict(os.environ), os.path.join(self.tmp.name, "err.txt"), "T")
        self.addCleanup(self.med.close_files)  # kill выполняется раньше (cleanup идут с конца)
        self.addCleanup(self.med.kill)

    def test_initialize_tools_and_notification_skipped(self):
        r = self.med.initialize()
        self.assertEqual(r["result"]["serverInfo"]["name"], "sosnina-albums")
        self.assertEqual(self.med.tools(), ax.STUB_TOOLS)
        # уведомление, пришедшее раньше ответа, не принимается за ответ и остаётся в seen
        self.assertTrue(any("list_changed" in s for s in self.med.seen))

    def test_timeout_returns_none(self):
        self.assertIsNone(self.med.request("hang", {}, timeout=0.5))
        self.assertTrue(self.med.alive())

    def test_eof_exit_code_zero(self):
        self.assertTrue(self.med.alive())
        self.assertEqual(self.med.eof(), 0)
        self.assertFalse(self.med.alive())


class Pieces(unittest.TestCase):
    def test_connected_needs_tool_outside_stub(self):
        # заглушка (два инструмента) не считается подключением; иначе Т1-2 проходит и без новой программы
        self.assertFalse(ax.connected(None))
        self.assertFalse(ax.connected([]))
        self.assertFalse(ax.connected(["program_status"]))
        self.assertFalse(ax.connected(ax.STUB_TOOLS))
        self.assertFalse(ax.connected(list(reversed(ax.STUB_TOOLS))))
        self.assertTrue(ax.connected(["list_albums", "get_album"]))
        self.assertTrue(ax.connected(ax.STUB_TOOLS + ["list_albums"]))

    def test_stand_config_valid_and_agents_on(self):
        c = json.loads(ax.stand_config())
        self.assertEqual(c["agents"], {"on": True, "level": 1})
        self.assertTrue(c["update"]["off"])
        self.assertEqual(set(c["tours"].values()), {"done"})

    def test_setup_name_matches_kit(self):
        self.assertEqual(ax.setup_name("0.25.2"), "SosninaAlbums-0.25.2-windows-x64-setup.exe")
        with open(os.path.join(HERE, "qa-kit.sh"), encoding="utf-8") as f:
            kit = f.read()
        for v in (ax.V0, ax.V1, ax.V2):
            self.assertRegex(kit, r"(OLD|NEW|NEWER)=%s\b" % v.replace(".", r"\."), v)

    def test_cyrillic_output_with_cp1252_stream(self):
        # консоль раннера - cp1252: без use_utf8_streams print кириллицы падал UnicodeEncodeError
        import contextlib
        import io

        before = io.TextIOWrapper(io.BytesIO(), encoding="cp1252")
        with self.assertRaises(UnicodeEncodeError):
            print("Т1", file=before, flush=True)
        raw = io.BytesIO()
        stream = io.TextIOWrapper(raw, encoding="cp1252")
        old = sys.stdout, sys.stderr
        sys.stdout = sys.stderr = stream
        try:
            ax.use_utf8_streams()
            with contextlib.redirect_stdout(stream):
                ax.Report().add("Т1-1 установка", True, "подробности: кириллица")
            stream.flush()
        finally:
            sys.stdout, sys.stderr = old
        self.assertIn("Т1-1 установка".encode("utf-8"), raw.getvalue())
        stream.detach()

    def test_password_fits_net_user(self):
        # длиннее 14 знаков net user спрашивает Y/N и без ввода выходит с -1 (CI winqa/143-t1b)
        for _ in range(50):
            pw = ax.make_password()
            self.assertLessEqual(len(pw), 14)
            self.assertTrue(any(c.isupper() for c in pw) and any(c.islower() for c in pw)
                            and any(c.isdigit() for c in pw) and any(not c.isalnum() for c in pw), pw)

    def test_create_user_reports_output_and_falls_back(self):
        import subprocess
        from unittest import mock

        def fake(cmd, **kw):
            self.assertIs(kw.get("stdin"), subprocess.DEVNULL)  # вопрос Y/N не должен ждать ввода
            if cmd[0] == "net":
                return subprocess.CompletedProcess(cmd, 4294967295, b"", b"password too long")
            return subprocess.CompletedProcess(cmd, 1, b"", b"denied")

        with mock.patch.object(ax.subprocess, "run", fake):
            ok, why = ax.create_user("u", "p")
        self.assertFalse(ok)
        self.assertIn("password too long", why)
        self.assertIn("4294967295", why)
        self.assertIn("New-LocalUser", why)

        def net_ok(cmd, **kw):
            return subprocess.CompletedProcess(cmd, 0, b"", b"")

        with mock.patch.object(ax.subprocess, "run", net_ok):
            self.assertEqual(ax.create_user("u", "p"), (True, ""))


class SwapCheck(unittest.TestCase):
    """Замена exe без опоры на содержимое и время создания нового файла: тот же exe в установщиках 0.25.1 и 0.25.2,
    а NTFS переносит время создания старого файла на новый под тем же именем."""

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.exe = os.path.join(self.tmp.name, "SosninaAlbums.exe")
        self.put(self.exe, b"B")  # exe - образ B
        self.put(self.exe + ".old", b"A")  # .old - образ A
        self.sha_b = ax.hashlib.sha256(b"B").hexdigest()
        self.sha_a = ax.hashlib.sha256(b"A").hexdigest()
        self.ids = ax.file_ids(self.exe)

    def put(self, path, data):
        with open(path, "wb") as f:
            f.write(data)

    def swap(self, new=b"B", tail=True, old=True):
        """Обмен установщика: .old -> .old-1, exe -> .old, новый exe (то же содержимое, что у прежнего - как в наборе)."""
        if tail:
            os.rename(self.exe + ".old", self.exe + ".old-1")
        if old:
            os.rename(self.exe, self.exe + ".old")
        self.put(self.exe, new)

    def check(self):
        return ax.swap_check(self.exe, self.sha_b, self.sha_a, *self.ids)

    def test_swapped_same_content(self):
        self.swap()  # новый exe байт-в-байт как прежний, но другой файл
        ok, why = self.check()
        self.assertTrue(ok, why)

    def test_not_replaced_is_caught(self):
        ok, why = self.check()  # обмена не было
        self.assertFalse(ok)
        self.assertIn(".old-*", why)
        self.assertIn("file index не изменился", why)

    def test_exe_rewritten_in_place_is_caught(self):
        # содержимое перезаписано на месте (тот же file index), .old-* нет
        self.put(self.exe, b"N")
        ok, why = self.check()
        self.assertFalse(ok)
        self.assertIn("file index не изменился", why)

    def test_wrong_old_is_caught(self):
        os.rename(self.exe + ".old", self.exe + ".old-1")
        self.put(self.exe + ".old", b"X")  # чужой .old: ни хеш, ни file index не прежние
        self.put(self.exe + ".new", b"N")
        os.replace(self.exe + ".new", self.exe)
        ok, why = self.check()
        self.assertFalse(ok)
        self.assertIn(".old не прежний exe", why)

    def test_old_tail_other_file_is_caught(self):
        # .old-* с нужным хешем, но скопирован, а не переименован: file index другой
        self.swap(tail=False)
        self.put(self.exe + ".old-1", b"A")
        ok, why = self.check()
        self.assertFalse(ok)


class ReportRows(unittest.TestCase):
    def test_report(self):
        r = ax.Report()
        r.add("a", True, "x|y")
        r.add("b", False, "bad")
        r.add("c", False, "skipped", skip=True)
        self.assertEqual([x[0] for x in r.failed()], ["b"])
        md = r.markdown()
        self.assertIn("| a | ok | x/y |", md)
        self.assertIn("| c | skip |", md)


if __name__ == "__main__":
    unittest.main()
