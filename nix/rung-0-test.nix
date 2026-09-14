# Rung 0: an ssh session to a machine that admits no inbound connection.
#
# This is the test the project exists to pass first. Everything above the
# transport — nix-copy-closure, switch-to-configuration, deploy-rs — is already
# proven in production elsewhere, hanging off one ssh ProxyCommand line. What
# has never been proven is the layer beneath it, and the claim is narrow enough
# to state as an experiment:
#
#   Given a host whose firewall admits nothing at all, an operator can
#   nonetheless open an ssh session to it and run a command.
#
# So the test asserts two things, and the second is as important as the first:
# that ssh through the tunnel works, and that ssh without it does NOT. A test
# that only proved the first could pass on a machine that was reachable all
# along, and would be proving nothing.
{ self, pkgs }:
let
  # A fixed keypair, because both halves must agree and a test should not
  # depend on generating one. The public half sits in the target's Nix store,
  # which is the point: a boot image carries key material and no secret.
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
in
pkgs.testers.runNixOSTest {
  name = "nivis-tunnel-rung-0";

  nodes = {
    # The rendezvous. The only machine in this test that accepts a connection.
    relay =
      { ... }:
      {
        # Through the module rather than a hand-written unit: a module only the
        # tests never use is a module nobody has run.
        imports = [ self.nixosModules.relay ];

        services.nivis-tunnel-relay = {
          enable = true;
          port = relayPort;
          # The one machine here that is supposed to accept a connection.
          openFirewall = true;
        };
      };

    # The target. Note what is absent: no allowedTCPPorts, so the firewall
    # admits nothing. sshd is running and cannot be reached from outside.
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
          # The important line. It defaults to true, and firewall port lists
          # MERGE rather than override — so declaring an empty allowedTCPPorts
          # below does not close 22, it just adds nothing to what openssh
          # already opened. The first run of this test proved exactly that by
          # reaching the target directly.
          openFirewall = false;
        };
        users.users.root.openssh.authorizedKeys.keys = [ sshPublicKey ];

        # Deliberately empty. The whole claim is that this machine is reachable
        # without opening anything.
        networking.firewall.allowedTCPPorts = [ ];
      };

    # The operator's machine.
    orchestrator =
      { ... }:
      {
        environment.systemPackages = [
          self.packages.${system}.tunnel
          pkgs.openssh
        ];

        # Both private keys arrive as configuration rather than through the
        # test script. A PEM contains newlines, and a newline inside a Python
        # string literal in the generated driver is a syntax error.
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

    with subtest("the relay module opened only its own port"):
        # openFirewall defaults to false and this machine asked for it, so the
        # relay's port must be open and nothing else the module added.
        relay.succeed("iptables -L nixos-fw -n | grep -q 'dpt:${toString relayPort}'")

    with subtest("the agent opens no port of its own"):
        # It dials outward and waits. If it listened, the design would be a
        # different one and the firewall below would not be the whole story.
        #
        # Checked against the service's actual PID rather than a process name:
        # grepping for a name that does not appear would pass whether or not
        # the agent listened, and prove nothing.
        pid = target.succeed(
            "systemctl show -p MainPID --value nivis-tunnel-agent.service"
        ).strip()
        assert pid not in ("", "0"), "the agent service has no main process"
        listening = target.succeed(f"ss -ltnp | grep 'pid={pid},' || true").strip()
        assert listening == "", f"the agent is listening on a port: {listening}"

    with subtest("the agent's configuration carries key material but no secret"):
        # This is what lets a boot image be published at all. Asserted against
        # the target's own unit rather than against /nix/store, because NixOS
        # test nodes share the host store — a grep there would also find the
        # orchestrator's files and prove nothing about the target.
        unit = target.succeed(
            "systemctl cat nivis-tunnel-agent.service"
        )
        assert "${orchestratorPublicKey}" in unit, (
            f"the agent is not configured with the orchestrator public key: {unit}"
        )
        assert "${orchestratorPrivateKey}" not in unit, (
            "the orchestrator PRIVATE key reached the target's configuration"
        )

    orchestrator.succeed("mkdir -p /root/.ssh")
    orchestrator.succeed(
        "install -m600 /etc/nivis-tunnel-test/id_ed25519 /root/.ssh/id_ed25519"
    )
    orchestrator.succeed(
        "install -m600 /etc/nivis-tunnel-test/orchestrator.key /root/orchestrator.key"
    )

    with subtest("the target is NOT reachable directly"):
        # The load-bearing negative. Without this, a passing test above could
        # mean the machine was reachable all along and the tunnel did nothing.
        orchestrator.fail(
            "timeout 10 ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 "
            "-i /root/.ssh/id_ed25519 root@target true"
        )

    with subtest("rung 0: ssh reaches the target through the tunnel"):
        out = orchestrator.succeed(
            "timeout 120 ssh "
            "-o ProxyCommand='tunnel connect ${streamID} --relay relay:${toString relayPort} --key /root/orchestrator.key' "
            "-o StrictHostKeyChecking=accept-new "
            "-o ConnectTimeout=30 "
            "-i /root/.ssh/id_ed25519 "
            "root@${streamID} "
            "'hostname; systemctl is-active nivis-tunnel-agent'"
        )
        assert "target" in out, f"the command did not run on the target: {out}"
        assert "active" in out, f"the agent was not active: {out}"

    with subtest("the machine is reachable again afterwards"):
        # A deploy is not a one-shot. The agent must return to waiting without
        # anything restarting it — there is no second channel to restart it
        # through.
        out = orchestrator.succeed(
            "timeout 120 ssh "
            "-o ProxyCommand='tunnel connect ${streamID} --relay relay:${toString relayPort} --key /root/orchestrator.key' "
            "-o StrictHostKeyChecking=accept-new "
            "-o ConnectTimeout=30 "
            "-i /root/.ssh/id_ed25519 "
            "root@${streamID} "
            "'echo second-session'"
        )
        assert "second-session" in out, f"the second session failed: {out}"

    with subtest("a stream carrying real volume survives the tunnel"):
        # ssh alone proves the path; it does not prove the path under load. A
        # closure push is megabytes, and a transport that carries an
        # interactive shell perfectly can still stall, deadlock or corrupt on
        # sustained transfer.
        #
        # The bytes must actually cross the tunnel, so the payload is produced
        # on the target and hashed on the orchestrator. Hashing it on the
        # target would send 64 characters over the wire and prove nothing.
        target.succeed("head -c 33554432 /dev/urandom > /tmp/blob")
        expected = target.succeed("sha256sum /tmp/blob").split()[0]

        orchestrator.succeed(
            "timeout 300 ssh "
            "-o ProxyCommand='tunnel connect ${streamID} --relay relay:${toString relayPort} --key /root/orchestrator.key' "
            "-o StrictHostKeyChecking=accept-new "
            "-i /root/.ssh/id_ed25519 "
            "root@${streamID} "
            "'cat /tmp/blob' > /tmp/pulled"
        )

        size = orchestrator.succeed("stat -c %s /tmp/pulled").strip()
        assert size == "33554432", f"only {size} bytes of 33554432 came through the tunnel"

        got = orchestrator.succeed("sha256sum /tmp/pulled").split()[0]
        assert got == expected, (
            f"32 MiB was altered in transit: target {expected}, received {got}"
        )

    with subtest("an impostor cannot reach the target"):
        # A different orchestrator key must not open a session, and must not
        # take the agent out of reach either.
        orchestrator.succeed(
            "tunnel keygen --out /root/impostor.key >/dev/null 2>&1"
        )
        orchestrator.fail(
            "timeout 60 ssh "
            "-o ProxyCommand='tunnel connect ${streamID} --relay relay:${toString relayPort} --key /root/impostor.key' "
            "-o StrictHostKeyChecking=accept-new "
            "-o ConnectTimeout=20 "
            "-i /root/.ssh/id_ed25519 "
            "root@${streamID} true"
        )

        out = orchestrator.succeed(
            "timeout 120 ssh "
            "-o ProxyCommand='tunnel connect ${streamID} --relay relay:${toString relayPort} --key /root/orchestrator.key' "
            "-o StrictHostKeyChecking=accept-new "
            "-o ConnectTimeout=30 "
            "-i /root/.ssh/id_ed25519 "
            "root@${streamID} "
            "'echo still-here'"
        )
        assert "still-here" in out, f"the agent did not survive an impostor: {out}"
  '';
}
