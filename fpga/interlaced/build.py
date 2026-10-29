#!/usr/bin/env python3
"""Build the interlaced Y/C core from pinned upstream source and a local patch."""

import argparse
import hashlib
import json
import re
from pathlib import Path
import shutil
import subprocess
import tarfile
import tempfile

ROOT = Path(__file__).resolve().parents[2]
HERE = Path(__file__).resolve().parent
IMAGE = "ghcr.io/raetro/quartus@sha256:55442b161163dc7723e3d4917ce046fcb682478365fe9320756c9a4ae7d7d4ce"


def prepare(destination):
    """Extract verified upstream source into a new, disposable build directory."""
    manifest = json.loads((ROOT / "tools/interlaced-core.json").read_text())
    spec = manifest["files"]["Menu_MiSTer-source.tar.gz"]
    archive = ROOT / "third_party/Menu_MiSTer-source.tar.gz"
    if not archive.exists():
        archive = ROOT / "build/interlaced-core/Menu_MiSTer-source.tar.gz"
    if not archive.exists():
        archive.parent.mkdir(parents=True, exist_ok=True)
        with tempfile.TemporaryDirectory(dir=archive.parent) as temporary:
            candidate = Path(temporary) / archive.name
            subprocess.run(["curl", "--fail", "--location", "--max-time", "180",
                            spec["url"], "-o", str(candidate)], check=True)
            if hashlib.sha256(candidate.read_bytes()).hexdigest() != spec["sha256"]:
                raise ValueError("upstream Menu core source checksum mismatch")
            candidate.replace(archive)
    if hashlib.sha256(archive.read_bytes()).hexdigest() != spec["sha256"]:
        raise ValueError("upstream Menu core source checksum mismatch")
    destination.mkdir(parents=True, exist_ok=False)
    with tarfile.open(archive) as source:
        source.extractall(destination, filter="data")
    upstream = next(destination.iterdir())
    for child in upstream.iterdir():
        child.rename(destination / child.name)
    upstream.rmdir()
    subprocess.run(["patch", "--batch", "--fuzz=0", "-p1", "-i", str(HERE / "scaler-yc.patch")],
                   cwd=destination, check=True)
    # Keep the OSD version date stable when rebuilding this source revision.
    build_id = destination / "sys/build_id.tcl"
    build_id.write_text(build_id.read_text().replace(
        "[clock format [ clock seconds ] -format %y%m%d]", manifest["build_date"]))
    shutil.copyfile(HERE / "interlaced_yc.sv", destination / "sys/interlaced_yc.sv")
    with (destination / "files.qip").open("a") as output:
        output.write("\nset_global_assignment -name SYSTEMVERILOG_FILE sys/interlaced_yc.sv\n")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("directory", type=Path, help="new scratch directory for the build")
    parser.add_argument("--prepare-only", action="store_true")
    args = parser.parse_args()
    destination = args.directory.resolve()
    prepare(destination)
    if args.prepare_only:
        return
    docker = ["docker", "run", "--rm", "--network", "none", "-v", f"{destination}:/build", "-w", "/build", IMAGE]
    subprocess.run(docker + ["quartus_sh", "--flow", "compile", "menu"], check=True)
    summary = (destination / "output_files/menu.sta.summary").read_text()
    slack = [float(value) for value in re.findall(r"^Slack\s*:\s*(-?[0-9.]+)", summary, re.M)]
    if not slack or min(slack) < 0:
        raise RuntimeError("FPGA timing requirements not met. Do not deploy this core.")
    # MiSTer's loader requires a compressed raw bitstream.
    subprocess.run(docker + ["quartus_cpf", "-c", "-o", "bitstream_compression=on",
                            "output_files/menu.sof", "InterlacedMenu.rbf"], check=True)
    bitstream = destination / "InterlacedMenu.rbf"
    print(f"{hashlib.sha256(bitstream.read_bytes()).hexdigest()}  {bitstream}")
    print("Review compiler timing reports before updating the bundled core and its checksum.")


if __name__ == "__main__":
    main()
