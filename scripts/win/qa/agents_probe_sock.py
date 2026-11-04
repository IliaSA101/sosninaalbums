#!/usr/bin/env python3
"""Проба сокета службы агентов (win-qa, agents_exe.py): connect по AF_UNIX через winsock, как делает посредник.
Python для Windows не всегда собран с AF_UNIX, поэтому вызовы winsock - напрямую через ctypes.
Печатает JSON {"ok": bool, "err": код WSA}; с вторым аргументом пишет его же в файл (процесс другого пользователя).
Запуск: python -I agents_probe_sock.py <путь сокета> [<файл результата>]. Только Windows; stdlib."""

import ctypes
import json
import sys


def probe(path):
    ws = ctypes.WinDLL("ws2_32")
    wsadata = ctypes.create_string_buffer(512)
    if ws.WSAStartup(0x0202, wsadata) != 0:
        return {"ok": False, "err": -1, "why": "WSAStartup"}
    ws.socket.restype = ctypes.c_size_t
    ws.socket.argtypes = [ctypes.c_int, ctypes.c_int, ctypes.c_int]
    ws.connect.argtypes = [ctypes.c_size_t, ctypes.c_void_p, ctypes.c_int]
    ws.closesocket.argtypes = [ctypes.c_size_t]
    s = ws.socket(1, 1, 0)  # AF_UNIX, SOCK_STREAM
    if s == ctypes.c_size_t(-1).value:
        return {"ok": False, "err": ws.WSAGetLastError(), "why": "socket"}
    raw = path.encode("utf-8")
    if len(raw) >= 108:
        ws.closesocket(s)
        return {"ok": False, "err": -2, "why": "путь длиннее sun_path"}
    addr = ctypes.create_string_buffer(2 + 108)
    addr[0:2] = (1).to_bytes(2, "little")  # sun_family = AF_UNIX
    addr[2:2 + len(raw)] = raw
    rc = ws.connect(s, addr, 110)
    err = 0 if rc == 0 else ws.WSAGetLastError()
    ws.closesocket(s)
    return {"ok": rc == 0, "err": err}


if __name__ == "__main__":
    if len(sys.argv) < 2:
        sys.exit(2)
    res = json.dumps(probe(sys.argv[1]))
    print(res)
    if len(sys.argv) > 2:
        with open(sys.argv[2], "w", encoding="utf-8") as f:
            f.write(res)
