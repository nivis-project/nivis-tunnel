# NixOS module for the rendezvous relay.
#
# The relay is the one component in this project that accepts a connection. The
# agent dials outward and listens on nothing; the orchestrator is invoked per
# connection. So this is the only place a port is opened, and the only place
# where getting the defaults wrong makes a machine reachable that its own
# configuration says should not be.
#
# It is also the component with the least to protect. It holds no key material,
# authenticates nobody, and cannot read what it carries — the Noise handshake
# runs end to end through it. The hardening below enforces that rather than
# merely relying on it being true.
{ self }:
{
  config,
  lib,
  pkgs,
  ...
}:
let
  cfg = config.services.nivis-tunnel-relay;
in
{
  options.services.nivis-tunnel-relay = {
    enable = lib.mkEnableOption "the nivis-tunnel rendezvous relay";

    package = lib.mkOption {
      type = lib.types.package;
      default = self.packages.${pkgs.stdenv.hostPlatform.system}.relay;
      defaultText = lib.literalExpression "nivis-tunnel.packages.\${system}.relay";
      description = "The relay package to run.";
    };

    port = lib.mkOption {
      type = lib.types.port;
      default = 7843;
      description = ''
        Port to accept rendezvous connections on.

        Both the agent on a target and the orchestrator dial this, so it has to
        be reachable from both. That is usually the constraint that decides it:
        an orchestrator on a hotel or corporate network may not be able to reach
        an unusual port outbound, while a cloud target almost always can.
      '';
    };

    address = lib.mkOption {
      type = lib.types.str;
      default = "";
      example = "10.0.0.5";
      description = ''
        Address to bind to. Empty means every interface.

        Binding to one interface is a real restriction, but note that it does
        not substitute for the firewall option below: a relay bound to all
        interfaces on a machine whose firewall is closed is still unreachable,
        and a relay bound to one interface on a machine whose firewall is open
        is still exposed on that interface.
      '';
    };

    openFirewall = lib.mkOption {
      type = lib.types.bool;
      default = false;
      description = ''
        Whether to open the relay's port in the firewall.

        Off by default, deliberately. Opening a port is a decision an operator
        makes, and NixOS firewall port lists MERGE rather than override — so a
        module that opens one quietly can leave a machine reachable that its own
        configuration says should not be. That is not hypothetical: the first
        run of this project's rung 0 test reached a target that was supposed to
        admit nothing, because `services.openssh.openFirewall` defaults to true
        and the empty `allowedTCPPorts` next to it added nothing rather than
        closing anything.
      '';
    };

    frameTimeout = lib.mkOption {
      type = lib.types.str;
      default = "10s";
      description = ''
        How long a connection may stay silent after being accepted.

        Short on purpose: announcing yourself costs one small write, so a
        connection that has not done it is not going to.
      '';
    };

    rendezvousTimeout = lib.mkOption {
      type = lib.types.str;
      default = "5m";
      description = ''
        How long a party waits for its counterpart before it is released.

        Generous on purpose: the usual wait is a machine finishing its boot.
      '';
    };

    maxParked = lib.mkOption {
      type = lib.types.ints.positive;
      default = 1024;
      description = ''
        How many connections may wait to be paired at once. Further ones are
        refused rather than absorbed, so an unmatched flood cannot take down the
        sessions that are working.
      '';
    };

    logLevel = lib.mkOption {
      type = lib.types.enum [
        "debug"
        "info"
        "warn"
        "error"
      ];
      default = "info";
      description = "Verbosity of the relay's structured log.";
    };
  };

  config = lib.mkIf cfg.enable {
    systemd.services.nivis-tunnel-relay = {
      description = "nivis-tunnel rendezvous relay";
      wantedBy = [ "multi-user.target" ];
      after = [ "network-online.target" ];
      wants = [ "network-online.target" ];

      serviceConfig = {
        ExecStart = lib.escapeShellArgs [
          (lib.getExe cfg.package)
          "--listen=${cfg.address}:${toString cfg.port}"
          "--frame-timeout=${cfg.frameTimeout}"
          "--rendezvous-timeout=${cfg.rendezvousTimeout}"
          "--max-parked=${toString cfg.maxParked}"
          "--log-level=${cfg.logLevel}"
        ];

        Restart = "always";
        RestartSec = "5s";

        # A relay is the only route to a machine with no inbound port, so an
        # outage stops every deploy against it. It drains live sessions on
        # stop rather than dropping them.
        KillSignal = "SIGTERM";
        TimeoutStopSec = "30s";

        # The relay holds no key material, reads no configuration and writes
        # nothing. Enforce that rather than relying on it staying true.
        DynamicUser = true;
        CapabilityBoundingSet = [ "" ];
        AmbientCapabilities = [ "" ];
        NoNewPrivileges = true;
        ProtectSystem = "strict";
        ProtectHome = true;
        ProtectKernelTunables = true;
        ProtectKernelModules = true;
        ProtectControlGroups = true;
        PrivateDevices = true;
        PrivateTmp = true;
        RestrictNamespaces = true;
        RestrictRealtime = true;
        RestrictSUIDSGID = true;
        LockPersonality = true;
        MemoryDenyWriteExecute = true;
        SystemCallArchitectures = "native";
        SystemCallFilter = [
          "@system-service"
          "~@privileged"
          "~@resources"
        ];
        RestrictAddressFamilies = [
          "AF_INET"
          "AF_INET6"
        ];
      };
    };

    networking.firewall.allowedTCPPorts = lib.mkIf cfg.openFirewall [ cfg.port ];
  };
}
