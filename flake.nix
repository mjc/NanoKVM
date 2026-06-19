{
  description = "NanoKVM development shell";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    rust-overlay = {
      url = "github:oxalica/rust-overlay";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs = {
    nixpkgs,
    flake-utils,
    rust-overlay,
    ...
  }:
    flake-utils.lib.eachDefaultSystem (system: let
      overlays = [(import rust-overlay)];
      pkgs = import nixpkgs {inherit system overlays;};
      lib = pkgs.lib;

      rustToolchain = pkgs.rust-bin.stable."1.96.0".default.override {
        extensions = ["rust-src" "rust-analyzer"];
        targets = [
          "riscv64gc-unknown-linux-gnu"
          "riscv64gc-unknown-linux-musl"
        ];
      };
      go = pkgs.go_1_25 or pkgs.go_1_24 or pkgs.go;
      nodejs = pkgs.nodejs_24 or pkgs.nodejs_22 or pkgs.nodejs;

      hostTools =
        if system == "x86_64-linux"
        then
          pkgs.stdenvNoCC.mkDerivation {
            pname = "sophon-host-tools";
            version = "2023-03-07";

            src = pkgs.fetchurl {
              url = "https://sophon-file.sophon.cn/sophon-prod-s3/drive/23/03/07/16/host-tools.tar.gz";
              hash = "sha256-/5pY6OGSsg6kLh1ynELSIZUjIJcGyz8M8TRYL2xw+AU=";
            };

            nativeBuildInputs = [pkgs.autoPatchelfHook];
            buildInputs = [
              pkgs.stdenv.cc.cc.lib
              pkgs.zlib
              pkgs.ncurses5
            ];

            dontBuild = true;
            dontStrip = true;

            installPhase = ''
              runHook preInstall
              mkdir -p "$out/host-tools"
              cp -a . "$out/host-tools/"
              runHook postInstall
            '';
          }
        else null;

      buildServer = pkgs.writeShellApplication {
        name = "nanokvm-build-server";
        runtimeInputs =
          [
            go
            pkgs.git
            pkgs.patchelf
          ]
          ++ lib.optionals (hostTools != null) [hostTools];
        text =
          if hostTools != null
          then ''
            repo_root="$(git rev-parse --show-toplevel)"
            cd "$repo_root/server"

            export PATH="${hostTools}/host-tools/gcc/riscv64-linux-musl-x86_64/bin:$PATH"
            export CGO_ENABLED=1
            export GOOS="''${NANOKVM_GOOS:-linux}"
            export GOARCH="''${NANOKVM_GOARCH:-riscv64}"
            export GOEXPERIMENT="''${GOEXPERIMENT:-boringcrypto}"
            export CC="''${NANOKVM_CC:-riscv64-unknown-linux-musl-gcc}"
            export CGO_CFLAGS="''${NANOKVM_CGO_CFLAGS:--mcpu=c906fdv -march=rv64imafdcv0p7xthead -mcmodel=medany -mabi=lp64d}"

            go mod download
            go build -o NanoKVM-Server
            go build -tags legacy_webrtc -o NanoKVM-Server-legacy
            for bin in NanoKVM-Server NanoKVM-Server-legacy; do
              patchelf --add-rpath "\$ORIGIN/dl_lib" "$bin"
            done
          ''
          else ''
            echo "nanokvm-build-server requires the x86_64-linux Sophon RISC-V musl toolchain." >&2
            exit 1
          '';
      };

      buildWebrtcRs = pkgs.writeShellApplication {
        name = "nanokvm-build-webrtc-rs";
        runtimeInputs =
          [
            rustToolchain
            pkgs.git
            pkgs.pkg-config
            pkgs.clang
            pkgs.libclang
            pkgs.lld
            pkgs.patchelf
          ]
          ++ lib.optionals (hostTools != null) [hostTools];
        text =
          if hostTools != null
          then ''
            repo_root="$(git rev-parse --show-toplevel)"
            cd "$repo_root/server/webrtc-rs"

            export PATH="${hostTools}/host-tools/gcc/riscv64-linux-musl-x86_64/bin:$PATH"
            export CC_riscv64gc_unknown_linux_musl="''${NANOKVM_RUST_CC:-riscv64-unknown-linux-musl-gcc}"
            linker_shim="$(mktemp -d)"
            lld_path="$(command -v ld.lld)"
            ln -sf "$lld_path" "$linker_shim/ld"
            ln -sf "$lld_path" "$linker_shim/riscv64-unknown-linux-musl-ld"
            ln -sf "$lld_path" "$linker_shim/ld.lld"
            ln -sf "$lld_path" "$linker_shim/riscv64-unknown-linux-musl-ld.lld"

            export CARGO_TARGET_RISCV64GC_UNKNOWN_LINUX_MUSL_LINKER="''${NANOKVM_RUST_LINKER:-riscv64-unknown-linux-musl-gcc}"
            export RUSTFLAGS="''${NANOKVM_RUSTFLAGS:--C link-arg=-B$linker_shim -C link-arg=-fuse-ld=lld -C link-arg=-Wl,-rpath,\$ORIGIN/dl_lib}"

            cargo build --release --target riscv64gc-unknown-linux-musl
            cp "target/riscv64gc-unknown-linux-musl/release/nanokvm-webrtc-rs" "$repo_root/server/nanokvm-webrtc-rs"
            patchelf --add-rpath "\$ORIGIN/dl_lib" "$repo_root/server/nanokvm-webrtc-rs" || true
          ''
          else ''
            echo "nanokvm-build-webrtc-rs requires the x86_64-linux Sophon RISC-V musl toolchain." >&2
            exit 1
          '';
      };

      buildWeb = pkgs.writeShellApplication {
        name = "nanokvm-build-web";
        runtimeInputs = [
          pkgs.git
          nodejs
          pkgs.pnpm
        ];
        text = ''
          repo_root="$(git rev-parse --show-toplevel)"
          cd "$repo_root/web"
          pnpm install --frozen-lockfile
          pnpm build
        '';
      };
    in {
      devShells.default = pkgs.mkShell {
        packages =
          [
            go
            nodejs
            pkgs.pnpm
            pkgs.pkg-config
            pkgs.cmake
            pkgs.ninja
            pkgs.gnumake
            pkgs.gcc
            pkgs.autoconf
            pkgs.automake
            pkgs.libtool
            pkgs.patchelf
            pkgs.python3
            pkgs.git
            pkgs.file
            pkgs.rsync
            pkgs.which
            pkgs.bc
            pkgs.bison
            pkgs.flex
            pkgs.cpio
            pkgs.unzip
            pkgs.zip
            pkgs.perl
            pkgs.gawk
            pkgs.libxml2
            pkgs.wget
            pkgs.gzip
            pkgs.gnutar
            pkgs.openssh
            pkgs.clang
            pkgs.libclang
            pkgs.lld
            pkgs.cargo-nextest
            rustToolchain
            buildServer
            buildWebrtcRs
            buildWeb
          ]
          ++ lib.optionals (hostTools != null) [hostTools];

        shellHook =
          ''
            export GOFLAGS="-trimpath"
            export PNPM_HOME="$PWD/.pnpm-home"
            export PATH="$PNPM_HOME:$PATH"

            export RUST_SRC_PATH="${rustToolchain}/lib/rustlib/src/rust/library"
            export LIBCLANG_PATH="${pkgs.libclang.lib}/lib"

            export NANOKVM_GOOS=linux
            export NANOKVM_GOARCH=riscv64
            export NANOKVM_CGO_CFLAGS="-mcpu=c906fdv -march=rv64imafdcv0p7xthead -mcmodel=medany -mabi=lp64d"
            export NANOKVM_RUST_TARGET="''${NANOKVM_RUST_TARGET:-riscv64gc-unknown-linux-musl}"
          ''
          + lib.optionalString (hostTools != null) ''

            export NANOKVM_HOST_TOOLS="${hostTools}/host-tools"
            export PATH="$NANOKVM_HOST_TOOLS/gcc/riscv64-linux-musl-x86_64/bin:$PATH"
          ''
          + lib.optionalString (hostTools == null) ''

            echo "NanoKVM: vendor RISC-V musl toolchain is packaged for x86_64-linux only."
            echo "NanoKVM: native Rust/Go/web checks are available, but target builds need Linux x86_64."
          ''
          + ''

            echo "NanoKVM nix shell ready."
            echo "  nanokvm-webrtc-rs       # run Rust WebRTC sidecar locally"
            echo "  nanokvm-build-webrtc-rs # build target Rust sidecar into server/"
            echo "  nanokvm-build-server    # build server/NanoKVM-Server and NanoKVM-Server-legacy for riscv64 linux"
            echo "  nanokvm-build-web       # install frontend deps and build web/dist"
          '';
      };
    });
}
