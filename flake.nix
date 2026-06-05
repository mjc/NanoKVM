{
  description = "NanoKVM development shell";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs =
    { nixpkgs, ... }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];

      forAllSystems = nixpkgs.lib.genAttrs systems;
    in
    {
      devShells = forAllSystems (
        system:
        let
          pkgs = import nixpkgs { inherit system; };
          lib = pkgs.lib;

          go = pkgs.go_1_25 or pkgs.go_1_24 or pkgs.go;
          nodejs = pkgs.nodejs_24 or pkgs.nodejs_22 or pkgs.nodejs;

          hostTools =
            if system == "x86_64-linux" then
              pkgs.stdenvNoCC.mkDerivation {
                pname = "sophon-host-tools";
                version = "2023-03-07";

                src = pkgs.fetchurl {
                  url = "https://sophon-file.sophon.cn/sophon-prod-s3/drive/23/03/07/16/host-tools.tar.gz";
                  hash = "sha256-/5pY6OGSsg6kLh1ynELSIZUjIJcGyz8M8TRYL2xw+AU=";
                };

                nativeBuildInputs = [ pkgs.autoPatchelfHook ];
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
            else
              null;

          buildServer = pkgs.writeShellApplication {
            name = "nanokvm-build-server";
            runtimeInputs =
              [
                go
                pkgs.git
                pkgs.patchelf
              ]
              ++ lib.optionals (hostTools != null) [ hostTools ];
            text =
              if hostTools != null then
                ''
                  repo_root="$(git rev-parse --show-toplevel)"
                  cd "$repo_root/server"

                  export PATH="${hostTools}/host-tools/gcc/riscv64-linux-musl-x86_64/bin:$PATH"
                  export CGO_ENABLED=1
                  export GOOS="''${NANOKVM_GOOS:-linux}"
                  export GOARCH="''${NANOKVM_GOARCH:-riscv64}"
                  export CC="''${NANOKVM_CC:-riscv64-unknown-linux-musl-gcc}"
                  export CGO_CFLAGS="''${NANOKVM_CGO_CFLAGS:--mcpu=c906fdv -march=rv64imafdcv0p7xthead -mcmodel=medany -mabi=lp64d}"

                  go mod download
                  go build -o NanoKVM-Server
                  patchelf --add-rpath "\$ORIGIN/dl_lib" NanoKVM-Server
                ''
              else
                ''
                  echo "nanokvm-build-server requires the x86_64-linux Sophon RISC-V musl toolchain." >&2
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

          buildSupport = pkgs.writeShellApplication {
            name = "nanokvm-build-support";
            runtimeInputs = [
              pkgs.git
              pkgs.python3
              pkgs.python3Packages.virtualenv
              pkgs.cmake
              pkgs.ninja
              pkgs.gnumake
              pkgs.gcc
              pkgs.wget
              pkgs.unzip
            ];
            text = ''
              repo_root="$(git rev-parse --show-toplevel)"
              maixcdk_dir="''${NANOKVM_MAIXCDK_DIR:-$HOME/src/MaixCDK}"
              work_dir="$repo_root/.image-work"
              fake_home="$work_dir/home"

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

              rm -rf "$fake_home"
              mkdir -p "$fake_home"
              ln -s "$maixcdk_dir" "$fake_home/MaixCDK"
              ln -s "$repo_root" "$fake_home/NanoKVM"

              (
                export HOME="$fake_home"
                # shellcheck disable=SC1091
                source "$fake_home/MaixCDK/bin/activate"
                cd "$repo_root/support/sg2002"
                ./build kvm_system
                ./build kvm_system add_to_kvmapp
              )
            '';
          };

          buildImage = pkgs.writeShellApplication {
            name = "nanokvm-build-image";
            runtimeInputs = [
              pkgs.git
              pkgs.bash
              pkgs.rsync
              pkgs.findutils
              pkgs.coreutils
              pkgs.gnused
              pkgs.gnugrep
              pkgs.util-linux
              pkgs.fuse3
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
              pkgs.cmake
              pkgs.ninja
              pkgs.gnumake
              pkgs.gcc
              pkgs.python3
              pkgs.python3Packages.virtualenv
              pkgs.wget
              buildServer
              buildWeb
              buildSupport
            ];
            text = ''
              repo_root="$(git rev-parse --show-toplevel)"
              exec "$repo_root/scripts/build-nanokvm-image.sh" "$@"
            '';
          };
        in
        {
          default = pkgs.mkShell {
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
                pkgs.python3Packages.virtualenv
                pkgs.git
                pkgs.file
                pkgs.rsync
                pkgs.util-linux
                pkgs.fuse3
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
                buildServer
                buildWeb
                buildSupport
                buildImage
              ]
              ++ lib.optionals (hostTools != null) [ hostTools ];

            shellHook =
              ''
                export GOFLAGS="-trimpath"
                export PNPM_HOME="$PWD/.pnpm-home"
                export PATH="$PNPM_HOME:$PATH"

                export NANOKVM_GOOS=linux
                export NANOKVM_GOARCH=riscv64
                export NANOKVM_CGO_CFLAGS="-mcpu=c906fdv -march=rv64imafdcv0p7xthead -mcmodel=medany -mabi=lp64d"
                export NANOKVM_SDK_DIR="''${NANOKVM_SDK_DIR:-$HOME/src/licheerv-nano}"
                export NANOKVM_SDK_URL="''${NANOKVM_SDK_URL:-https://github.com/sipeed/LicheeRV-Nano-Build}"
                export NANOKVM_MAIXCDK_DIR="''${NANOKVM_MAIXCDK_DIR:-$HOME/src/MaixCDK}"
                export NANOKVM_SDK_REF="''${NANOKVM_SDK_REF:-main}"
                export NANOKVM_IMAGE_BOARD="''${NANOKVM_IMAGE_BOARD:-sg2002_licheervnano_sd}"
                export NANOKVM_IMAGE_OUTPUT_DIR="''${NANOKVM_IMAGE_OUTPUT_DIR:-dist/images}"
              ''
              + lib.optionalString (hostTools != null) ''

                export NANOKVM_HOST_TOOLS="${hostTools}/host-tools"
                export PATH="$NANOKVM_HOST_TOOLS/gcc/riscv64-linux-musl-x86_64/bin:$PATH"
              ''
              + lib.optionalString (hostTools == null) ''

                echo "NanoKVM: vendor RISC-V musl toolchain is packaged for x86_64-linux only."
                echo "NanoKVM: web builds and native Go tooling are available, but target server builds need Linux x86_64."
              ''
              + ''

                echo "NanoKVM nix shell ready."
                echo "  nanokvm-build-server  # build server/NanoKVM-Server for riscv64 linux"
                echo "  nanokvm-build-web     # install frontend deps and build web/dist"
                echo "  nanokvm-build-support # build kvm_system with MaixCDK"
                echo "  nanokvm-build-image   # build SDK image and overlay /kvmapp"
              '';
          };
        }
      );
    };
}
