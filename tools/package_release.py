"""Build a matched MiSTer release and source bundle from a clean Git checkout."""

import argparse
import gzip
import hashlib
import io
import json
import os
from pathlib import Path
import re
import subprocess
import tarfile
import tempfile
import time
import zipfile

def interlaced_files(root, download=False):
    """Read pinned core and source bytes, downloading only during release builds."""
    manifest = json.loads((root / "tools/interlaced-core.json").read_text())
    cache = root / "build/interlaced-core"
    cache.mkdir(parents=True, exist_ok=True)
    result = {}
    for name, spec in manifest["files"].items():
        path = cache / name
        if not path.exists() and download:
            with tempfile.TemporaryDirectory(dir=cache) as temporary:
                candidate = Path(temporary) / name
                subprocess.check_call(["curl", "--fail", "--location", "--silent", "--show-error",
                                       "--retry", "3", "--max-time", "180", spec["url"], "-o", str(candidate)])
                if sha256(candidate.read_bytes()) != spec["sha256"]:
                    raise ValueError(f"interlaced component checksum mismatch: {name}")
                os.replace(candidate, path)
        data = path.read_bytes()
        if sha256(data) != spec["sha256"]:
            raise ValueError(f"interlaced component checksum mismatch: {name}")
        result[name] = data
    return result


ROOT = Path(__file__).resolve().parents[1]
VERSION_PATTERN = r"v(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)"


def git(root, *args):
    """Read Git metadata without including ignored configuration or build files."""
    return subprocess.check_output(["git", "-C", str(root), *args])


def checkout(root, version):
    """Reject ambiguous versions and source that cannot be reproduced from Git."""
    if re.fullmatch(VERSION_PATTERN, version) is None:
        raise ValueError("release version must be vMAJOR.MINOR.PATCH")
    if git(root, "status", "--porcelain", "--untracked-files=all").strip():
        raise ValueError("release builds require a clean checkout, including untracked files")
    return git(root, "rev-parse", "HEAD").decode().strip()


def sha256(data):
    return hashlib.sha256(data).hexdigest()


def arm_executable(path):
    """Read a little-endian ARM ELF executable, rejecting host or missing builds."""
    data = path.read_bytes()
    if len(data) < 52 or data[:6] != b"\x7fELF\x01\x01" or data[18:20] != b"\x28\x00":
        raise ValueError(f"not a 32-bit little-endian ARM executable: {path.name}")
    if int.from_bytes(data[16:18], "little") not in (2, 3):
        raise ValueError(f"not an executable ELF file: {path.name}")
    return data


def player_source(root):
    """Verify the exported upstream archive and read its unmodified license texts."""
    build = root / "build"
    script = (root / "docker/build-mplayer.sh").read_text()
    version_match = re.search(r"^MPLAYER_VER=([0-9.]+)$", script, re.M)
    checksum_match = re.search(r"^MPLAYER_SHA256=([0-9a-f]{64})$", script, re.M)
    if version_match is None or checksum_match is None:
        raise ValueError("MPlayer build recipe must declare its version and source checksum")
    version, expected = version_match.group(1), checksum_match.group(1)
    archive = (build / "mistervision-mplayer-source.tar.xz").read_bytes()
    if sha256(archive) != expected:
        raise ValueError("MPlayer source archive checksum does not match the build recipe")
    licenses = {}
    with tarfile.open(fileobj=io.BytesIO(archive), mode="r:xz") as source:
        for name in ("LICENSE", "Copyright", "ffmpeg/COPYING.GPLv2", "ffmpeg/COPYING.GPLv3", "ffmpeg/COPYING.LGPLv2.1", "ffmpeg/COPYING.LGPLv3"):
            member = source.getmember(f"MPlayer-{version}/{name}")
            if not member.isfile():
                raise ValueError(f"MPlayer license is not a regular file: {name}")
            licenses[f"mistervision/licenses/mplayer/{name}"] = source.extractfile(member).read()
    return archive, licenses


def source_bundle(root, path, version, epoch, upstream, core_source):
    """Include committed project source and the exact upstream player archive."""
    prefix = f"mistervision-{version}/"
    committed = git(root, "archive", "--format=tar", f"--prefix={prefix}", "HEAD")
    with path.open("wb") as raw, gzip.GzipFile(filename="", mode="wb", fileobj=raw, mtime=epoch) as compressed:
        with tarfile.open(fileobj=compressed, mode="w") as output:
            with tarfile.open(fileobj=io.BytesIO(committed)) as source:
                for member in source:
                    output.addfile(member, source.extractfile(member) if member.isfile() else None)
            for name, data in (("docker/MPlayer-source.tar.xz", upstream),
                               ("third_party/Menu_MiSTer-source.tar.gz", core_source)):
                member = tarfile.TarInfo(prefix + name)
                member.size = len(data)
                member.mtime = epoch
                member.mode = 0o644
                output.addfile(member, io.BytesIO(data))


def write_bundle(root, version, revision, output):
    """Package freshly built files without copying active configuration or state."""
    epoch = int(git(root, "show", "-s", "--format=%ct", "HEAD"))
    upstream, licenses = player_source(root)
    core = interlaced_files(root)
    payload = {
        "Scripts/MiSTerVision.sh": (root / "tools/mistervision.sh").read_bytes(),
        "mistervision/InterlacedMenu.rbf": core["InterlacedMenu.rbf"],
        "mistervision/licenses/interlaced-menu.txt": (root / "docs/licenses/interlaced-menu.txt").read_bytes(),
        "mistervision/licenses/interlaced-GPL-2.0.txt": licenses["mistervision/licenses/mplayer/ffmpeg/COPYING.GPLv2"],
        "mistervision/mistervision": arm_executable(root / "build/mistervision-arm"),
        "mistervision/mplayer-arm": arm_executable(root / "build/mistervision-mplayer-arm"),
        "mistervision/jellyfin.conf.example": (root / "jellyfin.conf.example").read_bytes(),
        "mistervision/settings.example.json": (root / "settings.example.json").read_bytes(),
        "mistervision/LICENSE": (root / "LICENSE").read_bytes(),
        "mistervision/licenses/go-LICENSE": (root / "build/go-LICENSE").read_bytes(),
        "mistervision/licenses/fusion-pixel.txt": (root / "docs/licenses/fusion-pixel.txt").read_bytes(),
        "mistervision/licenses/noto.txt": (root / "docs/licenses/noto.txt").read_bytes(),
        "mistervision/licenses/go-text.txt": (root / "docs/licenses/go-text.txt").read_bytes(),
        "mistervision/licenses/go-extensions.txt": (root / "docs/licenses/go-extensions.txt").read_bytes(),
        "mistervision/licenses/coder-websocket.txt": (root / "docs/licenses/coder-websocket.txt").read_bytes(),
        "mistervision/VERSION": (version + "\n").encode(),
        "mistervision/UPDATE_FORMAT": b"1\n",
        "INSTALL.txt": (root / "tools/release-install.txt").read_bytes(),
        **licenses,
    }
    # Keep notices readable outside the source checkout. Relative source links
    # point to the same revision, also provided in the accompanying source bundle.
    notices = (root / "docs/THIRD_PARTY.md").read_text()

    def source_link(match):
        label, target = match.groups()
        if "://" in target:
            return match.group(0)
        relative = os.path.normpath("docs/" + target)
        return f"[{label}](https://github.com/trentnix/mistervision/blob/{revision}/{relative})"
    payload["mistervision/THIRD_PARTY.md"] = re.sub(r"\[([^\]]+)\]\(([^)]+)\)", source_link, notices).encode()
    metadata = (root / "build/release-manifest.txt").read_bytes()
    payload["mistervision/BUILD.txt"] = f"Version: {version}\nRevision: {revision}\n\n".encode() + metadata
    # Retain the established filename so installed updaters and Downloader
    # continue to recognize the single package. Interlacing is a setting.
    launcher = payload["Scripts/MiSTerVision.sh"]
    if launcher.count(b"MISTERVISION_INITIAL_INTERLACED=0") != 1 or b"MISTERVISION_INITIAL_INTERLACED=1" in launcher:
        raise ValueError("launcher must default to non-interlaced output")
    path = output / f"mistervision-{version}-progressive.zip"
    write_zip(path, payload, epoch)
    paths = [path]
    source_path = output / f"mistervision-{version}-source.tar.gz"
    source_bundle(root, source_path, version, epoch, upstream, core["Menu_MiSTer-source.tar.gz"])
    paths.append(source_path)
    (output / "SHA256SUMS").write_text("".join(f"{sha256(path.read_bytes())}  {path.name}\n" for path in paths))


def write_zip(path, payload, epoch):
    """Write reproducible file metadata and a complete inner checksum manifest."""
    payload = dict(payload)
    payload["SHA256SUMS"] = "".join(f"{sha256(data)}  {name}\n" for name, data in sorted(payload.items())).encode()
    executable = {"Scripts/MiSTerVision.sh", "mistervision/mistervision", "mistervision/mplayer-arm"}
    stamp = time.gmtime(max(epoch, 315532800))[:6]
    with zipfile.ZipFile(path, "w", compression=zipfile.ZIP_DEFLATED, compresslevel=9) as archive:
        for name, data in sorted(payload.items()):
            info = zipfile.ZipInfo(name, stamp)
            info.create_system = 3
            info.external_attr = (0o100755 if name in executable else 0o100644) << 16
            info.compress_type = zipfile.ZIP_DEFLATED
            archive.writestr(info, data)


def build_release(root, version, go):
    """Build both executables from one checkout, then publish a complete local bundle."""
    revision = checkout(root, version)
    destination = root / "build/releases" / version
    if destination.exists():
        raise ValueError(f"release output already exists: {destination}")
    interlaced_files(root, download=True)
    for target in ("arm", "native-player", "release-manifest"):
        subprocess.check_call(["make", target, f"VERSION={version}", f"GO={go}"], cwd=root)
    if checkout(root, version) != revision:
        raise ValueError("source revision changed during the release build")
    destination.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix=".release-", dir=destination.parent) as temporary:
        write_bundle(root, version, revision, Path(temporary))
        if checkout(root, version) != revision:
            raise ValueError("source revision changed during packaging")
        os.rename(temporary, destination)
    print(f"Release files: {destination}")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--version", required=True, help="stable version, such as v1.0.0")
    parser.add_argument("--go", default="go", help="Go compiler executable")
    args = parser.parse_args()
    try:
        build_release(ROOT, args.version, args.go)
    except (OSError, ValueError, subprocess.CalledProcessError, tarfile.TarError) as error:
        parser.exit(1, f"Release build failed: {error}\n")


if __name__ == "__main__":
    main()
