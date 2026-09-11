# Rung 1: a Nix closure pushed through the tunnel.
#
# Rung 0 proved the tunnel carries an interactive ssh session. That is a
# different and much easier problem than the one this project exists to solve.
# A transport that handles a shell perfectly can still stall, deadlock or
# corrupt under a sustained multi-megabyte transfer, and the failure would show
# up as a deploy that hangs rather than one that errors.
#
# It is also the only property nobody involved has data on. Large closures go
# fine over AWS SSM, but that is Amazon's relay, engineered and operated by
# Amazon; ours is a few hundred lines old.
#
# The payload is created inside the orchestrator at test time. NixOS test nodes
# share the host's store, so copying any pre-existing path would transfer
# nothing and prove nothing — the copy has to be of something the target really
# does not have.
{ self, pkgs }:
let
  orchestratorPublicKey = "Gt+ivBgkZNNSQBvcRUX6LhhlGVfUupW+IZTu0JcWQ3A=";
  orchestratorPrivateKey = "/z1GJeSYMj+cr57Rfz64DRiY2dlV7IMZWpkTxNoQNBU=";

  sshPublicKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMPR3dZ3NnclYn3W4znSqC0dMjej/BEVxdJctSnR7C9y nivis-tunnel vm test";
  sshPrivateKey = ''
    -----BEGIN OPENSSH PRIVATE KEY-----
    b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
    QyNTUxOQAAACDD0d3WdzZ3JWJ91uM50qgtHTI3o/wRFcXSXLUp0ewvcgAAAJjzVOzr81Ts
    6wAAAAtzc2gtZWQyNTUxOQAAACDD0d3WdzZ3JWJ91uM50qgtHTI3o/wRFcXSXLUp0ewvcg
    AAAEBZdBRavNK8QtEY8pLh1kehwoGgtkve7/elHlCIIb9cBsPR3dZ3NnclYn3W4znSqC0d
    Mjej/BEVxdJctSnR7C9yAAAAFG5pdmlzLXR1bm5lbCB2bSB0ZXN0AQ==
    -----END OPENSSH PRIVATE KEY-----
  '';

  streamID = "poc-target-01";
  relayPort = 7843;

  system = pkgs.stdenv.hostPlatform.system;

  proxyCommand = "tunnel connect ${streamID} --relay relay:${toString relayPort} --key /root/orchestrator.key";
in
pkgs.testers.runNixOSTest {
  name = "nivis-tunnel-rung-1";

  nodes = {
    relay =
      { ... }:
      {
        systemd.services.nivis-tunnel-relay = {
          wantedBy = [ "multi-user.target" ];
          after = [ "network-online.target" ];
          wants = [ "network-online.target" ];
          serviceConfig = {
            ExecStart = "${self.packages.${system}.relay}/bin/relay --listen=:${toString relayPort} --log-level=debug";
            Restart = "always";
            DynamicUser = true;
          };
        };
        networking.firewall.allowedTCPPorts = [ relayPort ];
      };

    target =
      { ... }:
      {
        imports = [ self.nixosModules.default ];

        services.nivis-tunnel-agent = {
          enable = true;
          relay = "relay:${toString relayPort}";
          streamId = streamID;
          inherit orchestratorPublicKey;
        };

        services.openssh = {
          enable = true;
          settings.PasswordAuthentication = false;
          # Port lists merge rather than override, so this is what actually
          # keeps the machine closed. A successful copy below can therefore
          # only have gone through the tunnel.
          openFirewall = false;
        };
        users.users.root.openssh.authorizedKeys.keys = [ sshPublicKey ];
        networking.firewall.allowedTCPPorts = [ ];

        # The receiving end of a closure push needs somewhere to put it.
        virtualisation.writableStore = true;
        nix.settings.trusted-users = [ "root" ];
      };

    orchestrator =
      { ... }:
      {
        environment.systemPackages = [
          self.packages.${system}.tunnel
          pkgs.openssh
        ];

        # The payload is added here at test time, so it exists on this machine
        # and nowhere else.
        virtualisation.writableStore = true;

        environment.etc."nivis-tunnel-test/id_ed25519" = {
          text = sshPrivateKey;
          mode = "0600";
        };
        environment.etc."nivis-tunnel-test/orchestrator.key" = {
          text = orchestratorPrivateKey + "\n";
          mode = "0600";
        };
      };
  };

  testScript = ''
    start_all()

    relay.wait_for_unit("nivis-tunnel-relay.service")
    relay.wait_for_open_port(${toString relayPort})
    target.wait_for_unit("nivis-tunnel-agent.service")
    target.wait_for_unit("sshd.service")
    orchestrator.wait_for_unit("multi-user.target")

    orchestrator.succeed("mkdir -p /root/.ssh")
    orchestrator.succeed(
        "install -m600 /etc/nivis-tunnel-test/id_ed25519 /root/.ssh/id_ed25519"
    )
    orchestrator.succeed(
        "install -m600 /etc/nivis-tunnel-test/orchestrator.key /root/orchestrator.key"
    )

    ssh_opts = (
        "-o ProxyCommand='${proxyCommand}' "
        "-o StrictHostKeyChecking=accept-new "
        "-o UserKnownHostsFile=/root/.ssh/known_hosts "
        "-i /root/.ssh/id_ed25519"
    )

    with subtest("the target admits nothing directly"):
        # Without this, a successful copy below could mean the machines could
        # reach each other all along and the tunnel did nothing.
        orchestrator.fail(
            "timeout 10 ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 "
            "-i /root/.ssh/id_ed25519 root@target true"
        )

    with subtest("a payload is created that the target does not have"):
        orchestrator.succeed("head -c 16777216 /dev/urandom > /tmp/payload")
        expected = orchestrator.succeed("sha256sum /tmp/payload").split()[0]
        path = orchestrator.succeed("nix-store --add /tmp/payload").strip()
        print(f"rung 1 payload: {path} (16 MiB, sha256 {expected})")

        # The premise of the whole test. If the target already had it, the copy
        # would transfer nothing and prove nothing.
        target.fail(f"test -e {path}")

    with subtest("rung 1: nix-copy-closure pushes it through the tunnel"):
        # Warm the known_hosts entry first, so the copy measures the transfer
        # rather than an interactive host key prompt.
        orchestrator.succeed(f"timeout 60 ssh {ssh_opts} root@${streamID} true")

        start = orchestrator.succeed("date +%s").strip()
        orchestrator.succeed(
            f"NIX_SSHOPTS=\"{ssh_opts}\" timeout 600 "
            f"nix-copy-closure --to root@${streamID} {path}"
        )
        end = orchestrator.succeed("date +%s").strip()
        print(f"rung 1: 16 MiB closure copied in {int(end) - int(start)}s")

    with subtest("the closure arrived intact"):
        target.succeed(f"test -e {path}")
        got = target.succeed(f"sha256sum {path}").split()[0]
        assert got == expected, (
            f"the closure was altered in transit: sent {expected}, received {got}"
        )

    with subtest("a repeated copy sends nothing further"):
        # What makes every deploy after the first small, and therefore what
        # decides whether a relay is cheap or expensive to run.
        out = orchestrator.succeed(
            f"NIX_SSHOPTS=\"{ssh_opts}\" timeout 300 "
            f"nix-copy-closure --to root@${streamID} {path} 2>&1"
        )
        assert "copying 1 paths" not in out, (
            f"the path was sent a second time: {out}"
        )
  '';
}
