"""Exercise native analog synchronization and unchanged scaler flips without hardware."""
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
with tempfile.TemporaryDirectory() as directory:
    path = Path(directory) / 'vo_fbdev.c'
    path.write_bytes((ROOT / 'docker/vo_fbdev.c').read_bytes())
    for patch in ('vo_fbdev_go.patch', 'vo_fbdev_interlaced.patch'):
        subprocess.run(['patch', '-s', str(path), str(ROOT / 'docker' / patch)], check=True)
    source = path.read_text()
def function(name):
    start = source.index(name + '(')
    start = source.rfind('\nstatic ', 0, start) + 1
    end = source.index('\n}\n', start) + 3
    return source[start:end]

program = r'''
#define _POSIX_C_SOURCE 200809L
#include <assert.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <stdio.h>
#include <unistd.h>
#include <sys/wait.h>
#define PF_SSPI_IO_EN (1u << 20)
#define PF_UIO_SET_FBUF 0x2fu
#define PF_FMT_WORD 0x8016u
#define mp_msg(...) ((void)0)
static uint32_t reg, *pf_gpo=&reg;
static uint32_t pf_page_phys[2]={0x20000000,0x20100000};
static int pf_analog, fb_yres=240, fb_line_len=2560;
static unsigned pf_analog_flips,pf_analog_retries,pf_analog_late;
static int64_t pf_analog_max_us, now;
static int unsupported, stopped, stall, reads, flips, words;
static uint16_t payload[4];
static int64_t av_gettime_relative(void) { return now; }
static void av_usleep(int us) { now+=us; if(stall && now>=950) {now+=1500;stall=0;} }
static uint16_t pf_spi_word(uint32_t base,uint16_t word) {
    (void)base;
    if(word==0x42) {reads++; return unsupported?0:0x100|((stopped?0:(now/1000)+255)&255);}
    if(word==0x2f) {flips++;words=0;}
    assert(words<4);payload[words++]=word;return 0;
}
''' + '\n'.join(function(n) for n in ['pf_analog_requested','pf_analog_counter','pf_analog_blank','pf_analog_border','pf_flip_to']) + r'''
static void expect_failure(int missing) {
    pid_t child=fork();assert(child>=0);
    if(!child) { unsupported=missing;stopped=!missing; pf_analog_blank(); _exit(99); }
    int status;waitpid(child,&status,0);assert(WIFEXITED(status)&&WEXITSTATUS(status)==1);
}
int main(void) {
    assert(!pf_analog_requested());
    setenv("MISTERVISION_NATIVE_ANALOG_SYNC","1",1);assert(!pf_analog_requested());
    setenv("MISTERVISION_PROGRESSIVE_PAGEFLIP","1",1);assert(pf_analog_requested());
    setenv("MISTERVISION_INTERLACED","1",1);assert(!pf_analog_requested());
    unsetenv("MISTERVISION_INTERLACED");
    setenv("MISTERVISION_NATIVE_ANALOG_SYNC","0",1);assert(!pf_analog_requested());
    pf_flip_to(1);assert(reads==0 && flips==1 && words==4);
    assert(payload[0]==0x2f && payload[1]==0x8016 && payload[2]==0 && payload[3]==0x2010);
    pf_analog=1;pf_flip_to(0);
    assert(reads>1 && flips==2 && pf_analog_flips==1 && pf_analog_late==0);
    // Counter wrap from 255 to zero must still count as a fresh transition.
    assert(now==1000);
    now=0;stall=1;pf_analog_retries=0;pf_flip_to(1);
    assert(pf_analog_retries==1 && now==3000);
    expect_failure(1);expect_failure(0);
    for(fb_yres=240;fb_yres<=288;fb_yres+=48) {
        size_t size=(size_t)fb_yres*fb_line_len;
        unsigned char *memory=malloc(size+32);assert(memory);memset(memory,0xaa,size+32);
        pf_analog_border(memory+16);
        for(size_t i=0;i<size+32;i++) {
            int edge=(i>=16&&i<16+(size_t)fb_line_len)||(i>=16+size-fb_line_len&&i<16+size);
            assert(memory[i]==(edge?0:0xaa));
        }
        free(memory);
    }
    puts("adapter: route gating, legacy flip, counter wrap, stall rejection, missing/stopped counter, borders passed");
}
'''
class NativeAnalogTest(unittest.TestCase):
    def test_synchronization_and_legacy_flips(self):
        with tempfile.TemporaryDirectory() as directory:
            path=Path(directory)/'test.c';path.write_text(program)
            binary=Path(directory)/'test'
            subprocess.run(['cc','-std=c11','-Wall','-Wextra','-Werror','-fsanitize=undefined',str(path),'-o',str(binary)],check=True)
            subprocess.run([str(binary)],check=True)
