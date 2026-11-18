#!/usr/bin/env python3
"""Puts each Windows and Linux program in a .zip (with its icon, menu entry and installer for Linux), since
many sites and chat apps refuse to pass on bare .exe files. Run after building dist/ (see build.yml)."""
import os, time, zipfile
os.chdir(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

def add(z, path, arc, exe=False):
    info = zipfile.ZipInfo(arc, time.localtime(os.path.getmtime(path))[:6]); info.compress_type = zipfile.ZIP_DEFLATED
    info.external_attr = (0o100755 if exe else 0o100644) << 16
    z.writestr(info, open(path, "rb").read())

for arch in ["x64", "arm64"]:
    exe = f"dist/ResearchDigest-windows-{arch}.exe"
    if os.path.exists(exe):
        with zipfile.ZipFile(f"dist/ResearchDigest-windows-{arch}.zip", "w") as z:
            add(z, exe, "ResearchDigest.exe", True)
    prog = f"dist/ResearchDigest-linux-{arch}"
    if os.path.exists(prog):
        top = f"ResearchDigest-linux-{arch}/"
        with zipfile.ZipFile(f"dist/ResearchDigest-linux-{arch}.zip", "w") as z:
            add(z, prog, top + f"ResearchDigest-linux-{arch}", True)
            add(z, "build/linux/install.sh", top + "install.sh", True)
            add(z, "build/linux/research-digest.desktop", top + "research-digest.desktop")
            for s in [48, 64, 128, 256, 512]:
                add(z, f"build/icon/icon-{s}.png", top + f"icons/icon-{s}.png")
            add(z, "web/icon.svg", top + "icons/icon.svg")
print("zipped:", sorted(f for f in os.listdir("dist") if f.endswith(".zip")))
