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

      # One Go module builds three binaries; they share the wire protocol in
      # ./proto, which is the reason they live in one repository at all.
      mkBinary =
        pkgs: name:
        pkgs.buildGoModule {
          pname = "nivis-tunnel-${name}";
          inherit version;
          src = ./.;
          # No external dependencies yet. Replace with the real hash the moment
          # go.mod gains one; `nix build` prints the expected value on mismatch.
          vendorHash = null;
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

        unit = pkgs.runCommand "go-test" { nativeBuildInputs = [ pkgs.go ]; } ''
          export HOME=$TMPDIR
          export GOFLAGS=-mod=mod
          export GOCACHE=$TMPDIR/go-cache
          cp -r ${./.} src && chmod -R +w src && cd src
          go test ./... 2>&1 | tee $out
        '';

        fmt = pkgs.runCommand "nixfmt-check" { nativeBuildInputs = [ pkgs.nixfmt-rfc-style ]; } ''
          nixfmt --check ${./flake.nix} && touch $out
        '';
      });

      formatter = forAllSystems (pkgs: pkgs.nixfmt-rfc-style);

      nixosModules.default = import ./nix/module.nix { inherit self; };
    };
}
