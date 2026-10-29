#!/usr/bin/env python3
"""Агенты-exe в win-qa (спека 0.31/143, Т1; design §4.7, Г8): настоящий установленный exe на Windows-раннере.

Цепочка одной ВМ: setup 0.25.0 /S -> посредник A (`SosninaAlbums.exe --mcp`, stdin держим) -> setup 0.25.1 /S /relaunch
-> посредник B -> setup 0.25.2 /S /relaunch (.old A занят: уходит в .old-<тики>) -> EOF обоим -> тихое удаление при
живом посреднике C. Между обновлениями: чужой пользователь Windows не подключается к сокету запущенной программы.
Установщики 0.25.x собраны qa-kit.sh из кода той же ветки (--mcp в них есть), SA_STAND=qa держит данные в отдельной папке.

Один прогон собирает все провалы: пункт упал - остальные идут, где их входы целы; в конце дамп состояния и код 1.
Запуск: python agents_exe.py --kit <набор qa-kit> --out <папка итогов>. Только Windows; stdlib.
Файл в UTF-8, ASCII не требуется (запускается python, не PowerShell).
"""

import argparse
import hashlib
import json
import os
import queue
import secrets
import subprocess
import sys
import threading
import time

STAND = "qa"
DATA = "SosninaAlbums-stand-" + STAND  # папка данных стенда (osx: DataName + "-stand-" + имя)
V0, V1, V2 = "0.25.0", "0.25.1", "0.25.2"
EXE_NAME = "SosninaAlbums.exe"
REQ_TIMEOUT = 20.0


def use_utf8_streams():
    """Консоль раннера в cp1252, а `python -I` игнорирует PYTHONUTF8: кириллица в print иначе роняет прогон (как в pack.py)."""
    for stream in (sys.stdout, sys.stderr):
        if hasattr(stream, "reconfigure"):
            stream.reconfigure(encoding="utf-8", errors="replace")


def setup_name(ver):
    return "SosninaAlbums-%s-windows-x64-setup.exe" % ver


class Mediator:
    """Живой посредник `exe --mcp`: stdin открыт, строки stdout - в очередь, stderr - в файл (для дампа)."""

    def __init__(self, exe, env, errfile, tag):
        self.tag = tag
        self.exe = exe
        self.errfile = errfile
        self._err = open(errfile, "wb")
        self.proc = subprocess.Popen([exe, "--mcp"], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                     stderr=self._err, env=env, cwd=os.path.dirname(exe))
        self.lines = queue.Queue()
        self.seen = []  # все прочитанные строки (для дампа)
        self._next = 0
        threading.Thread(target=self._pump, daemon=True).start()

    def _pump(self):
        for raw in self.proc.stdout:
            self.lines.put(raw)
        self.lines.put(None)

    def request(self, method, params=None, timeout=REQ_TIMEOUT):
        """Отправить запрос и дождаться ответа с тем же id; уведомления по пути складываются в seen. None - не дождались."""
        self._next += 1
        rid = "%s-%d" % (self.tag, self._next)
        msg = {"jsonrpc": "2.0", "id": rid, "method": method}
        if params is not None:
            msg["params"] = params
        self.send(msg)
        end = time.monotonic() + timeout
        while True:
            left = end - time.monotonic()
            if left <= 0:
                return None
            try:
                raw = self.lines.get(timeout=left)
            except queue.Empty:
                return None
            if raw is None:
                return None
            self.seen.append(raw.decode("utf-8", "replace").rstrip())
            try:
                obj = json.loads(raw)
            except ValueError:
                continue
            if isinstance(obj, dict) and obj.get("id") == rid:
                return obj

    def send(self, obj):
        self.proc.stdin.write((json.dumps(obj) + "\n").encode("utf-8"))
        self.proc.stdin.flush()

    def initialize(self):
        r = self.request("initialize", {"protocolVersion": "2025-06-18", "capabilities": {},
                                        "clientInfo": {"name": "sa-win-qa", "version": "1"}})
        if r is not None and "result" in r:
            self.send({"jsonrpc": "2.0", "method": "notifications/initialized"})
        return r

    def tools(self, timeout=REQ_TIMEOUT):
        r = self.request("tools/list", {}, timeout)
        if not r or "result" not in r:
            return None
        return [t.get("name") for t in r["result"].get("tools", [])]

    def alive(self):
        return self.proc.poll() is None

    def eof(self, timeout=15.0):
        """Закрыть stdin; код выхода или None, если не вышел за срок (процесс тогда снимается по PID)."""
        try:
            self.proc.stdin.close()
        except OSError:
            pass
        try:
            return self.proc.wait(timeout)
        except subprocess.TimeoutExpired:
            self.kill()
            return None

    def kill(self):
        if self.proc.poll() is None:
            if os.name == "nt":
                subprocess.run(["taskkill", "/PID", str(self.proc.pid), "/T", "/F"], capture_output=True)
            else:  # тесты на Linux
                self.proc.kill()

    def close_files(self):
        """Закрыть трубы и файл stderr (после kill/EOF); процесс дожидается, чтобы не оставить зомби."""
        for f in (self.proc.stdin, self.proc.stdout, self._err):
            try:
                f.close()
            except OSError:
                pass
        try:
            self.proc.wait(10)
        except subprocess.TimeoutExpired:
            pass


class Report:
    def __init__(self):
        self.rows = []

    def add(self, rid, ok, detail, skip=False):
        st = "skip" if skip else ("ok" if ok else "FAIL")
        self.rows.append((rid, st, detail))
        print("[%s] %s: %s" % (st, rid, detail), flush=True)

    def failed(self):
        return [r for r in self.rows if r[1] == "FAIL"]

    def markdown(self):
        out = ["## win-qa: агенты-exe (143 Т1)", "", "| пункт | итог | подробности |", "|---|---|---|"]
        for rid, st, d in self.rows:
            out.append("| %s | %s | %s |" % (rid, st, d.replace("|", "/").replace("\n", " ")))
        return "\n".join(out) + "\n"


def ps(script, timeout=90):
    """PowerShell-однострочник; (код, stdout+stderr)."""
    r = subprocess.run(["powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script],
                       capture_output=True, timeout=timeout)
    return r.returncode, (r.stdout + r.stderr).decode("utf-8", "replace")


def run_setup(exe, args, env, timeout=180):
    """Код выхода установщика; -1 - не уложился (снят по PID). Без ожидания потомков: /relaunch оставляет программу живой."""
    p = subprocess.Popen([exe] + args, env=env)
    try:
        return p.wait(timeout)
    except subprocess.TimeoutExpired:
        subprocess.run(["taskkill", "/PID", str(p.pid), "/T", "/F"], capture_output=True)
        return -1


def reg_version():
    import winreg
    try:
        with winreg.OpenKey(winreg.HKEY_CURRENT_USER, r"Software\Microsoft\Windows\CurrentVersion\Uninstall\SosninaAlbums") as k:
            return winreg.QueryValueEx(k, "DisplayVersion")[0]
    except OSError:
        return None


def processes(instexe):
    """Процессы exe из установленной папки: [(pid, cmdline)]. Через WMI: Path/CommandLine нужны, по имени не убиваем."""
    code, out = ps("Get-CimInstance Win32_Process -Filter \"Name='%s'\" | ForEach-Object { '{0}|{1}|{2}' -f $_.ProcessId, $_.ExecutablePath, $_.CommandLine }" % EXE_NAME)
    res = []
    for ln in out.splitlines():
        parts = ln.strip().split("|", 2)
        if len(parts) == 3 and parts[1].lower() == instexe.lower():
            res.append((int(parts[0]), parts[2]))
    return res


def gui_pids(instexe):
    return [pid for pid, cmd in processes(instexe) if "--mcp" not in cmd]


def wait_for(cond, timeout, step=0.5):
    end = time.monotonic() + timeout
    while time.monotonic() < end:
        v = cond()
        if v:
            return v
        time.sleep(step)
    return cond()


def stop_gui(instexe):
    for pid in gui_pids(instexe):
        subprocess.run(["taskkill", "/PID", str(pid), "/T", "/F"], capture_output=True)
    wait_for(lambda: not gui_pids(instexe), 15)


def sha(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def listing(d):
    try:
        return ["%s (%d)" % (n, os.path.getsize(os.path.join(d, n))) for n in sorted(os.listdir(d))]
    except OSError as e:
        return ["<нет: %s>" % e]


def file_ids(exe):
    """Идентификаторы файлов (на Windows st_ino - file index NTFS) exe и exe.old до обновления: переименование их сохраняет."""
    old = exe + ".old"
    return os.stat(exe).st_ino, (os.stat(old).st_ino if os.path.isfile(old) else None)


def swap_check(exe, prev_exe_sha, prev_old_sha, prev_exe_id, prev_old_id):
    """Замена exe не по содержимому нового файла (установщики 0.25.1 и 0.25.2 несут один и тот же exe), а по следам обмена:
    прежний exe лёг в .old, прежний .old (образ живого посредника) ушёл в .old-<тики> - и по хешу, и по file index;
    новый exe - другой файл (file index не прежний). Время создания не годится: NTFS отдаёт новому файлу время старого
    под тем же именем (туннелирование имён, ~15 с). (ok, объяснение)."""
    d = os.path.dirname(exe)
    old = exe + ".old"
    if not os.path.isfile(exe):
        return False, "exe нет"
    why = []
    if not os.path.isfile(old):
        why.append(".old нет")
    else:
        if sha(old) != prev_exe_sha:
            why.append(".old не прежний exe (хеш)")
        if os.stat(old).st_ino != prev_exe_id:
            why.append(".old не тот файл, что был exe (file index)")
    tails = [os.path.join(d, n) for n in os.listdir(d) if n.startswith(os.path.basename(exe) + ".old-")]
    if not any(sha(t) == prev_old_sha and os.stat(t).st_ino == prev_old_id for t in tails):
        why.append("в .old-* нет прежнего .old (хеш и file index)")
    if os.stat(exe).st_ino == prev_exe_id:
        why.append("exe тот же файл (file index не изменился)")
    return not why, "; ".join(why) or "обмен .old -> .old-*, exe -> .old, новый файл exe"


def make_password():
    """Пароль локального пользователя: до 14 знаков (длиннее - `net user` задаёт вопрос Y/N и без ввода падает с -1),
    но со всеми четырьмя классами знаков, чтобы пройти политику сложности."""
    return "Aa1!" + secrets.token_hex(5)


def create_user(user, pw):
    """Создать локального пользователя; (ok, текст ошибки). Сначала net user без ввода, затем New-LocalUser."""
    r = subprocess.run(["net", "user", user, pw, "/add"], capture_output=True, stdin=subprocess.DEVNULL, timeout=60)
    if r.returncode == 0:
        return True, ""
    first = "net user код %d: %s" % (r.returncode, " ".join((r.stdout + r.stderr).decode("mbcs" if os.name == "nt" else "utf-8", "replace").split()))
    script = ("New-LocalUser -Name $env:QA_USER -Password (ConvertTo-SecureString $env:QA_PW -AsPlainText -Force) "
              "-AccountNeverExpires -PasswordNeverExpires | Out-Null")
    r = subprocess.run(["powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script],
                       capture_output=True, stdin=subprocess.DEVNULL, timeout=60, env=dict(os.environ, QA_USER=user, QA_PW=pw))
    if r.returncode == 0:
        return True, ""
    return False, first + "; New-LocalUser код %d: %s" % (r.returncode, " ".join((r.stdout + r.stderr).decode("utf-8", "replace").split()))


def stand_config():
    """config.json стенда как New-QaStandConfig (winqa-update.psm1) плюс служба агентов включена."""
    tours = ",".join('"%s":"done"' % t for t in ("first", "folders", "library", "autobuild", "timeline", "layouts", "photo", "export"))
    return '{"level":"simple","hideStart":true,"assoc":"off","tours":{%s},"update":{"off":true},"agents":{"on":true,"level":1}}' % tours


def write_text(path, text):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8", newline="") as f:
        f.write(text)


def foreign_probe(sock, work, rep):
    """Пункт С2 (б): пользователь Windows без прав на папку mcp не подключается к сокету программы.
    Контроль: тем же скриптом свой пользователь подключается (иначе проба сама негодная)."""
    probe_src = os.path.join(os.path.dirname(os.path.abspath(__file__)), "agents_probe_sock.py")
    pub = r"C:\Users\Public\sa-qa-probe"  # читаемая всем папка: чужой пользователь не видит ни рабочую папку раннера, ни профиль
    os.makedirs(pub, exist_ok=True)
    subprocess.run(["icacls", pub, "/grant", "*S-1-1-0:(OI)(CI)M"], capture_output=True)
    probe = os.path.join(pub, "agents_probe_sock.py")
    with open(probe_src, "rb") as a, open(probe, "wb") as b:
        b.write(a.read())

    def run_own():
        r = subprocess.run([sys.executable, "-I", probe, sock], capture_output=True, timeout=60)
        return r.stdout.decode("utf-8", "replace").strip()

    own = run_own()
    try:
        own_ok = json.loads(own).get("ok") is True
    except ValueError:
        own_ok = False
    rep.add("Т1-5a контроль: свой пользователь подключается к сокету", own_ok, "проба: %s" % own)

    # DACL папки mcp: ни Users, ни Everyone, ни Authenticated Users (TestSocketDACL проверяет то же в Go)
    mcp = os.path.dirname(sock)
    r = subprocess.run(["icacls", mcp], capture_output=True)
    acl = r.stdout.decode("utf-8", "replace")
    bad = [w for w in ("Everyone", "BUILTIN\\Users", "Authenticated Users", "BUILTIN\\Administrators") if w.lower() in acl.lower()]
    rep.add("Т1-5b DACL папки mcp без чужих записей", r.returncode == 0 and not bad, "icacls: %s" % " ".join(acl.split()))

    user, pw = "qaother", make_password()
    made, why = create_user(user, pw)
    if not made:
        rep.add("Т1-5c чужой пользователь не подключается", False, "не удалось создать пользователя: %s" % why, skip=True)
        return
    try:
        out_file = os.path.join(pub, "foreign.json")
        if os.path.exists(out_file):
            os.remove(out_file)
        script = ("$pw = ConvertTo-SecureString $env:QA_PW -AsPlainText -Force; "
                  "$c = New-Object System.Management.Automation.PSCredential('.\\%s', $pw); "
                  "$p = Start-Process -FilePath $env:QA_PY -ArgumentList @('-I', $env:QA_PROBE, $env:QA_SOCK, $env:QA_OUT) "
                  "-Credential $c -LoadUserProfile -WorkingDirectory $env:SystemRoot -WindowStyle Hidden -Wait -PassThru; "
                  "exit $p.ExitCode" % user)
        env = dict(os.environ, QA_PW=pw, QA_PY=sys.executable, QA_PROBE=probe, QA_SOCK=sock, QA_OUT=out_file)
        r = subprocess.run(["powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script],
                           capture_output=True, env=env, timeout=120)
        text = (r.stdout + r.stderr).decode("utf-8", "replace").strip()
        res = None
        if os.path.exists(out_file):
            try:
                with open(out_file, encoding="utf-8") as f:
                    res = json.load(f)
            except ValueError:
                res = None
        if res is None:
            rep.add("Т1-5c чужой пользователь не подключается", False,
                    "проба под чужим пользователем не отработала (код %d): %s" % (r.returncode, " ".join(text.split())[:300]), skip=True)
        else:
            rep.add("Т1-5c чужой пользователь не подключается", res.get("ok") is False,
                    "результат пробы: %s (ошибка 10013/5 - отказ в доступе; сам профиль раннера чужому тоже закрыт, DACL папки - Т1-5b и TestSocketDACL)" % json.dumps(res))
    finally:
        subprocess.run(["net", "user", user, "/delete"], capture_output=True, stdin=subprocess.DEVNULL)


def main():
    use_utf8_streams()
    ap = argparse.ArgumentParser()
    ap.add_argument("--kit", required=True)
    ap.add_argument("--out", required=True)
    a = ap.parse_args()
    if os.name != "nt":
        print("только Windows", file=sys.stderr)
        return 2
    if not a.out:
        print("--out пуст", file=sys.stderr)
        return 2
    out = os.path.abspath(a.out)
    os.makedirs(out, exist_ok=True)
    rep = Report()
    local = os.environ.get("LOCALAPPDATA", "")
    roaming = os.environ.get("APPDATA", "")
    if not local or not roaming:
        print("нет LOCALAPPDATA/APPDATA", file=sys.stderr)
        return 2
    inst = os.path.join(local, "Programs", "Sosnina Albums")
    exe = os.path.join(inst, EXE_NAME)
    data = os.path.join(roaming, DATA)
    sock = os.path.join(data, "mcp", "sock")
    env = dict(os.environ, SA_STAND=STAND)
    setups = {v: os.path.join(a.kit, "setup", setup_name(v)) for v in (V0, V1, V2)}
    meds = {}
    try:
        for v, p in setups.items():
            if not os.path.isfile(p):
                rep.add("подготовка", False, "нет установщика в наборе: %s" % p)
                return finish(rep, out, exe, inst, data, meds)

        # ---- 1. установка и посредник A при закрытой программе ----
        shutil_rmtree(data)
        write_text(os.path.join(data, "config.json"), stand_config())
        code = run_setup(setups[V0], ["/S"], env)
        ok = code == 0 and os.path.isfile(exe) and reg_version() == V0
        rep.add("Т1-1 установка 0.25.0 /S", ok, "код %s; DisplayVersion %s; exe есть: %s" % (code, reg_version(), os.path.isfile(exe)))
        if not ok:
            return finish(rep, out, exe, inst, data, meds)
        sha0 = sha(exe)

        meds["A"] = Mediator(exe, env, os.path.join(out, "mediator-A.stderr.txt"), "A")
        r = meds["A"].initialize()
        names = meds["A"].tools() if r and "result" in r else None
        info = (r or {}).get("result", {}).get("serverInfo", {})
        rep.add("Т1-1 посредник A: initialize и заглушка при закрытой программе",
                bool(r) and "result" in r and info.get("name") == "sosnina-albums" and names == ["program_status"],
                "serverInfo %s; инструменты %s; stdout: %s" % (info, names, meds["A"].seen[-2:]))

        # ---- 2. обновление 0.25.1 при живом A ----
        code = run_setup(setups[V1], ["/S", "/relaunch"], env)
        old = os.path.join(inst, EXE_NAME + ".old")
        ok = code == 0 and reg_version() == V1 and sha(exe) != sha0
        rep.add("Т1-2 обновление 0.25.1 /S /relaunch при живом посреднике", ok,
                "код %s (0 - успех, 5 - ждал программу, 6 - замена exe); DisplayVersion %s; exe новый: %s; файлы: %s" %
                (code, reg_version(), os.path.isfile(exe) and sha(exe) != sha0, listing(inst)))
        rep.add("Т1-2 образ A лежит в .old, A жив", meds["A"].alive() and os.path.isfile(old), "A жив: %s; .old есть: %s" % (meds["A"].alive(), os.path.isfile(old)))
        gui = wait_for(lambda: gui_pids(exe), 30)
        rep.add("Т1-2 /relaunch открыл программу", bool(gui), "PID программы: %s" % gui)
        sock_ok = wait_for(lambda: os.path.isdir(os.path.dirname(sock)) and "sock" in os.listdir(os.path.dirname(sock)), 30)
        rep.add("Т1-2 программа подняла сокет агентов", bool(sock_ok), "папка mcp: %s" % listing(os.path.dirname(sock)))
        # A подхватывает новую программу сам (повтор initialize, §4.2): инструментов больше одного
        def a_full():
            n = meds["A"].tools(8)
            return n if n and len(n) > 1 else None
        full = wait_for(a_full, 45, 1) if meds["A"].alive() and sock_ok else None
        rep.add("Т1-2 старый A после обновления подключился к новой программе", bool(full), "инструменты: %s" % (full,))

        # ---- 5. чужой пользователь (программа открыта, сокет есть) ----
        if sock_ok:
            foreign_probe(sock, out, rep)
        else:
            rep.add("Т1-5 чужой пользователь не подключается", False, "нет сокета: программа не подняла службу агентов", skip=True)

        # ---- 3. второе обновление при двух живых посредниках (A держит .old) ----
        meds["B"] = Mediator(exe, env, os.path.join(out, "mediator-B.stderr.txt"), "B")
        r = meds["B"].initialize()
        rep.add("Т1-3 посредник B из новой копии при открытой программе", bool(r) and "result" in r, "ответ: %s" % (meds["B"].seen[-1:],))
        stop_gui(exe)  # тихая установка ждёт закрытия программы до 60 с: закрываем сами
        sha1 = sha(exe)
        sha_old = sha(exe + ".old") if os.path.isfile(exe + ".old") else None
        id_exe, id_old = file_ids(exe)
        code = run_setup(setups[V2], ["/S", "/relaunch"], env)
        tails = [n for n in os.listdir(inst) if n.startswith(EXE_NAME + ".old-")]
        swapped, swap_why = swap_check(exe, sha1, sha_old, id_exe, id_old)
        ok = code == 0 and reg_version() == V2 and swapped and bool(tails)
        rep.add("Т1-3 второе обновление 0.25.2 при живых A и B", ok,
                "код %s; DisplayVersion %s; хвосты .old-*: %s; обмен файлов: %s; файлы: %s" % (code, reg_version(), tails, swap_why, listing(inst)))
        rep.add("Т1-3 A и B живы после второго обновления",
                meds["A"].alive() and meds["B"].alive() and bool(meds["A"].request("ping", {}, 10)) and bool(meds["B"].request("ping", {}, 10)),
                "A жив: %s; B жив: %s" % (meds["A"].alive(), meds["B"].alive()))
        stop_gui(exe)

        # ---- 2/3. EOF: посредники выходят сами с кодом 0 ----
        ca, cb = meds["A"].eof(), meds["B"].eof()
        rep.add("Т1-2 EOF stdin: A (образ в .old-*) и B выходят сами с кодом 0", ca == 0 and cb == 0, "код A %s, код B %s" % (ca, cb))

        # ---- 4. тихое удаление при живом посреднике C ----
        meds["C"] = Mediator(exe, env, os.path.join(out, "mediator-C.stderr.txt"), "C")
        r = meds["C"].initialize()
        rep.add("Т1-4 посредник C перед удалением", bool(r) and "result" in r, "ответ: %s" % (meds["C"].seen[-1:],))
        before = listing(inst)
        uninst = os.path.join(inst, "uninstall.exe")
        t0 = time.monotonic()
        # uninstall.exe копирует себя в TEMP и сразу выходит: ждём исчезновения ключа реестра (как Invoke-QuietUninstall)
        subprocess.Popen([uninst, "/S"], env=env)
        gone = wait_for(lambda: reg_version() is None, 60)
        time.sleep(3)
        left = listing(inst) if os.path.isdir(inst) else []
        # Ожидание по installer.nsi: /S + MessageBox /SD IDCANCEL - без окна и без зависания; занятый exe остаётся,
        # свободные .old* стёрты, uninstall.exe и ярлыки удалены, реестр чист; папка остаётся из-за exe
        exe_left = os.path.isfile(exe)
        olds = [n for n in os.listdir(inst) if ".old" in n] if os.path.isdir(inst) else []
        lnk = os.path.join(roaming, "Microsoft", "Windows", "Start Menu", "Programs", "Sosnina Albums.lnk")
        rep.add("Т1-4 тихое удаление при живом C: не висит, реестр чист", bool(gone),
                "за %.0f с; DisplayVersion %s; до: %s; после: %s" % (time.monotonic() - t0, reg_version(), before, left))
        rep.add("Т1-4 занятый exe остаётся, остальное удалено", exe_left and not olds and not os.path.exists(uninst) and not os.path.exists(lnk),
                "exe остался: %s; .old*: %s; uninstall.exe: %s; ярлык меню: %s" % (exe_left, olds, os.path.exists(uninst), os.path.exists(lnk)))
        rep.add("Т1-4 C пережил удаление", meds["C"].alive() and bool(meds["C"].request("ping", {}, 10)), "C жив: %s" % meds["C"].alive())
        cc = meds["C"].eof()
        rep.add("Т1-4 EOF: C выходит с кодом 0, после этого exe удаляется", cc == 0, "код C %s" % cc)
        if cc == 0 and os.path.isfile(exe):
            try:
                os.remove(exe)
                rep.add("Т1-4 exe удаляется после выхода C", True, "удалён")
            except OSError as e:
                rep.add("Т1-4 exe удаляется после выхода C", False, "не удалился: %s" % e)
    except Exception as e:  # сбой самого сценария - в итог, остальное (дамп) всё равно соберётся
        import traceback
        rep.add("сценарий", False, "исключение: %s" % traceback.format_exc().replace("\n", " / "))
    return finish(rep, out, exe, inst, data, meds)


def shutil_rmtree(path):
    import shutil
    shutil.rmtree(path, ignore_errors=True)


def finish(rep, out, exe, inst, data, meds):
    """Снять процессы, собрать дамп состояния, записать итоги; код 1, если есть провалы."""
    for m in meds.values():
        if m.alive():
            m.eof(5)
        m.close_files()
    try:
        stop_gui(exe)
    except Exception:
        pass
    dump = ["состояние на конец прогона", "DisplayVersion: %s" % reg_version(), "папка установки: %s" % listing(inst),
            "папка данных стенда: %s" % listing(data), "mcp: %s" % listing(os.path.join(data, "mcp")),
            "процессы exe: %s" % (processes(exe),)]
    for k, m in meds.items():
        dump.append("посредник %s: код %s; stdout: %s" % (k, m.proc.poll(), m.seen[-6:]))
    log = os.path.join(data, "log.txt")
    if os.path.isfile(log):
        with open(log, encoding="utf-8", errors="replace") as f:
            dump.append("log.txt (хвост):\n" + "".join(f.readlines()[-40:]))
    text = "\n".join(dump)
    write_text(os.path.join(out, "state-dump.txt"), text)
    write_text(os.path.join(out, "summary.md"), rep.markdown())
    if rep.failed():
        print("\n" + text, flush=True)
    skipped = [r for r in rep.rows if r[1] == "skip"]
    print("\nпунктов: %d, провалов: %d, не проверено: %d" % (len(rep.rows), len(rep.failed()), len(skipped)), flush=True)
    return 1 if rep.failed() else 0


if __name__ == "__main__":
    sys.exit(main())
