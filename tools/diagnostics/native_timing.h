/* Temporary, opt-in native timing capture. No media URLs or titles are logged.
 * Each translation unit writes at most 6000 numeric records to its own file. */
#include <stdio.h>
#include <stdlib.h>
#include <time.h>
#include <unistd.h>
#include <fcntl.h>
#include <sys/stat.h>

static long long mv_clock_us(void)
{
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return (long long)ts.tv_sec * 1000000 + ts.tv_nsec / 1000;
}

static void mv_trace(const char *event, long long a, long long b, long long c)
{
    static FILE *file;
    static int initialized, records;
    static long long flushed;
    const char *enabled = getenv("MISTERVISION_NATIVE_TIMING");
    if (!enabled || enabled[0] != '1' || enabled[1]) return;
    if (!initialized) {
        initialized = 1;
        char path[128];
        snprintf(path, sizeof(path), "/tmp/mv-native-%ld-%s.log", (long)getpid(), MV_TRACE_ROLE);
        int fd = open(path, O_WRONLY | O_CREAT | O_EXCL, 0600);
        if (fd >= 0) {
            file = fdopen(fd, "w");
            if (!file) close(fd);
        }
    }
    if (!file || records >= 6000) return;
    long long now = mv_clock_us();
    if (fprintf(file, "%s %lld %lld %lld %lld\n", event, now, a, b, c) < 0) {
        fclose(file);
        file = NULL;
        return;
    }
    records++;
    if (now - flushed >= 1000000 || records == 6000) {
        fflush(file);
        flushed = now;
    }
}
