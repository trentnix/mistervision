#!/bin/bash
# Install as /media/fat/Scripts/MiSTerVision.sh. Main_MiSTer does not quote paths.
# Launch from MiSTer's Scripts menu so Main_MiSTer enables framebuffer output.
# Binaries, login, and playback choices persist on the SD card.
set -eu

# ConsoleMode closes its Scripts terminal when the host changes. Keep the whole
# launcher in a separate session so that terminal hangup cannot kill the display
# supervisor before it restores ConsoleMode. Restarted launchers reuse the session.
if [ "${MISTERVISION_CONSOLEMODE_SESSION:-}" != 1 ] && pidof MiSTer_ConsoleMode >/dev/null 2>&1; then
    export MISTERVISION_CONSOLEMODE_SESSION=1
    exec setsid --fork --wait bash "$0" </dev/null >/tmp/mistervision-consolemode.log 2>&1
fi

# Keep early errors even when a display supervisor restores the menu. Capture
# output in bounded chunks, including output without newlines. The logger never
# closes its input on a storage failure, so it cannot interrupt the application.
log_file=/media/fat/mistervision/startup.log
if ! (umask 077; : > "$log_file") 2>/dev/null; then
    log_file=/tmp/mistervision-startup.log
    if ! (umask 077; : > "$log_file") 2>/dev/null; then
        log_file=
        echo "Could not save MiSTerVision startup output." >&2
    fi
fi
version=unknown
if [ -r /media/fat/mistervision/VERSION ]; then
    candidate=$(head -c 80 /media/fat/mistervision/VERSION)
    if [[ "$candidate" =~ ^[a-zA-Z0-9.+_-]+$ ]]; then version=$candidate; fi
fi

capture_output() {
    local chunk result bytes=0 writable=1
    export LC_ALL=C
    umask 077
    rm -f "$log_file.1" 2>/dev/null || true
    while :; do
        chunk=
        result=0
        IFS= read -r -N 4096 -t 1 chunk || result=$?
        if [ -n "$chunk" ]; then
            if [ "$writable" -eq 1 ]; then
                if [ "$((bytes + ${#chunk}))" -gt 65536 ]; then
                    if mv -f "$log_file" "$log_file.1" 2>/dev/null; then
                        bytes=0
                    else
                        writable=0
                    fi
                fi
                if [ "$writable" -eq 1 ]; then
                    printf '%s' "$chunk" >> "$log_file" 2>/dev/null || writable=0
                    bytes=$((bytes + ${#chunk}))
                fi
            fi
            if [ "$writable" -eq 0 ]; then
                printf '%s' "$chunk" >&3 || true
            fi
        fi
        # read returns 1 at EOF and >128 when the flush interval expires.
        if [ "$result" -eq 1 ]; then break; fi
    done
}

close_log() {
    local status=$1
    if [ -n "${logger_pid:-}" ]; then
        printf '\nlauncher_exit=%s installed_version=%s time=%s\n' "$status" "$version" "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
        exec 1>&3 2>&4
        wait "$logger_pid" || true
        logger_pid=
        if [ "$status" -ne 0 ] && [ "$status" -ne 75 ]; then
            # Keep the last failure across later successful launches. Both
            # rolling files together contain at most 128 KiB of output.
            local saved="${log_file%.log}-error.log"
            if (umask 077
                if [ -f "$log_file.1" ]; then cat "$log_file.1" || exit 1; fi
                cat "$log_file"
            ) > "$saved.tmp" 2>/dev/null && mv -f "$saved.tmp" "$saved" 2>/dev/null; then
                :
            else
                rm -f "$saved.tmp" 2>/dev/null || true
                saved=$log_file
            fi
            tail -c 2048 "$saved" >&2 2>/dev/null || true
            printf 'MiSTerVision exited with status %s. Startup log: %s\n' "$status" "$saved" >&2
        fi
    fi
}

exec 3>&1 4>&2
if [ -n "$log_file" ]; then
    exec > >(capture_output) 2>&1
    logger_pid=$!
    printf 'MiSTerVision launcher: installed_version=%s time=%s\n' "$version" "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
fi

# Address the active virtual console directly. Scripts stdout can point at a
# different terminal, leaving the CRT's login text untouched.
clear_console() {
    printf '\033[0m\033[40m\033[2J\033[3J\033[H' > /dev/tty0
}
finish() {
    local status=$?
    close_log "$status"
    if [ "$status" -eq 0 ]; then
        clear_console
    fi
    printf '\033[?25h' > /dev/tty0

    # The display supervisor restores ConsoleMode before returning. Loading
    # the standard core again would undo that handoff. Detect the running host,
    # not merely an installation that may be inactive.
    if pidof MiSTer_ConsoleMode >/dev/null 2>&1; then
        return
    fi

    # MiSTer's Scripts wrapper waits for a key after the launcher returns.
    # Reload the menu on success. Leave errors visible for troubleshooting.
    if [ "$status" -eq 0 ] && [ -p /dev/MiSTer_cmd ] && [ -f /media/fat/menu.rbf ]; then
        if ! timeout 2 sh -c 'printf "%s\n" "load_core /media/fat/menu.rbf" > /dev/MiSTer_cmd'; then
            echo "Could not return to the MiSTer menu. Press any key to continue." >&2
        fi
    fi
}
trap finish EXIT
clear_console
printf '\033[?25l' > /dev/tty0

binary=/media/fat/mistervision/mistervision
player=/media/fat/mistervision/mplayer-arm
if [ ! -x "$binary" ]; then
    echo "Install the Go ARM build at $binary before launching MiSTerVision."
    exit 1
fi

if [ ! -x "$player" ]; then
    echo "Install the Go-specific MPlayer build at $player before launching MiSTerVision."
    exit 1
fi

# Main_MiSTer pins Scripts to CPU 1. Let Go and its decoder share both cores.
# The Cortex-A9 decoder cannot keep up while competing with the UI on one core.
taskset -p 3 "$$" >/dev/null

# The 480i supervisor leaves normal menu return to finish(), but still restores
# hardware itself after a failure. Older launchers retain supervisor restoration.
status=0
# The release preset only seeds settings for a new installation. Existing
# settings and legacy configuration take precedence, including during updates.
MISTERVISION_INITIAL_INTERLACED=0 MISTERVISION_LAUNCHER=1 MISTERVISION_AUTO_RESTART=1 "$binary" -browse \
    -config /media/fat/mistervision/jellyfin.conf \
    -state-dir /media/fat/mistervision/state \
    -player "$player" || status=$?

# Exit 75 means installation and cleanup succeeded. The 480i supervisor has
# restored the normal core. Exec the installed launcher to use updated startup
# logic, even when MiSTer ran this script from a temporary copy. Other exits
# retain finish() behavior. A failed restart stays visible instead of looping.
if [ "$status" -eq 75 ]; then
    if [ ! -x /media/fat/Scripts/MiSTerVision.sh ]; then
        echo "The updated launcher is missing or not executable. Reinstall MiSTerVision." >&2
        exit 1
    fi
    close_log "$status"
    exec /media/fat/Scripts/MiSTerVision.sh
fi
exit "$status"
