{
  description = "nivis-tunnel — a cloud-neutral deploy transport for NixOS closures";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs =
    { self, nixpkgs }:
    let
      # Supported systems are enumerated here in plain Nix. flake-utils is
      # deliberately NOT used: one `genAttrs` is the whole abstraction it
      # provides, and keeping it explicit means a reader can see exactly which
      # systems are built without learning another flake library.
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];

      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});

      version = "0.1.0-dev";

      # Pins the dependency tree the three binaries are built from. The agent
      # ships inside a boot image, so what it links against is part of that
      # image's change budget; this hash is what makes that auditable.
      #
      # Stated once: the binaries and the test derivation must be built from
      # the same tree, or the gate would be testing something other than what
      # it ships.
      vendorHash = "sha256-wKrycy8pg6UBnRtkM/xmXE5Ky/yXV3+Q8xk3n6VHClE=";

      # One Go module builds three binaries; they share the wire protocol in
      # ./proto, which is the reason they live in one repository at all.
      mkBinary =
        pkgs: name:
        pkgs.buildGoModule {
          pname = "nivis-tunnel-${name}";
          inherit version;
          src = ./.;
          inherit vendorHash;
          subPackages = [ "cmd/${name}" ];
          ldflags = [
            "-s"
            "-w"
            "-X main.version=${version}"
          ];
          meta.mainProgram = name;
        };
    in
    {
      packages = forAllSystems (pkgs: {
        relay = mkBinary pkgs "relay";
        agent = mkBinary pkgs "agent";
        tunnel = mkBinary pkgs "tunnel";
        default = self.packages.${pkgs.stdenv.hostPlatform.system}.tunnel;
      });

      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          packages = [
            pkgs.go
            pkgs.gopls
            pkgs.go-tools
            pkgs.golangci-lint
            pkgs.jujutsu
            pkgs.openssh
            pkgs.nixfmt-rfc-style
          ];
        };
      });

      # The gate. `nix flake check` must stay green: it is what every OpenSpec
      # change is measured against before it can be archived.
      checks = forAllSystems (pkgs: {
        build-relay = self.packages.${pkgs.stdenv.hostPlatform.system}.relay;
        build-agent = self.packages.${pkgs.stdenv.hostPlatform.system}.agent;
        build-tunnel = self.packages.${pkgs.stdenv.hostPlatform.system}.tunnel;

        # Run through buildGoModule rather than a bare `go test`: the Nix
        # sandbox has no network, so the tests must be built from the same
        # vendored tree as the binaries. A hand-rolled runCommand would try to
        # fetch modules and fail.
        unit = pkgs.buildGoModule {
          pname = "nivis-tunnel-tests";
          inherit version vendorHash;
          src = ./.;
          doCheck = true;
          installPhase = "touch $out";
        };

        fmt = pkgs.runCommand "nixfmt-check" { nativeBuildInputs = [ pkgs.nixfmt-rfc-style ]; } ''
          nixfmt --check ${./flake.nix} && touch $out
        '';
      });

      formatter = forAllSystems (pkgs: pkgs.nixfmt-rfc-style);

      nixosModules.default = import ./nix/module.nix { inherit self; };
    };
}
