#!/usr/bin/env python3
"""Builds dist/ResearchDigest-mac.zip: one ResearchDigest.app that runs on Intel and Apple Silicon Macs.

Run from the project folder after building dist/ResearchDigest-darwin-amd64 and dist/ResearchDigest-darwin-arm64
(see .github/workflows/build.yml). On a Mac, lipo, codesign and ditto are used when present, and the app is
signed ad hoc so macOS can check it has not been changed; elsewhere the two programs are joined by this script
(each keeps the ad-hoc signature Go gives Apple Silicon programs).
"""
import os, re, shutil, struct, subprocess, sys, zipfile

root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
os.chdir(root)
version = re.search(r'var version = "([^"]+)"', open("main.go").read()).group(1)
thin = ["dist/ResearchDigest-darwin-amd64", "dist/ResearchDigest-darwin-arm64"]
app = "dist/ResearchDigest.app"
shutil.rmtree(app, ignore_errors=True)
os.makedirs(app + "/Contents/MacOS"); os.makedirs(app + "/Contents/Resources")
exe = app + "/Contents/MacOS/ResearchDigest"

def fat(paths, out):
    """A universal (fat) Mach-O file: a header listing each architecture, then each program, 16 KB aligned."""
    parts = []
    for p in paths:
        b = open(p, "rb").read()
        magic, cputype, cpusub = struct.unpack_from("<III", b, 0)
        assert magic == 0xFEEDFACF, p + " is not a 64-bit Mach-O program"
        parts.append((cputype, cpusub & 0x00FFFFFF, b))
    align = 14
    off = 1 << align
    header = struct.pack(">II", 0xCAFEBABE, len(parts))
    body, entries = b"", b""
    for cputype, cpusub, b in parts:
        entries += struct.pack(">IIIII", cputype, cpusub, off, len(b), align)
        pad = (-(off + len(b))) % (1 << align)
        body += b + b"\0" * pad
        off += len(b) + pad
    data = header + entries
    data += b"\0" * ((1 << align) - len(data))
    open(out, "wb").write(data + body)

if shutil.which("lipo"):
    subprocess.check_call(["lipo", "-create", "-output", exe] + thin)
else:
    fat(thin, exe)
os.chmod(exe, 0o755)
open(app + "/Contents/Info.plist", "w").write(open("build/mac/Info.plist").read().replace("VERSION", version))
shutil.copy("build/mac/ResearchDigest.icns", app + "/Contents/Resources/ResearchDigest.icns")
open(app + "/Contents/PkgInfo", "w").write("APPL????")

if shutil.which("codesign"):
    subprocess.check_call(["codesign", "--force", "--deep", "--sign", "-", app])
out = "dist/ResearchDigest-mac.zip"
if os.path.exists(out): os.remove(out)
if shutil.which("ditto"):
    subprocess.check_call(["ditto", "-c", "-k", "--keepParent", app, out])
else:
    with zipfile.ZipFile(out, "w", zipfile.ZIP_DEFLATED) as z:
        for d, _, files in os.walk(app):
            for f in files:
                p = os.path.join(d, f)
                info = zipfile.ZipInfo(os.path.relpath(p, "dist"))
                info.external_attr = (0o100755 if os.access(p, os.X_OK) else 0o100644) << 16
                info.compress_type = zipfile.ZIP_DEFLATED
                z.writestr(info, open(p, "rb").read())
print("Built", out, "version", version)
