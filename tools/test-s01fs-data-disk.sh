#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
script="$repo_root/kvmapp/system/init.d/S01fs"

make_stubs() {
	local bin="$1"

	cat >"$bin/mount" <<'EOF'
#!/bin/sh
printf 'mount %s\n' "$*" >> "$NANOKVM_TEST_LOG"
exit 0
EOF

	cat >"$bin/resize2fs" <<'EOF'
#!/bin/sh
printf 'resize2fs %s\n' "$*" >> "$NANOKVM_TEST_LOG"
exit 0
EOF

	cat >"$bin/sleep" <<'EOF'
#!/bin/sh
exit 0
EOF

	cat >"$bin/parted" <<'EOF'
#!/bin/sh
printf 'parted %s\n' "$*" >> "$NANOKVM_TEST_LOG"
case " $* " in
	*" mkpart primary 8193MB 100% "*) : > "$NANOKVM_DATA_PART" ;;
esac
exit 0
EOF

cat >"$bin/blkid" <<'EOF'
#!/bin/sh
printf 'blkid %s\n' "$*" >> "$NANOKVM_TEST_LOG"
if [ -e "$1.hasfs" ]; then
	printf '%s: UUID="test" TYPE="exfat"\n' "$1"
fi
exit 0
EOF

	cat >"$bin/mkfs.exfat" <<'EOF'
#!/bin/sh
printf 'mkfs.exfat %s\n' "$*" >> "$NANOKVM_TEST_LOG"
if [ "${NANOKVM_MKFS_FAIL:-0}" = "1" ]; then
	exit 1
fi
: > "$1.hasfs"
exit 0
EOF

	chmod +x "$bin/mount" "$bin/resize2fs" "$bin/sleep" "$bin/parted" "$bin/blkid" "$bin/mkfs.exfat"
}

setup_case() {
	tmpdir="$(mktemp -d)"
	mkdir -p "$tmpdir/bin" "$tmpdir/boot" "$tmpdir/data" "$tmpdir/dev" "$tmpdir/etc"
	make_stubs "$tmpdir/bin"

	export PATH="$tmpdir/bin:$PATH"
	export NANOKVM_TEST_LOG="$tmpdir/log"
	export NANOKVM_DISK="$tmpdir/dev/mmcblk0"
	export NANOKVM_BOOT_PART="$tmpdir/dev/mmcblk0p1"
	export NANOKVM_ROOT_PART="$tmpdir/dev/mmcblk0p2"
	export NANOKVM_DATA_PART="$tmpdir/dev/mmcblk0p3"
	export NANOKVM_BOOT_DIR="$tmpdir/boot"
	export NANOKVM_DATA_DIR="$tmpdir/data"
	export NANOKVM_DISK0_MARKER="$tmpdir/etc/kvm.disk0"

	: > "$NANOKVM_DISK"
	: > "$NANOKVM_BOOT_PART"
	: > "$NANOKVM_ROOT_PART"
	: > "$NANOKVM_BOOT_DIR/usb.disk0"
	: > "$NANOKVM_TEST_LOG"
}

teardown_case() {
	rm -rf "$tmpdir"
}

assert_log_contains() {
	local pattern="$1"
	if ! grep -Fq "$pattern" "$NANOKVM_TEST_LOG"; then
		echo "expected log to contain: $pattern" >&2
		cat "$NANOKVM_TEST_LOG" >&2
		exit 1
	fi
}

assert_log_not_contains() {
	local pattern="$1"
	if grep -Fq "$pattern" "$NANOKVM_TEST_LOG"; then
		echo "expected log not to contain: $pattern" >&2
		cat "$NANOKVM_TEST_LOG" >&2
		exit 1
	fi
}

test_formats_existing_unformatted_partition_with_stale_marker() {
	setup_case
	trap teardown_case RETURN
	: > "$NANOKVM_DATA_PART"
	: > "$NANOKVM_DISK0_MARKER"

	"$script" start >/dev/null

	[ -e "$NANOKVM_DATA_PART.hasfs" ]
	[ -e "$NANOKVM_DISK0_MARKER" ]
	assert_log_contains "blkid $NANOKVM_DATA_PART"
	assert_log_contains "mkfs.exfat $NANOKVM_DATA_PART"
}

test_removes_marker_when_format_fails() {
	setup_case
	trap teardown_case RETURN
	: > "$NANOKVM_DATA_PART"
	: > "$NANOKVM_DISK0_MARKER"
	export NANOKVM_MKFS_FAIL=1

	"$script" start >/dev/null

	[ ! -e "$NANOKVM_DISK0_MARKER" ]
	assert_log_contains "mkfs.exfat $NANOKVM_DATA_PART"
	unset NANOKVM_MKFS_FAIL
}

test_does_not_format_unknown_non_empty_partition() {
	setup_case
	trap teardown_case RETURN
	printf 'user data' > "$NANOKVM_DATA_PART"
	: > "$NANOKVM_DISK0_MARKER"

	"$script" start >/dev/null

	[ ! -e "$NANOKVM_DISK0_MARKER" ]
	assert_log_contains "blkid $NANOKVM_DATA_PART"
	assert_log_not_contains "mkfs.exfat $NANOKVM_DATA_PART"
}

test_skips_format_when_partition_has_filesystem() {
	setup_case
	trap teardown_case RETURN
	: > "$NANOKVM_DATA_PART"
	: > "$NANOKVM_DATA_PART.hasfs"

	"$script" start >/dev/null

	[ -e "$NANOKVM_DISK0_MARKER" ]
	assert_log_contains "blkid $NANOKVM_DATA_PART"
	assert_log_not_contains "mkfs.exfat $NANOKVM_DATA_PART"
}

test_creates_and_formats_missing_partition() {
	setup_case
	trap teardown_case RETURN

	"$script" start >/dev/null

	[ -e "$NANOKVM_DATA_PART" ]
	[ -e "$NANOKVM_DATA_PART.hasfs" ]
	[ -e "$NANOKVM_DISK0_MARKER" ]
	assert_log_contains "parted -s $NANOKVM_DISK mkpart primary 8193MB 100%"
	assert_log_contains "mkfs.exfat $NANOKVM_DATA_PART"
}

test_formats_existing_unformatted_partition_with_stale_marker
test_removes_marker_when_format_fails
test_does_not_format_unknown_non_empty_partition
test_skips_format_when_partition_has_filesystem
test_creates_and_formats_missing_partition

echo "S01fs data disk tests passed"
