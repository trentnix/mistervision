"""Exercise the actual experimental C buffer-retirement fence with a fake clock.

Pass the fully patched and instrumented vo_fbdev.c used for the test build.
These checks do not substitute for validating the FPGA counter's edge timing.
"""
from pathlib import Path
import subprocess
import sys
import tempfile

source = Path(sys.argv[1]).read_text()

def function(name):
    start = source.index(name)
    start = source.rfind("static ", 0, start)
    end = source.index("\n}", start) + 2
    return source[start:end]

program = r'''
#include <assert.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <setjmp.h>
static int pf_pending, reads, sleeps, exits;
static uint32_t pf_submitted_field, next_field;
static int64_t now, advance_at;
static jmp_buf timeout;
static int64_t av_gettime_relative(void) { return now; }
static void av_usleep(int us) { now += us; sleeps++; }
static uint32_t pf_field(void) {
    reads++;
    return now >= advance_at ? next_field : pf_submitted_field;
}
#define mp_msg(...) ((void)0)
#define mv_trace(...) ((void)0)
static void failed(int status) { assert(status == 1); exits++; longjmp(timeout, 1); }
#define exit failed
'''
program += function("pf_release_page(void)") + "\n"
program += function("progressive_pageflip_test(void)") + "\n"
program += r'''
int main(void) {
    /* An unused page needs neither a counter read nor a wait. */
    pf_release_page();
    assert(!reads && !sleeps);
    /* Decoding already allowed the previous frame to reach scanout. */
    pf_pending = 1; next_field = 2; pf_submitted_field = 1;
    pf_release_page();
    assert(!pf_pending && !sleeps);
    /* A fast redraw must not overwrite the old front page too early. */
    pf_pending = 1; advance_at = 1500;
    pf_release_page();
    assert(!pf_pending && sleeps == 3 && now == 1500);
    /* Counter wraparound is a change, not a hung display. */
    pf_pending = 1; pf_submitted_field = UINT32_MAX; next_field = 0;
    pf_release_page();
    assert(!pf_pending && sleeps == 3);
    /* A stopped display fails within 60ms, without releasing its page. */
    pf_pending = 1; advance_at = INT64_MAX;
    if (!setjmp(timeout)) { pf_release_page(); assert(0); }
    assert(exits == 1 && pf_pending && now == 61500);
    unsetenv("MISTERVISION_PROGRESSIVE_PAGEFLIP");
    assert(!progressive_pageflip_test());
    const char *invalid[] = {"", "0", "true", "11", "1 "};
    for (unsigned i = 0; i < sizeof(invalid)/sizeof(*invalid); i++) {
        setenv("MISTERVISION_PROGRESSIVE_PAGEFLIP", invalid[i], 1);
        assert(!progressive_pageflip_test());
    }
    setenv("MISTERVISION_PROGRESSIVE_PAGEFLIP", "1", 1);
    assert(progressive_pageflip_test());
    return 0;
}
'''
with tempfile.TemporaryDirectory() as directory:
    work = Path(directory)
    (work / "test.c").write_text(program)
    subprocess.run(["cc", "-std=gnu11", "-Wall", "-Wextra", "-Werror", str(work / "test.c"), "-o", str(work / "test")], check=True)
    subprocess.run([str(work / "test")], check=True)
print("Progressive page-flip fence and opt-in checks passed.")
