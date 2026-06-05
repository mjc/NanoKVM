# Nix Development Environment

This repository has a Nix flake for local development and target builds.

Enter the shell from the repository root:

```sh
nix develop
```

The shell provides:

- Go for the backend service.
- Node.js and pnpm for the web UI.
- CMake, Ninja, Make, Autotools, Python, and patchelf for support tooling.
- On `x86_64-linux`, the pinned Sophon host-tools archive with `riscv64-unknown-linux-musl-gcc`.

Common build commands:

```sh
nanokvm-build-server
nanokvm-build-web
```

`nanokvm-build-server` builds `server/NanoKVM-Server` for `linux/riscv64` with the C906 flags used by the upstream Docker build, then patches its RPATH to find `server/dl_lib`.

`nanokvm-build-web` runs `pnpm install --frozen-lockfile` and `pnpm build` under `web/`.

Deploy to a running NanoKVM over SSH:

```sh
scripts/deploy-nanokvm.sh deploy 192.168.1.42 --name my-change
```

The deploy script builds the server and web artifacts, backs up the current remote `NanoKVM-Server` and `web/` under `/kvmapp/.deploy-backups/<timestamp>-<name>/`, keeps the newest five backups, deploys the new artifacts to `/kvmapp/server/`, and restarts `/etc/init.d/S95nanokvm` when present.

Rollback and backup inspection:

```sh
scripts/deploy-nanokvm.sh list-backups 192.168.1.42
scripts/deploy-nanokvm.sh rollback 192.168.1.42
scripts/deploy-nanokvm.sh rollback 192.168.1.42 --backup 20260605-153022-my-change
```

Build a full SD-card image:

```sh
scripts/build-nanokvm-image.sh --name my-change
```

The image script uses `~/src/licheerv-nano` by default for the Sipeed `LicheeRV-Nano-Build` SDK checkout, cloning it if needed. It also uses `~/src/MaixCDK` to build the required `kvm_system` support binary. It builds the `sg2002_licheervnano_sd` Buildroot image, assembles this repository's `kvmapp` with the freshly built support, server, and web artifacts, mounts the SDK image's rootfs partition with `host/mount_ext4.sh`, overlays `/kvmapp`, and writes the patched image under `dist/images/`.

Useful image build options:

```sh
scripts/build-nanokvm-image.sh --skip-sdk-build
scripts/build-nanokvm-image.sh --skip-support-build
scripts/build-nanokvm-image.sh --no-build-app
scripts/build-nanokvm-image.sh --sdk-dir ~/src/licheerv-nano
scripts/build-nanokvm-image.sh --maixcdk-dir ~/src/MaixCDK
```

The vendor RISC-V musl toolchain is Linux x86_64-only, matching the upstream backend build instructions. On Darwin or non-x86_64 Linux, the shell still provides the frontend and general development tools, but target server builds need an `x86_64-linux` builder.
