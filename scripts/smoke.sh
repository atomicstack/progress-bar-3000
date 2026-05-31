#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_PATH="${ROOT_DIR}/progress-bar-3000"
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

# Render a command array as a single, copy-pasteable line. Each argument is
# shell-quoted so what is printed matches what is executed exactly.
render_cmd() {
	local rendered
	rendered="$(printf '%q ' "$@")"
	printf '%s' "${rendered% }"
}

# announce <prefix> -- <cmd...>
#
# Prints "Command: <prefix><rendered cmd>\n\n". Use with an empty prefix for
# standalone invocations, or a pipeline prefix like "printf ... | " when the
# scenario pipes data into the binary.
announce() {
	local prefix="$1"
	shift
	if [[ "$1" != "--" ]]; then
		printf 'announce: expected -- separator, got %q\n' "$1" >&2
		return 2
	fi
	shift
	printf 'Command: %s%s\n\n' "${prefix}" "$(render_cmd "$@")"
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
if [[ -x "${BIN_PATH}" ]]; then
	printf 'Reusing existing binary at %s\n' "${BIN_PATH}"
else
	printf 'Building binary at %s (run `make build` to pre-build)\n' "${BIN_PATH}"
	(
		cd "${ROOT_DIR}"
		GOCACHE="${GOCACHE_DIR}" GOMODCACHE="${GOMODCACHE_DIR}" go build -o "${BIN_PATH}" .
	)
fi

section "Help Output"
HELP_CMD=("${BIN_PATH}" --help)
announce "" -- "${HELP_CMD[@]}"
"${HELP_CMD[@]}"
pause_between_steps

section "Plain Line Mode"
PLAIN_CMD=("${BIN_PATH}" --total 3 --detail --width "${BAR_WIDTH}")
announce "printf ... | " -- "${PLAIN_CMD[@]}"
(
	printf 'build\n'
	sleep 0.4
	printf 'test\n'
	sleep 0.4
	printf 'package\n'
) | "${PLAIN_CMD[@]}"
pause_between_steps

section "Minimal Bar"
MINIMAL_CMD=("${BIN_PATH}" --format '%p %{percent}' --width "${BAR_WIDTH}")
announce "printf '@...' ... | " -- "${MINIMAL_CMD[@]}"
(
	printf '@set-total 3\n'
	sleep 0.4
	printf '@tick\n'
	sleep 0.4
	printf '@tick\n'
	sleep 0.4
	printf '@tick\n'
) | "${MINIMAL_CMD[@]}"
pause_between_steps

section "Control Protocol"
CONTROL_CMD=("${BIN_PATH}" --style gradient-granular --bg-style shade-light --fps 30 --detail --width "${BAR_WIDTH}")
announce "printf '@...' ... | " -- "${CONTROL_CMD[@]}"
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
) | "${CONTROL_CMD[@]}"
pause_between_steps

section "Custom Background"
CUSTOM_BG_CMD=("${BIN_PATH}" --style gradient-granular --bg-style custom --bg-char '░' --detail --width "${BAR_WIDTH}")
announce "printf '@...' ... | " -- "${CUSTOM_BG_CMD[@]}"
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
) | "${CUSTOM_BG_CMD[@]}"
pause_between_steps

section "Tint Animation: Pulse"
PULSE_CMD=("${BIN_PATH}" --style gradient-block --tint-animation pulse --detail --width "${BAR_WIDTH}")
announce "printf '@...' ... | " -- "${PULSE_CMD[@]}"
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
) | "${PULSE_CMD[@]}"
pause_between_steps

section "Tint Animation: Shimmer"
SHIMMER_CMD=("${BIN_PATH}" --style gradient-block --tint-animation shimmer --detail --width "${BAR_WIDTH}")
announce "printf '@...' ... | " -- "${SHIMMER_CMD[@]}"
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
) | "${SHIMMER_CMD[@]}"
pause_between_steps

section "Tint Animation: Cycle"
CYCLE_CMD=("${BIN_PATH}" --style gradient-block --tint-animation cycle --detail --width "${BAR_WIDTH}")
announce "printf '@...' ... | " -- "${CYCLE_CMD[@]}"
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
) | "${CYCLE_CMD[@]}"
pause_between_steps

section "JSON Protocol"
JSON_CMD=("${BIN_PATH}" --input-mode json --detail --width "${BAR_WIDTH}")
announce "printf '{...}' ... | " -- "${JSON_CMD[@]}"
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
) | "${JSON_CMD[@]}"
pause_between_steps

section "Phase File Bootstrap"
PHASE_FILE_CMD=("${BIN_PATH}" --input-mode value --phase-file "${ROOT_DIR}/testdata/phase-files/phases.txt" --detail --width "${BAR_WIDTH}")
announce "printf '<n>' ... | " -- "${PHASE_FILE_CMD[@]}"
(
	printf '1\n'
	sleep 0.4
	printf '2\n'
	sleep 0.4
	printf '3\n'
) | "${PHASE_FILE_CMD[@]}"
pause_between_steps

section "Socket Mode"
SOCKET_CMD=("${BIN_PATH}" --socket-path "${SOCKET_PATH}" --total 3 --detail --width "${BAR_WIDTH}")
announce "" -- "${SOCKET_CMD[@]}"

if command -v nc >/dev/null 2>&1; then
	"${SOCKET_CMD[@]}" &
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
FAIL_CMD=("${BIN_PATH}" --socket-path relative.sock)
announce "" -- "${FAIL_CMD[@]}"
set +e
"${FAIL_CMD[@]}"
status=$?
set -e
printf '\nExit status: %d (expected non-zero)\n' "${status}"

section "Done"
printf 'Smoke run finished.\n'
