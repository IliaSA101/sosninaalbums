"""Подъём и остановка стенда e2e-регресса (166, design §2): сборка фронта и dev-программы, xvfb, группа процессов.

Чистые части (имена папок прогона, отбор старых, порт, хвост вывода) проверяет test_regress.py без стенда.
Всё пишется в .gotmp/e2e/regress/<время>/; за пределами этой папки ничего не создаётся и не удаляется.
Окно dev-программы живёт в Xvfb: на рабочем столе владельца ничего не появляется, даже если DISPLAY задан.
"""

from __future__ import annotations

import datetime as dt
import os
import re
import shutil
import signal
import socket
import subprocess
import time
import urllib.request
from collections.abc import Callable
from pathlib import Path

from common import PORT_MAX, PORT_MIN, ROOT, Refuse, check_port, check_stand, slow_factor

REGRESS_OUT = ROOT / ".gotmp" / "e2e" / "regress"
DEFAULT_PORT = 34117  # QA — 34116, e2e 089 — 34118 (решение 166-12)
KEEP_RUNS = 5
RUN_RX = re.compile(r"^\d{8}-\d{6}(-\d{1,3})?$")  # старые прогоны удаляются только по этому шаблону имени (суффикс — два прогона в одну секунду)
MIN_AGE = dt.timedelta(hours=2)  # моложе — не трогаем: параллельный агент может ещё работать в этой папке (decisions 166-14)
BUILD_TAGS = "dev,webkit2_41"
READY_S = 90.0  # предел ожидания «GET / отвечает 200», умножается на SA_TEST_SLOW


def run_name(now: dt.datetime | None = None) -> str:
    return (now or dt.datetime.now()).strftime("%Y%m%d-%H%M%S")


def make_run_dir(base: Path, now: dt.datetime | None = None) -> Path:
    """Новая папка прогона; если в эту секунду уже есть такая (параллельный запуск) — с суффиксом -2, -3…"""
    name = run_name(now)
    for i in range(1, 100):
        p = base / (name if i == 1 else f"{name}-{i}")
        try:
            p.mkdir(parents=True)
        except FileExistsError:
            continue
        return p
    raise Refuse(f"не удалось создать папку прогона в {base}: слишком много запусков в одну секунду")


def check_run_dir(run_dir: Path) -> Path:
    """Папки стенда пишутся только внутрь .gotmp/e2e/regress: пустой или чужой путь — отказ, не «текущая папка»."""
    if not str(run_dir).strip() or not run_dir.is_absolute():
        raise Refuse(f"папка прогона {str(run_dir)!r}: нужен абсолютный путь внутри {REGRESS_OUT}")
    base, p = REGRESS_OUT.resolve(), run_dir.resolve()
    if base not in p.parents:
        raise Refuse(f"папка прогона {run_dir} вне {base}")
    return p


def run_time(name: str) -> dt.datetime:
    return dt.datetime.strptime(name[:15], "%Y%m%d-%H%M%S")


IS_NT = os.name == "nt"
STILL_ACTIVE = 259
QUERY_LIMITED = 0x1000  # PROCESS_QUERY_LIMITED_INFORMATION
ERROR_INVALID_PARAMETER = 87  # OpenProcess: процесса с таким PID нет


def _pid_alive_nt(pid: int) -> bool:
    """Windows: os.kill(pid, 0) там TerminateProcess и убил бы чужой процесс с переиспользованным PID — спрашиваем код выхода.

    Любая ошибка, кроме «нет такого процесса», — считаем живым: лучше не удалить папку, чем удалить чужую работу.
    """
    try:
        import ctypes
        from ctypes import wintypes

        # Свой экземпляр с типами и use_last_error: общий windll.kernel32 разделяют все, кто правит argtypes, а GetLastError без этого ненадёжен.
        k32 = ctypes.WinDLL("kernel32", use_last_error=True)
        k32.OpenProcess.argtypes = [wintypes.DWORD, wintypes.BOOL, wintypes.DWORD]
        k32.OpenProcess.restype = wintypes.HANDLE
        k32.GetExitCodeProcess.argtypes = [wintypes.HANDLE, ctypes.POINTER(wintypes.DWORD)]
        k32.GetExitCodeProcess.restype = wintypes.BOOL
        k32.CloseHandle.argtypes = [wintypes.HANDLE]
        k32.CloseHandle.restype = wintypes.BOOL
        h = k32.OpenProcess(QUERY_LIMITED, False, pid)
        if not h:
            return ctypes.get_last_error() != ERROR_INVALID_PARAMETER
        try:
            code = wintypes.DWORD()
            if not k32.GetExitCodeProcess(h, ctypes.byref(code)):
                return True
            return code.value == STILL_ACTIVE
        finally:
            k32.CloseHandle(h)
    except Exception:  # noqa: BLE001 — нет ctypes/windll или сбой вызова
        return True


def pid_alive(pid: int) -> bool:
    if pid <= 0 or pid > 0xFFFFFFFF:  # PID — 32 бита; битый run.pid не должен ронять чистку
        return False
    if IS_NT:
        return _pid_alive_nt(pid)
    try:
        os.kill(pid, 0)  # сигнал 0 на Unix ничего не посылает, только проверяет
    except ProcessLookupError:
        return False
    except PermissionError:
        return True  # процесс есть, но чужой
    except (OverflowError, ValueError):
        return False  # PID не влезает в pid_t
    return True


def run_alive(run: Path) -> bool:
    """Папка в работе: жив скрипт прогона (run.pid) или оставленный --keep стенд (stand.pid)."""
    for f in ("run.pid", "stand.pid"):
        try:
            if pid_alive(int(run.joinpath(f).read_text(encoding="utf-8").strip())):
                return True
        except (OSError, ValueError, OverflowError):
            continue
    return False


def stale_runs(names: list[str], keep: int = KEEP_RUNS, now: dt.datetime | None = None, alive: Callable[[str], bool] | None = None) -> list[str]:
    """Имена папок прогонов к удалению: по шаблону времени, кроме последних keep, не моложе MIN_AGE и без живого процесса.

    Чужие имена не трогаются. alive(имя) — есть ли живой процесс прогона или стенда в этой папке.
    """
    now = now or dt.datetime.now()
    alive = alive or (lambda _n: False)
    ours = sorted(n for n in names if RUN_RX.match(n))
    old = ours[: max(0, len(ours) - keep)]
    return [n for n in old if now - run_time(n) >= MIN_AGE and not alive(n)]


def prune_runs(base: Path, keep: int = KEEP_RUNS, now: dt.datetime | None = None) -> list[str]:
    if not base.is_dir():
        return []
    gone = stale_runs([p.name for p in base.iterdir() if p.is_dir()], keep, now, lambda n: run_alive(base / n))
    for n in gone:
        shutil.rmtree(base / n, ignore_errors=True)
    return gone


def check_regress_port(port: int) -> int:
    """Порт в допустимых границах (34115 — отказ) и свободен: иначе прогон попал бы на чужой стенд."""
    check_port(port)
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)  # хвосты TIME_WAIT прошлого прогона не помеха, как и для Go
        try:
            s.bind(("127.0.0.1", port))
        except OSError:
            raise Refuse(f"порт {port} занят: возьмите другой в {PORT_MIN}–{PORT_MAX}") from None
    return port


def tail(text: str, n: int = 40) -> str:
    return "\n".join((text or "").splitlines()[-n:])


def need_tools(tree: Path, attach: bool = False) -> None:
    """Всё нужное до первой записи; нет чего-то — отказ с подсказкой. --attach: без xvfb, tar, node, git и фронта."""
    for tool in ("go",) if attach else ("xvfb-run", "go", "node", "git", "tar"):
        if shutil.which(tool) is None:
            raise Refuse(f"нет «{tool}» в PATH" + (" (xvfb: apt install xvfb)" if tool == "xvfb-run" else ""))
    try:
        import playwright.sync_api  # noqa: F401
    except ImportError:
        raise Refuse("нет Python Playwright (pip install playwright; playwright install chromium)") from None
    if attach:
        return
    if not (ROOT / "frontend" / "node_modules").is_dir():
        raise Refuse("нет frontend/node_modules: выполните npm ci в frontend")
    if not tree.is_dir():
        raise Refuse(f"нет папки дерева {tree}")


def git_common(path: Path) -> str:
    r = subprocess.run(["git", "-C", str(path), "rev-parse", "--path-format=absolute", "--git-common-dir"],
                       capture_output=True, text=True, check=False)
    return r.stdout.strip()


def check_tree(arg: str | None) -> Path | None:
    """--tree: только рабочая копия этого же репозитория (общий .git); None — чистый срез HEAD."""
    if arg is None:
        return None
    p = Path(arg)
    if not p.is_absolute() or not p.is_dir():
        raise Refuse(f"--tree {arg!r}: нужен абсолютный путь к существующей папке")
    mine, theirs = git_common(ROOT), git_common(p)
    if not theirs or theirs != mine:
        raise Refuse(f"--tree {arg!r}: не рабочая копия этого репозитория")
    return p.resolve()


def run(cmd: list[str], cwd: Path, what: str, env: dict | None = None, log: Path | None = None) -> str:
    """Короткая команда сборки; провал — Refuse с хвостом вывода (рабочее дерево может быть в середине правки)."""
    r = subprocess.run(cmd, cwd=cwd, env=env, capture_output=True, text=True, encoding="utf-8", errors="replace", check=False)
    out = (r.stdout or "") + (r.stderr or "")
    if log is not None:
        log.write_text(out, encoding="utf-8")
    if r.returncode != 0:
        raise Refuse(f"{what} не удалась (код {r.returncode}); хвост вывода:\n{tail(out)}")
    return out


def prepare_tree(run_dir: Path, tree: Path | None) -> Path:
    """Дерево для сборки: рабочая копия как есть или чистый срез HEAD (`git archive`) в run_dir/src."""
    if tree is not None:
        if not (tree / "frontend" / "dist").is_dir():
            raise Refuse(f"в {tree} нет frontend/dist (нужен go:embed): соберите фронт или возьмите срез HEAD без --tree")
        return tree
    src = run_dir / "src"
    src.mkdir(parents=True)
    tar = run_dir / "src.tar"
    run(["git", "-C", str(ROOT), "archive", "--format=tar", "-o", str(tar), "HEAD"], ROOT, "git archive")
    run(["tar", "-xf", str(tar), "-C", str(src)], ROOT, "распаковка среза")
    tar.unlink()
    (src / "frontend" / "dist").mkdir(exist_ok=True)  # go:embed в чистом срезе, как gitkeep в клоне
    (src / "frontend" / "dist" / "gitkeep").write_text("", encoding="utf-8")
    nm = src / "frontend" / "node_modules"
    nm.symlink_to(ROOT / "frontend" / "node_modules", target_is_directory=True)
    return src


def build(src: Path, run_dir: Path) -> tuple[Path, Path]:
    """vite build в run_dir/dist и go build -tags dev,webkit2_41 в run_dir/bin; возвращает (бинарь, dist)."""
    dist, binp = run_dir / "dist", run_dir / "bin" / "sa-dev"
    binp.parent.mkdir(parents=True, exist_ok=True)
    run(["npx", "vite", "build", "--outDir", str(dist), "--emptyOutDir"], src / "frontend", "сборка фронта (vite build)", log=run_dir / "vite.log")
    run(["go", "build", "-tags", BUILD_TAGS, "-o", str(binp), "."], src, "сборка программы (go build)", log=run_dir / "gobuild.log")
    return binp, dist


def seed_data(src: Path, run_dir: Path) -> Path:
    """Синтетические фото и альбомы Alpha/Beta (scripts/qa-albums.go): одинаковы локально и в CI.

    src — дерево, где запускается go run: срез/--tree для --up, корень репозитория для --attach.
    """
    data = run_dir / "data"
    gotmp = ROOT / ".gotmp"
    gotmp.mkdir(exist_ok=True)
    env = dict(os.environ, GOTMPDIR=str(gotmp))  # временные сборки go — в .gotmp (антивирус, правило проекта)
    run(["go", "run", "scripts/qa-albums.go", "-out", str(data), "-photos", "12", "-spreads", "3"], src, "набор данных (qa-albums)", env=env)
    return data


def stand_env(run_dir: Path, stand: str, base: dict | None = None) -> dict:
    """Среда стенда: все XDG_* внутри папки прогона — настройки, кэш, резервные копии черновика и состояние не уходят в ~/."""
    home = check_run_dir(run_dir) / "home"
    e = dict(os.environ if base is None else base)
    e.update(SA_STAND=check_stand(stand), XDG_CONFIG_HOME=str(home / "config"), XDG_CACHE_HOME=str(home / "cache"),
             XDG_DATA_HOME=str(home / "data"), XDG_STATE_HOME=str(home / "state"))
    return e


class StandProc:
    """Dev-программа в xvfb-run, в собственной группе процессов: остановка — killpg, ничего по имени."""

    def __init__(self, binp: Path, dist: Path, run_dir: Path, port: int, stand: str):
        self.binp, self.dist, self.run_dir, self.port = binp, dist, run_dir, port
        self.stand = check_stand(stand)
        self.proc: subprocess.Popen | None = None
        self.log = run_dir / "stand.log"

    def env(self) -> dict:
        return stand_env(self.run_dir, self.stand)

    def start(self) -> None:
        check_run_dir(self.run_dir)
        for sub in ("config", "cache", "data", "state"):
            (self.run_dir / "home" / sub).mkdir(parents=True, exist_ok=True)
        check_regress_port(self.port)  # порт могли занять за время сборки
        f = self.log.open("wb")
        self.proc = subprocess.Popen(
            ["xvfb-run", "-a", str(self.binp), "-devserver", f"localhost:{self.port}", "-assetdir", str(self.dist)],
            env=self.env(), stdout=f, stderr=subprocess.STDOUT, stdin=subprocess.DEVNULL, start_new_session=True,
        )
        f.close()
        (self.run_dir / "stand.pid").write_text(str(self.proc.pid), encoding="utf-8")
        self.wait_ready()

    def wait_ready(self) -> None:
        end = time.monotonic() + READY_S * slow_factor()
        while time.monotonic() < end:
            if self.proc.poll() is not None:
                raise Refuse(f"стенд завершился с кодом {self.proc.returncode}; stand.log:\n{tail(self.log_text())}")
            if "address already in use" in self.log_text():
                raise Refuse(f"порт {self.port} занят другим процессом (сразу после проверки); stand.log:\n{tail(self.log_text())}")
            try:
                with urllib.request.urlopen(f"http://localhost:{self.port}/", timeout=2) as r:
                    if r.status == 200:
                        return
            except OSError:
                pass
            time.sleep(0.2)  # опрос порта до браузера, не тест: стража sleep касается только regress/reg_*.py
        raise Refuse(f"стенд не ответил за {READY_S * slow_factor():g} с; stand.log:\n{tail(self.log_text())}")

    def log_text(self) -> str:
        try:
            return self.log.read_text(encoding="utf-8", errors="replace")
        except OSError:
            return ""

    def stop(self) -> None:
        """SIGTERM своей группе, через 10 с SIGKILL; затем порт должен быть свободен."""
        p, self.proc = self.proc, None
        if p is None:
            return
        for sig, wait in ((signal.SIGTERM, 10.0), (signal.SIGKILL, 5.0)):
            try:
                os.killpg(p.pid, sig)
            except ProcessLookupError:
                break
            try:
                p.wait(timeout=wait)
                break
            except subprocess.TimeoutExpired:
                continue
        try:
            os.killpg(p.pid, signal.SIGKILL)  # дети (Xvfb, программа) могли пережить xvfb-run
        except ProcessLookupError:
            pass
        end = time.monotonic() + 5
        while time.monotonic() < end:
            with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
                if s.connect_ex(("127.0.0.1", self.port)) != 0:
                    return
            time.sleep(0.2)
        raise Refuse(f"порт {self.port} не освободился после остановки стенда")
