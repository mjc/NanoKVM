#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  scripts/deploy-nanokvm.sh deploy HOST [options]
  scripts/deploy-nanokvm.sh rollback HOST [options]
  scripts/deploy-nanokvm.sh list-backups HOST [options]

Commands:
  deploy        Build server/web artifacts and copy them to a running NanoKVM.
  rollback      Restore the latest backup, or one selected with --backup.
  list-backups  List remote backups.

Options:
  -u, --user USER             SSH user. Default: root
  -p, --port PORT             SSH port. Default: 22
  -i, --identity FILE         SSH identity file
      --remote-root PATH      Remote app root. Default: /kvmapp
      --backup-root PATH      Remote backup root. Default: REMOTE_ROOT/.deploy-backups
      --name NAME             Human-readable deploy name included in the backup ID
      --backup BACKUP_ID      Roll back a specific backup ID
      --no-build              Deploy existing local artifacts without rebuilding
      --keep N                Number of backups to retain. Default: 5
  -h, --help                  Show this help

Examples:
  scripts/deploy-nanokvm.sh deploy 192.168.1.42
  scripts/deploy-nanokvm.sh deploy 192.168.1.42 --name usb-state-fix
  scripts/deploy-nanokvm.sh rollback nanokvm.local --backup 20260605-153022-usb-state-fix
  scripts/deploy-nanokvm.sh list-backups root@192.168.1.42
EOF
}

die() {
  echo "error: $*" >&2
  exit 1
}

command_exists() {
  command -v "$1" >/dev/null 2>&1
}

require_command() {
  command_exists "$1" || die "required command '$1' not found"
}

repo_root() {
  git rev-parse --show-toplevel
}

quote_remote() {
  printf "%s" "$1" | sed "s/'/'\\\\''/g; s/^/'/; s/$/'/"
}

command="${1:-}"
case "$command" in
  deploy | rollback | list-backups)
    shift
    ;;
  -h | --help | "")
    usage
    exit 0
    ;;
  *)
    usage >&2
    exit 2
    ;;
esac

host="${1:-}"
[ -n "$host" ] || die "HOST is required"
shift

ssh_user="root"
ssh_port="22"
identity_file=""
remote_root="/kvmapp"
backup_root=""
deploy_name="deploy"
backup_name=""
build=1
keep=5

while [ "$#" -gt 0 ]; do
  case "$1" in
    -u | --user)
      ssh_user="${2:-}"
      [ -n "$ssh_user" ] || die "$1 requires a value"
      shift 2
      ;;
    -p | --port)
      ssh_port="${2:-}"
      [ -n "$ssh_port" ] || die "$1 requires a value"
      shift 2
      ;;
    -i | --identity)
      identity_file="${2:-}"
      [ -n "$identity_file" ] || die "$1 requires a value"
      shift 2
      ;;
    --remote-root)
      remote_root="${2:-}"
      [ -n "$remote_root" ] || die "$1 requires a value"
      shift 2
      ;;
    --backup-root)
      backup_root="${2:-}"
      [ -n "$backup_root" ] || die "$1 requires a value"
      shift 2
      ;;
    --name)
      deploy_name="${2:-}"
      [ -n "$deploy_name" ] || die "$1 requires a value"
      shift 2
      ;;
    --backup)
      backup_name="${2:-}"
      [ -n "$backup_name" ] || die "$1 requires a value"
      shift 2
      ;;
    --no-build)
      build=0
      shift
      ;;
    --keep)
      keep="${2:-}"
      [ -n "$keep" ] || die "$1 requires a value"
      shift 2
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

case "$keep" in
  '' | *[!0-9]*)
    die "--keep must be a positive integer"
    ;;
esac
[ "$keep" -gt 0 ] || die "--keep must be greater than zero"

deploy_label="$(printf "%s" "$deploy_name" | tr -cs 'A-Za-z0-9._-' '-' | sed 's/^-//; s/-$//')"
[ -n "$deploy_label" ] || deploy_label="deploy"

if [ -z "$backup_root" ]; then
  backup_root="${remote_root%/}/.deploy-backups"
fi

if [[ "$host" == *@* ]]; then
  ssh_target="$host"
else
  ssh_target="$ssh_user@$host"
fi

ssh_args=(-p "$ssh_port")
scp_args=(-P "$ssh_port")
if [ -n "$identity_file" ]; then
  ssh_args+=(-i "$identity_file")
  scp_args+=(-i "$identity_file")
fi

remote_server_dir="${remote_root%/}/server"
remote_server_bin="$remote_server_dir/NanoKVM-Server"
remote_web_dir="$remote_server_dir/web"
remote_tmp="/tmp/nanokvm-deploy-$$.tar.gz"

run_remote() {
  ssh "${ssh_args[@]}" "$ssh_target" "$@"
}

build_artifacts() {
  echo "Building NanoKVM server and web artifacts..."
  nix develop "$root" -c nanokvm-build-server
  nix develop "$root" -c nanokvm-build-web
}

make_payload() {
  local payload_dir tarball
  payload_dir="$(mktemp -d)"
  tarball="$(mktemp -t nanokvm-deploy.XXXXXX.tar.gz)"

  mkdir -p "$payload_dir/web"
  cp "$root/server/NanoKVM-Server" "$payload_dir/NanoKVM-Server"
  cp -a "$root/web/dist/." "$payload_dir/web/"
  chmod 0755 "$payload_dir/NanoKVM-Server"

  tar -C "$payload_dir" -czf "$tarball" NanoKVM-Server web
  rm -rf "$payload_dir"
  printf "%s\n" "$tarball"
}

ensure_local_artifacts() {
  [ -f "$root/server/NanoKVM-Server" ] || die "missing server/NanoKVM-Server; run deploy without --no-build first"
  [ -d "$root/web/dist" ] || die "missing web/dist; run deploy without --no-build first"
}

deploy() {
  if [ "$build" -eq 1 ]; then
    build_artifacts
  fi
  ensure_local_artifacts

  local tarball stamp backup_id q_remote_root q_backup_root q_remote_server_dir q_remote_server_bin q_remote_web_dir q_remote_tmp q_deploy_name
  tarball="$(make_payload)"
  stamp="$(date -u +%Y%m%d-%H%M%S)"
  backup_id="$stamp-$deploy_label"

  echo "Uploading artifacts to $ssh_target..."
  scp "${scp_args[@]}" "$tarball" "$ssh_target:$remote_tmp"
  rm -f "$tarball"

  q_remote_root="$(quote_remote "$remote_root")"
  q_backup_root="$(quote_remote "$backup_root")"
  q_remote_server_dir="$(quote_remote "$remote_server_dir")"
  q_remote_server_bin="$(quote_remote "$remote_server_bin")"
  q_remote_web_dir="$(quote_remote "$remote_web_dir")"
  q_remote_tmp="$(quote_remote "$remote_tmp")"
  q_deploy_name="$(quote_remote "$deploy_name")"

  echo "Backing up current remote artifacts as $backup_id and deploying..."
  run_remote sh -s -- "$backup_id" "$keep" <<EOF
set -eu
backup_id="\$1"
keep="\$2"
remote_root=$q_remote_root
backup_root=$q_backup_root
server_dir=$q_remote_server_dir
server_bin=$q_remote_server_bin
web_dir=$q_remote_web_dir
remote_tmp=$q_remote_tmp
deploy_name=$q_deploy_name
backup_dir="\$backup_root/\$backup_id"

mkdir -p "\$backup_dir" "\$server_dir"

server_exists=0
web_exists=0
if [ -e "\$server_bin" ]; then
  cp -a "\$server_bin" "\$backup_dir/NanoKVM-Server"
  server_exists=1
fi
if [ -e "\$web_dir" ]; then
  cp -a "\$web_dir" "\$backup_dir/web"
  web_exists=1
fi
{
  echo "backup_id=\$backup_id"
  echo "deploy_name=\$deploy_name"
  echo "created_utc=$stamp"
  echo "server=\$server_exists"
  echo "web=\$web_exists"
} > "\$backup_dir/manifest"

rm -f "\$server_bin"
rm -rf "\$web_dir"
tar -xzf "\$remote_tmp" -C "\$server_dir"
chmod 0755 "\$server_bin"
rm -f "\$remote_tmp"

if [ -x /etc/init.d/S95nanokvm ]; then
  /etc/init.d/S95nanokvm restart
fi

if [ -d "\$backup_root" ]; then
  ls -1 "\$backup_root" | sort -r | sed -n "\$((keep + 1)),\$p" | while IFS= read -r old; do
    [ -n "\$old" ] && rm -rf "\$backup_root/\$old"
  done
fi
EOF

  echo "Deploy complete. Backup: $backup_id"
}

list_backups() {
  local q_backup_root
  q_backup_root="$(quote_remote "$backup_root")"
  run_remote sh -s <<EOF
set -eu
backup_root=$q_backup_root
if [ ! -d "\$backup_root" ]; then
  exit 0
fi
ls -1 "\$backup_root" | sort -r | while IFS= read -r backup; do
  [ -n "\$backup" ] || continue
  manifest="\$backup_root/\$backup/manifest"
  if [ -f "\$manifest" ]; then
    deploy_name="\$(sed -n 's/^deploy_name=//p' "\$manifest" | sed -n '1p')"
    created_utc="\$(sed -n 's/^created_utc=//p' "\$manifest" | sed -n '1p')"
    printf '%s\t%s\t%s\n' "\$backup" "\$created_utc" "\$deploy_name"
  else
    printf '%s\n' "\$backup"
  fi
done
EOF
}

rollback() {
  local q_backup_root q_remote_server_dir q_remote_server_bin q_remote_web_dir q_backup_name
  q_backup_root="$(quote_remote "$backup_root")"
  q_remote_server_dir="$(quote_remote "$remote_server_dir")"
  q_remote_server_bin="$(quote_remote "$remote_server_bin")"
  q_remote_web_dir="$(quote_remote "$remote_web_dir")"
  q_backup_name="$(quote_remote "$backup_name")"

  echo "Rolling back $ssh_target..."
  run_remote sh -s <<EOF
set -eu
backup_root=$q_backup_root
server_dir=$q_remote_server_dir
server_bin=$q_remote_server_bin
web_dir=$q_remote_web_dir
backup_name=$q_backup_name

if [ ! -d "\$backup_root" ]; then
  echo "No backups found at \$backup_root" >&2
  exit 1
fi

if [ -z "\$backup_name" ]; then
  backup_name="\$(ls -1 "\$backup_root" | sort -r | sed -n '1p')"
fi
[ -n "\$backup_name" ] || {
  echo "No backups found at \$backup_root" >&2
  exit 1
}

backup_dir="\$backup_root/\$backup_name"
[ -d "\$backup_dir" ] || {
  echo "Backup not found: \$backup_name" >&2
  exit 1
}

server_was_present=1
web_was_present=1
if [ -f "\$backup_dir/manifest" ]; then
  server_was_present="\$(sed -n 's/^server=//p' "\$backup_dir/manifest" | sed -n '1p')"
  web_was_present="\$(sed -n 's/^web=//p' "\$backup_dir/manifest" | sed -n '1p')"
fi

mkdir -p "\$server_dir"

if [ "\$server_was_present" = "1" ] && [ -f "\$backup_dir/NanoKVM-Server" ]; then
  cp -a "\$backup_dir/NanoKVM-Server" "\$server_bin"
  chmod 0755 "\$server_bin"
else
  rm -f "\$server_bin"
fi

if [ "\$web_was_present" = "1" ] && [ -d "\$backup_dir/web" ]; then
  rm -rf "\$web_dir"
  cp -a "\$backup_dir/web" "\$web_dir"
else
  rm -rf "\$web_dir"
fi

if [ -x /etc/init.d/S95nanokvm ]; then
  /etc/init.d/S95nanokvm restart
fi

echo "Rolled back to \$backup_name"
EOF
}

require_command git
require_command nix
require_command ssh
require_command scp
require_command tar

root="$(repo_root)"
cd "$root"

case "$command" in
  deploy)
    deploy
    ;;
  rollback)
    rollback
    ;;
  list-backups)
    list_backups
    ;;
esac
