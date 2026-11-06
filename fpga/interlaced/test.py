#!/usr/bin/env python3
"""Simulate the scaler encoder against the pinned interlaced encoder using Verilator."""

import argparse
from pathlib import Path
import shutil
import subprocess
import tempfile

HERE = Path(__file__).resolve().parent


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("source", type=Path, help="prepared upstream core source directory")
    args = parser.parse_args()
    with tempfile.TemporaryDirectory(prefix="mistervision-yc-test-") as temporary:
        root = Path(temporary)
        encoder = (args.source / "sys/interlaced_yc_encoder.sv").read_text()
        # Verilator 5.006 loses overlapping nonblocking assignments to members
        # of unpacked structs. Match the synthesis behavior in the simulation.
        encoder = encoder.replace("typedef struct {", "typedef struct packed {")
        (root / "interlaced_yc_encoder.sv").write_text(encoder)
        for name in ("interlaced_yc.sv", "yc_test.sv", "yc_signal_test.sv", "scaler_test.sv"):
            shutil.copyfile(HERE / name, root / name)
        shutil.copyfile(args.source / "sys/vga_out.sv", root / "vga_out.sv")
        top = (args.source / "sys/sys_top.v").read_text()
        block = top.split("wire [23:0] vgas_o, vgas_rgb;", 1)[1].split("wire [23:0] vga_o, vga_o_t;", 1)[0]
        block = "wire [23:0] vgas_rgb;" + block
        block = block.replace("wire vgas_hs, vgas_vs, vgas_cs, vgas_de;", "")
        (root / "test_scaler.sv").write_text("""module test_scaler (
            input clk_hdmi, ypbpr_en, ilace_auto, yc_en, pal_en, cvbs,
            input [39:0] PhaseInc, input [16:0] ColorBurst_Range,
            input [23:0] hdmi_data_osd,
            input hdmi_hs_osd, hdmi_vs_osd, hdmi_cs_osd, hdmi_de_osd,
            output [23:0] vgas_o, output vgas_hs, vgas_vs, vgas_cs, vgas_de
        );
""" + block + "\nendmodule\n")
        for top, files, defines in (
            ("yc_test", ["yc_test.sv"], []),
            ("yc_signal_test", ["yc_signal_test.sv"], []),
            ("scaler_test", ["scaler_test.sv", "test_scaler.sv", "vga_out.sv"], []),
            ("scaler_test", ["scaler_test.sv", "test_scaler.sv", "vga_out.sv"], ["+define+MISTER_DISABLE_YC"]),
            ("scaler_test", ["scaler_test.sv", "test_scaler.sv", "vga_out.sv"], ["+define+MISTER_DEBUG_NOHDMI"]),
            ("scaler_test", ["scaler_test.sv", "test_scaler.sv", "vga_out.sv"], ["+define+MISTER_DEBUG_NOHDMI", "+define+MISTER_DISABLE_YC"]),
        ):
            subprocess.run(["verilator", "--binary", "-j", "4", "--timing", "--top-module", top,
                            "-Wno-fatal", "interlaced_yc.sv", "interlaced_yc_encoder.sv"] + files + defines, cwd=root, check=True)
            subprocess.run([str(root / "obj_dir" / ("V" + top))], cwd=root, check=True)


if __name__ == "__main__":
    main()
