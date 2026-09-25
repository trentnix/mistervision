#!/usr/bin/env python3
"""Package a v1.6 application and opt-in diagnostic player beside the stable install."""
from pathlib import Path
import hashlib
import shutil
import sys
import zipfile

root = Path(__file__).resolve().parents[2]
dest, player, license_file = map(Path, sys.argv[1:])
app = dest / 'mistervision-pageflip-app-test'
scripts = dest / 'Scripts'
app.mkdir(parents=True, exist_ok=True)
scripts.mkdir(parents=True, exist_ok=True)
for src, name in [(root/'build/mistervision-arm', 'mistervision'), (player, 'mplayer-arm'), (license_file, 'MPLAYER-LICENSE')]:
    shutil.copyfile(src, app/name)
for name in ('mistervision','mplayer-arm'): (app/name).chmod(0o755)
launcher = (root/'tools/mistervision.sh').read_text()
launcher = launcher.replace('set -eu\n', '''set -eu
if pidof mistervision mplayer-arm >/dev/null 2>&1; then
    echo "Exit MiSTerVision before starting this test."
    exit 1
fi
mkdir -p /media/fat/mistervision-pageflip-app-test/logs
umask 077
test_logs=$(mktemp -d /media/fat/mistervision-pageflip-app-test/logs/run-XXXXXXXX)
rm -f /tmp/mv-native-*-output.log /tmp/mv-native-*-clock.log
export MISTERVISION_NATIVE_TIMING=1 MISTERVISION_PROGRESSIVE_TEST=1
unset MISTERVISION_PROGRESSIVE_PAGEFLIP
''', 1)
assert launcher.count('log_file=/media/fat/mistervision/startup.log') == 1
assert launcher.count('    close_log "$status"\n    if') == 1
launcher = launcher.replace('log_file=/media/fat/mistervision/startup.log', 'log_file="$test_logs/startup.log"')
launcher = launcher.replace('binary=/media/fat/mistervision/mistervision', 'binary=/media/fat/mistervision-pageflip-app-test/mistervision')
launcher = launcher.replace('player=/media/fat/mistervision/mplayer-arm', 'player=/media/fat/mistervision-pageflip-app-test/mplayer-arm')
launcher = launcher.replace('MISTERVISION_AUTO_RESTART=1', 'MISTERVISION_AUTO_RESTART=0')
launcher = launcher.replace('    close_log "$status"\n    if', '''    for timing in /tmp/mv-native-*-output.log /tmp/mv-native-*-clock.log; do
        [ -f "$timing" ] || continue
        cp "$timing" "$test_logs/" || true
    done
    close_log "$status"
    if''',1)
path = scripts/'MiSTerVision-Pageflip-App-Test.sh'
path.write_text(launcher)
path.chmod(0o755)
shutil.copyfile(root/'tools/diagnostics/APP-PAGEFLIP.md',dest/'README.md')
files = sorted(p for p in dest.rglob('*') if p.is_file() and p != dest/'SHA256SUMS')
(dest/'SHA256SUMS').write_text(''.join(hashlib.sha256(p.read_bytes()).hexdigest()+'  '+p.relative_to(dest).as_posix()+'\n' for p in files))
with zipfile.ZipFile(dest.parent / (dest.name + '.zip'), 'w', zipfile.ZIP_DEFLATED) as z:
    for p in sorted(dest.rglob('*')):
        if p.is_file(): z.write(p,p.relative_to(dest))
