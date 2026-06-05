#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  scripts/build-nanokvm-image.sh [options]

Build a LicheeRV Nano SD-card image and overlay this repository's NanoKVM
application tree into /kvmapp.

Options:
      --sdk-dir PATH       SDK checkout directory. Default: ~/src/licheerv-nano
      --sdk-url URL        SDK repository URL. Default: https://github.com/sipeed/LicheeRV-Nano-Build
      --sdk-ref REF        SDK branch/tag/commit to checkout after clone/fetch. Default: main
      --maixcdk-dir PATH   MaixCDK checkout directory. Default: ~/src/MaixCDK
      --board BOARD        SDK defconfig board. Default: sg2002_licheervnano_sd
      --output-dir PATH    Output directory. Default: dist/images
      --name NAME          Output image label. Default: nanokvm
      --skip-sdk-build     Reuse the newest SDK-built image found under SDK install/
      --skip-support-build Reuse existing kvmapp/kvm_system/kvm_system
      --no-build-app       Reuse existing server/NanoKVM-Server and web/dist
  -h, --help              Show this help

Examples:
  scripts/build-nanokvm-image.sh
  scripts/build-nanokvm-image.sh --name usb-state-fix
  scripts/build-nanokvm-image.sh --skip-sdk-build --no-build-app
EOF
}

die() {
  echo "error: $*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || die "required command '$1' not found"
}

repo_root() {
  git rev-parse --show-toplevel
}

abspath() {
  case "$1" in
    /*)
      printf "%s\n" "$1"
      ;;
    *)
      printf "%s/%s\n" "$(pwd)" "$1"
      ;;
  esac
}

sdk_dir="$HOME/src/licheerv-nano"
sdk_url="https://github.com/sipeed/LicheeRV-Nano-Build"
sdk_ref="main"
maixcdk_dir="$HOME/src/MaixCDK"
board="sg2002_licheervnano_sd"
output_dir="dist/images"
image_name="nanokvm"
build_sdk=1
build_support=1
build_app=1

while [ "$#" -gt 0 ]; do
  case "$1" in
    --sdk-dir)
      sdk_dir="${2:-}"
      [ -n "$sdk_dir" ] || die "$1 requires a value"
      shift 2
      ;;
    --sdk-url)
      sdk_url="${2:-}"
      [ -n "$sdk_url" ] || die "$1 requires a value"
      shift 2
      ;;
    --sdk-ref)
      sdk_ref="${2:-}"
      [ -n "$sdk_ref" ] || die "$1 requires a value"
      shift 2
      ;;
    --maixcdk-dir)
      maixcdk_dir="${2:-}"
      [ -n "$maixcdk_dir" ] || die "$1 requires a value"
      shift 2
      ;;
    --board)
      board="${2:-}"
      [ -n "$board" ] || die "$1 requires a value"
      shift 2
      ;;
    --output-dir)
      output_dir="${2:-}"
      [ -n "$output_dir" ] || die "$1 requires a value"
      shift 2
      ;;
    --name)
      image_name="${2:-}"
      [ -n "$image_name" ] || die "$1 requires a value"
      shift 2
      ;;
    --skip-sdk-build)
      build_sdk=0
      shift
      ;;
    --skip-support-build)
      build_support=0
      shift
      ;;
    --no-build-app)
      build_app=0
      shift
      ;;
    -h | --help)
      usage
      exit 0
      ;;
    *)
      die "unknown option: $1"
      ;;
  esac
done

case "$(uname -s)" in
  Linux)
    ;;
  *)
    die "full image building and ext4 image patching require Linux"
    ;;
esac

safe_name="$(printf "%s" "$image_name" | tr -cs 'A-Za-z0-9._-' '-' | sed 's/^-//; s/-$//')"
[ -n "$safe_name" ] || safe_name="nanokvm"

require_command git
require_command nix
require_command rsync
require_command find
require_command sort
require_command tail

root="$(repo_root)"
sdk_dir="$(abspath "$sdk_dir")"
maixcdk_dir="$(abspath "$maixcdk_dir")"
output_dir="$(abspath "$output_dir")"
work_dir="$root/.image-work"
app_dir="$work_dir/kvmapp"

ensure_sdk() {
  if [ ! -d "$sdk_dir/.git" ]; then
    mkdir -p "$(dirname "$sdk_dir")"
    git clone --depth=1 --branch "$sdk_ref" "$sdk_url" "$sdk_dir"
  else
    git -C "$sdk_dir" fetch --tags origin "$sdk_ref" || git -C "$sdk_dir" fetch --tags origin
    git -C "$sdk_dir" checkout "$sdk_ref"
  fi

  if [ ! -d "$sdk_dir/host-tools/.git" ]; then
    git clone --depth=1 https://github.com/sophgo/host-tools "$sdk_dir/host-tools"
  fi
}

ensure_maixcdk() {
  if [ ! -d "$maixcdk_dir/.git" ]; then
    mkdir -p "$(dirname "$maixcdk_dir")"
    git clone https://github.com/Sipeed/MaixCDK "$maixcdk_dir"
  else
    git -C "$maixcdk_dir" fetch --tags origin
  fi

  if [ ! -x "$maixcdk_dir/bin/maixcdk" ]; then
    (
      cd "$maixcdk_dir"
      python3 -m venv .
      # shellcheck disable=SC1091
      source bin/activate
      pip install -U -r requirements.txt
    )
  fi
}

build_sdk_image() {
  echo "Building SDK image in $sdk_dir for $board..."
  (
    cd "$sdk_dir"
    # cvisetup.sh defines defconfig and build_all.
    # shellcheck disable=SC1091
    source build/cvisetup.sh
    defconfig "$board"
    build_all
  )
}

build_support_artifacts() {
  local fake_home
  ensure_maixcdk

  echo "Building NanoKVM support artifacts with MaixCDK..."
  fake_home="$work_dir/home"
  rm -rf "$fake_home"
  mkdir -p "$fake_home"
  ln -s "$maixcdk_dir" "$fake_home/MaixCDK"
  ln -s "$root" "$fake_home/NanoKVM"

  (
    export HOME="$fake_home"
    # shellcheck disable=SC1091
    source "$fake_home/MaixCDK/bin/activate"
    cd "$root/support/sg2002"
    ./build kvm_system
    ./build kvm_system add_to_kvmapp
  )
}

build_app_artifacts() {
  echo "Building NanoKVM application artifacts..."
  nix develop "$root" -c nanokvm-build-server
  nix develop "$root" -c nanokvm-build-web
}

ensure_app_artifacts() {
  [ -f "$root/server/NanoKVM-Server" ] || die "missing server/NanoKVM-Server; rerun without --no-build-app"
  [ -d "$root/web/dist" ] || die "missing web/dist; rerun without --no-build-app"
  [ -f "$root/kvmapp/kvm_system/kvm_system" ] || die "missing kvmapp/kvm_system/kvm_system; rerun without --skip-support-build"
}

assemble_kvmapp() {
  echo "Assembling kvmapp payload..."
  rm -rf "$app_dir"
  mkdir -p "$app_dir/server"

  rsync -a --delete "$root/kvmapp/" "$app_dir/"
  install -m 0755 "$root/server/NanoKVM-Server" "$app_dir/server/NanoKVM-Server"
  rsync -a --delete "$root/server/dl_lib/" "$app_dir/server/dl_lib/"
  rsync -a --delete "$root/web/dist/" "$app_dir/server/web/"

  if [ ! -f "$app_dir/version" ]; then
    printf "%s-%s\n" "$safe_name" "$(git -C "$root" rev-parse --short HEAD)" > "$app_dir/version"
  fi

  find "$app_dir" -type d -exec chmod 0755 {} +
  find "$app_dir" -type f -exec chmod 0755 {} +
}

find_sdk_image() {
  local newest
  newest="$(
    find "$sdk_dir/install" -type f -path '*/images/*.img' -printf '%T@ %p\n' 2>/dev/null \
      | sort -n \
      | tail -n 1 \
      | sed 's/^[^ ]* //'
  )"
  [ -n "$newest" ] || die "no SDK image found under $sdk_dir/install; run without --skip-sdk-build"
  printf "%s\n" "$newest"
}

unmount_image() {
  local mount_dir="$1"
  if mountpoint -q "$mount_dir" 2>/dev/null; then
    if command -v fusermount >/dev/null 2>&1; then
      fusermount -u "$mount_dir" || true
    elif command -v fusermount3 >/dev/null 2>&1; then
      fusermount3 -u "$mount_dir" || true
    else
      umount "$mount_dir" || true
    fi
  fi
}

patch_image() {
  local source_image output_image mount_dir
  source_image="$(find_sdk_image)"
  mkdir -p "$output_dir"

  output_image="$output_dir/${safe_name}-$(date -u +%Y%m%d-%H%M%S).img"
  cp "$source_image" "$output_image"

  mount_dir="$(mktemp -d)"
  trap 'unmount_image "$mount_dir"; rmdir "$mount_dir" 2>/dev/null || true' EXIT

  echo "Patching image rootfs: $output_image"
  "$sdk_dir/host/mount_ext4.sh" "$output_image" "$mount_dir"

  rm -rf "$mount_dir/kvmapp"
  rsync -a --delete "$app_dir/" "$mount_dir/kvmapp/"

  if [ -d "$mount_dir/etc/init.d" ]; then
    rsync -a "$app_dir/system/init.d/" "$mount_dir/etc/init.d/"
  fi

  sync
  unmount_image "$mount_dir"
  rmdir "$mount_dir" 2>/dev/null || true
  trap - EXIT

  echo "Built NanoKVM image: $output_image"
}

ensure_sdk

if [ "$build_support" -eq 1 ]; then
  build_support_artifacts
fi

if [ "$build_app" -eq 1 ]; then
  build_app_artifacts
fi
ensure_app_artifacts
assemble_kvmapp

if [ "$build_sdk" -eq 1 ]; then
  build_sdk_image
fi
patch_image
