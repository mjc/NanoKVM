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

The vendor RISC-V musl toolchain is Linux x86_64-only, matching the upstream backend build instructions. On Darwin or non-x86_64 Linux, the shell still provides the frontend and general development tools, but target server builds need an `x86_64-linux` builder.
