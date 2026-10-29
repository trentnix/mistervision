# Interlaced Y/C core

This core extends Izzie Walton's standalone interlaced Menu core with S-Video and composite encoding on the analog scaler output. The application still uses the Linux framebuffer. No change to Main, the kernel, or the system Menu core is required.

The encoder is experimental until color and sync have been checked through a direct S-Video/composite adapter. RGB, component, native progressive Y/C, and HDMI retain their existing routes. An RGB-to-S-Video adapter such as Super Video Custard must keep `vga_mode=rgb`.

## Implementation

`scaler-yc.patch` selects the new encoder only when the standalone interlaced modeline and Main's Y/C flag are both active. `interlaced_yc.sv` uses the upstream `yc_out` encoder at the scaler clock. It converts Main's 20 MHz encoder parameters to the standalone raster's 12.5875 MHz clock. The conversion preserves the selected carrier, including NTSC, PAL, PAL60, PAL-M, and Main's carrier adjustments. It does not change the raster, framebuffer dimensions, page flipping, or video decoding.

Parameter conversion uses registered arithmetic and serial division. Settings are published together after conversion. A change during conversion discards the pending result. This work happens in the FPGA, not on the ARM processor.

The core and its upstream source are pinned in `tools/interlaced-core.json`. The small upstream patch, added RTL, and build script are part of the corresponding source. They remain under GPL-2.0-or-later. Upstream files retain their copyright notices. The prebuilt bitstream is kept here so application builds and installations do not require Quartus.

## Build and test

Install Docker, Python 3.12 or later, `curl`, and `patch`. The build uses a pinned Quartus 17.0.2 container. The output directory must not exist:

```sh
python3 fpga/interlaced/build.py /tmp/interlaced-yc-build
```

The script verifies the upstream archive, applies the patch, builds the FPGA image, rejects negative timing slack, and creates a compressed `InterlacedMenu.rbf`. Review the compiler reports before replacing the bundled bitstream. Update the manifest and application checksum together. The build date is pinned. Compare rebuilt artifacts rather than assuming different hosts produce identical bitstreams.

With Verilator 5.006 or later, a C++ compiler, and Make, run the simulations against prepared source:

```sh
python3 fpga/interlaced/build.py /tmp/interlaced-yc-simulation --prepare-only
python3 fpga/interlaced/test.py /tmp/interlaced-yc-simulation
```

The tests measure carrier frequency, check burst timing and sync alignment, exercise settings changes, and compare the actual scaler-output block against upstream RGB/component output. They also compile the path with Y/C disabled. Simulation adapts two upstream declarations for Verilator without changing synthesis source.

Physical validation must check color, field lock, cropping, and motion through the intended adapter and television. Software framebuffer readback cannot validate the analog signal.
