#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/progress-bar-3000-smoke.XXXXXX")"
BIN_PATH="${TMP_DIR}/progress-bar-3000"
SOCKET_PATH="/tmp/pb3-smoke-$$.sock"
GOCACHE_DIR="${ROOT_DIR}/.gocache"
GOMODCACHE_DIR="${ROOT_DIR}/.gomodcache"
BAR_WIDTH_RESERVE=24
BAR_WIDTH_MIN=10

SOCKET_PID=""

cleanup() {
	if [[ -n "${SOCKET_PID}" ]] && kill -0 "${SOCKET_PID}" 2>/dev/null; then
		kill "${SOCKET_PID}" 2>/dev/null || true
		wait "${SOCKET_PID}" 2>/dev/null || true
	fi
	rm -f "${SOCKET_PATH}"
	rm -rf "${TMP_DIR}"
}

trap cleanup EXIT

pause_between_steps() {
	if [[ -t 0 && -z "${NO_PAUSE:-}" ]]; then
		read -r -p $'\nPress Enter to continue to the next scenario... '
	else
		printf '\n'
	fi
}

section() {
	printf '\n== %s ==\n' "$1"
}

compute_terminal_width() {
	if [[ "${COLUMNS:-}" =~ ^[0-9]+$ ]] && (( COLUMNS > 0 )); then
		printf '%s\n' "${COLUMNS}"
		return 0
	fi

	if command -v tput >/dev/null 2>&1; then
		local cols
		cols="$(tput cols 2>/dev/null || true)"
		if [[ "${cols}" =~ ^[0-9]+$ ]] && (( cols > 0 )); then
			printf '%s\n' "${cols}"
			return 0
		fi
	fi

	printf '80\n'
}

compute_bar_width() {
	local terminal_width
	terminal_width="$(compute_terminal_width)"
	if (( terminal_width > BAR_WIDTH_RESERVE + BAR_WIDTH_MIN )); then
		printf '%s\n' "$((terminal_width - BAR_WIDTH_RESERVE))"
		return 0
	fi

	printf '%s\n' "${BAR_WIDTH_MIN}"
}

wait_for_socket() {
	local deadline
	deadline=$((SECONDS + 5))
	while [[ ! -S "${SOCKET_PATH}" ]]; do
		if (( SECONDS >= deadline )); then
			printf 'socket did not appear at %s\n' "${SOCKET_PATH}" >&2
			return 1
		fi
		sleep 0.1
	done
}

if [[ ! -t 1 ]]; then
	printf 'This smoke script needs a real terminal so you can watch the progress bar output.\n' >&2
	exit 1
fi

mkdir -p "${GOCACHE_DIR}" "${GOMODCACHE_DIR}"

BAR_WIDTH="$(compute_bar_width)"

section "Build"
printf 'Building binary at %s\n' "${BIN_PATH}"
(
	cd "${ROOT_DIR}"
	GOCACHE="${GOCACHE_DIR}" GOMODCACHE="${GOMODCACHE_DIR}" go build -o "${BIN_PATH}" .
)

section "Help Output"
printf 'Command: %s --help\n\n' "${BIN_PATH}"
"${BIN_PATH}" --help
pause_between_steps

section "Plain Line Mode"
printf 'Command: printf ... | %s --total 3 --detail --width %s\n\n' "${BIN_PATH}" "${BAR_WIDTH}"
(
	printf 'build\n'
	sleep 0.4
	printf 'test\n'
	sleep 0.4
	printf 'package\n'
) | "${BIN_PATH}" --total 3 --detail --width "${BAR_WIDTH}"
pause_between_steps

section "Control Protocol"
printf 'Command: control events piped into %s --detail --width %s\n\n' "${BIN_PATH}" "${BAR_WIDTH}"
(
	printf '@set-total 3\n'
	printf '@phase-name build\n'
	sleep 0.4
	printf '@tick\n'
	printf '@phase-name test\n'
	sleep 0.4
	printf '@tick\n'
	printf '@phase-name package\n'
	sleep 0.4
	printf '@tick\n'
) | "${BIN_PATH}" --style gradient-granular --bg-style shade-light --fps 30 --detail --width "${BAR_WIDTH}"
pause_between_steps

section "Custom Background"
printf 'Command: custom background demo with %s --bg-style custom --bg-char ░ --detail --width %s\n\n' "${BIN_PATH}" "${BAR_WIDTH}"
(
	printf '@set-total 3\n'
	printf '@phase-name build\n'
	sleep 0.5
	printf '@tick\n'
	printf '@phase-name test\n'
	sleep 0.5
	printf '@tick\n'
	printf '@phase-name package\n'
	sleep 0.5
	printf '@tick\n'
) | "${BIN_PATH}" --style gradient-granular --bg-style custom --bg-char '░' --detail --width "${BAR_WIDTH}"
pause_between_steps

section "Tint Animation: Pulse"
printf 'Command: pulse tint animation demo with %s --tint-animation pulse --detail --width %s\n\n' "${BIN_PATH}" "${BAR_WIDTH}"
(
	printf '@set-total 3\n'
	printf '@phase-name build\n'
	sleep 1.0
	printf '@tick\n'
	sleep 1.0
	printf '@phase-name test\n'
	sleep 1.0
	printf '@tick\n'
	sleep 1.0
	printf '@phase-name package\n'
	sleep 1.0
	printf '@tick\n'
) | "${BIN_PATH}" --style gradient-block --tint-animation pulse --detail --width "${BAR_WIDTH}"
pause_between_steps

section "Tint Animation: Shimmer"
printf 'Command: shimmer tint animation demo with %s --tint-animation shimmer --detail --width %s\n\n' "${BIN_PATH}" "${BAR_WIDTH}"
(
	printf '@set-total 3\n'
	printf '@phase-name build\n'
	sleep 1.0
	printf '@tick\n'
	sleep 1.0
	printf '@phase-name test\n'
	sleep 1.0
	printf '@tick\n'
	sleep 1.0
	printf '@phase-name package\n'
	sleep 1.0
	printf '@tick\n'
) | "${BIN_PATH}" --style gradient-block --tint-animation shimmer --detail --width "${BAR_WIDTH}"
pause_between_steps

section "Tint Animation: Cycle"
printf 'Command: cycle tint animation demo with %s --tint-animation cycle --detail --width %s\n\n' "${BIN_PATH}" "${BAR_WIDTH}"
(
	printf '@set-total 3\n'
	printf '@phase-name build\n'
	sleep 1.0
	printf '@tick\n'
	sleep 1.0
	printf '@phase-name test\n'
	sleep 1.0
	printf '@tick\n'
	sleep 1.0
	printf '@phase-name package\n'
	sleep 1.0
	printf '@tick\n'
) | "${BIN_PATH}" --style gradient-block --tint-animation cycle --detail --width "${BAR_WIDTH}"
pause_between_steps

section "JSON Protocol"
printf 'Command: JSON events piped into %s --input-mode json --detail --width %s\n\n' "${BIN_PATH}" "${BAR_WIDTH}"
(
	printf '{"type":"reset","total":3,"phases":["build","test","package"]}\n'
	sleep 0.4
	printf '{"type":"tick","amount":1}\n'
	printf '{"type":"phase","name":"test"}\n'
	sleep 0.4
	printf '{"type":"tick","amount":1}\n'
	printf '{"type":"phase","name":"package"}\n'
	sleep 0.4
	printf '{"type":"tick","amount":1}\n'
) | "${BIN_PATH}" --input-mode json --detail --width "${BAR_WIDTH}"
pause_between_steps

section "Phase File Bootstrap"
printf 'Command: numeric values piped into %s with --phase-file --detail --width %s\n\n' "${BIN_PATH}" "${BAR_WIDTH}"
(
	printf '1\n'
	sleep 0.4
	printf '2\n'
	sleep 0.4
	printf '3\n'
) | "${BIN_PATH}" --input-mode value --phase-file "${ROOT_DIR}/testdata/phase-files/phases.txt" --detail --width "${BAR_WIDTH}"
pause_between_steps

section "Socket Mode"
printf 'Command: %s --socket-path %s --total 3 --detail --width %s\n\n' "${BIN_PATH}" "${SOCKET_PATH}" "${BAR_WIDTH}"

if command -v nc >/dev/null 2>&1; then
	"${BIN_PATH}" --socket-path "${SOCKET_PATH}" --total 3 --detail --width "${BAR_WIDTH}" &
	SOCKET_PID=$!
	wait_for_socket

	(
		printf '@phase-name build\n'
		sleep 0.4
		printf '@tick\n'
		printf '@phase-name test\n'
		sleep 0.4
		printf '@tick\n'
		printf '@phase-name package\n'
		sleep 0.4
		printf '@tick\n'
	) | nc -U "${SOCKET_PATH}"

	sleep 1
	kill "${SOCKET_PID}" 2>/dev/null || true
	wait "${SOCKET_PID}" 2>/dev/null || true
	SOCKET_PID=""
	rm -f "${SOCKET_PATH}"
else
	printf 'Skipping socket scenario because `nc` is not installed.\n'
fi

pause_between_steps

section "Expected Failure"
printf 'Command: %s --socket-path relative.sock\n\n' "${BIN_PATH}"
set +e
"${BIN_PATH}" --socket-path relative.sock
status=$?
set -e
printf '\nExit status: %d (expected non-zero)\n' "${status}"

section "Done"
printf 'Smoke run finished.\n'
