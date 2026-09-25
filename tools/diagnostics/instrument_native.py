#!/usr/bin/env python3
"""Instrument a fully patched MPlayer source tree for an isolated test build."""
from pathlib import Path
import shutil
import sys


def replace_once(text, old, new):
    if text.count(old) != 1:
        raise ValueError(f'Expected one insertion point: {old!r}')
    return text.replace(old, new, 1)


def instrument(root):
    vo = root / 'libvo/vo_fbdev.c'
    player = root / 'mplayer.c'
    output = vo.read_text()
    main = player.read_text()
    output = replace_once(output, 'static int pf_pending, pf_prepared;', '#define MV_TRACE_ROLE "output"\n#include "../native_timing.h"\nstatic int pf_pending, pf_prepared;')
    output = replace_once(output, '    go_output_fd = fd;', '    go_output_fd = fd;\n    mv_trace("output", fb_xres, fb_yres, access("/tmp/misterdvd_vsync", F_OK) == 0);')
    output = replace_once(output, '        ioctl(fb_dev_fd, FBIO_WAITFORVSYNC, &dummy);', '        long long start = mv_clock_us();\n        int result = ioctl(fb_dev_fd, FBIO_WAITFORVSYNC, &dummy);\n        int error = result < 0 ? errno : 0;\n        mv_trace("wait", mv_clock_us() - start, result, error);')
    output = replace_once(output, '    if (pf_active) pf_release_page();', '    long long start = mv_clock_us();\n    if (pf_active) pf_release_page();')
    output = replace_once(output, '    pf_prepared = pf_active;', '    pf_prepared = pf_active;\n    mv_trace("frame", start, mv_clock_us() - start, pf_active);')
    main = replace_once(main, 'static int total_frame_cnt;', '#define MV_TRACE_ROLE "clock"\n#include "native_timing.h"\nstatic int total_frame_cnt;')
    main = replace_once(main, '            if (!quiet)\n                print_status(a_pts - audio_delay, AV_delay, c_total);', '''            {
                static long long last;
                long long now = mv_clock_us();
                if (now - last >= 1000000) {
                    mv_trace("clock", total_frame_cnt, drop_frame_cnt, (long long)(AV_delay * 1000000));
                    last = now;
                }
            }
            if (!quiet)
                print_status(a_pts - audio_delay, AV_delay, c_total);''')
    # Validate every insertion before changing either source file.
    vo.write_text(output)
    player.write_text(main)
    shutil.copyfile(Path(__file__).with_name('native_timing.h'), root / 'native_timing.h')


if __name__ == '__main__':
    instrument(Path(sys.argv[1]))
