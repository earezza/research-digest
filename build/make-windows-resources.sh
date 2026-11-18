#!/bin/sh
# Rebuilds the Windows icon resources (needs llvm-rc and llvm-cvtres). The .syso files are committed,
# so ordinary builds do not need this; Go links them into Windows builds automatically.
set -e
cd "$(dirname "$0")"
llvm-rc /FO app.res app.rc
llvm-cvtres /MACHINE:X64   /OUT:../rsrc_windows_amd64.syso app.res
llvm-cvtres /MACHINE:ARM64 /OUT:../rsrc_windows_arm64.syso app.res
rm -f app.res
