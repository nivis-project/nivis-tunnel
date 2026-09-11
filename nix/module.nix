# NixOS module for the nivis-tunnel agent.
#
# This is the half of the system that goes into the BOOT IMAGE, which makes it
# the part with the strictest change budget: partition layout, filesystem,
# boot mode and this agent are the only things that force an image rebuild.
# Everything else about a machine can be pushed as a closure afterwards.
#
# The consequence is a rule to hold to: the agent baked into the image needs to
# be good enough to accept ONE push and no more. A newer agent can then arrive
# through the live configuration like any other package. Keep this module dull.
{ self }:
{
  config,
  lib,
  pkgs,
  ...
}:
let
  cfg = config.services.nivis-tunnel-agent;
in
{
  options.services.nivis-tunnel-agent = {
    enable = lib.mkEnableOption "the nivis-tunnel agent";

    package = lib.mkOption {
      type = lib.types.package;
      default = self.packages.${pkgs.stdenv.hostPlatform.system}.agent;
      defaultText = lib.literalExpression "nivis-tunnel.packages.\${system}.agent";
      description = "The agent package to run.";
    };

    relay = lib.mkOption {
      type = lib.types.str;
      example = "relay.example.org:443";
      description = ''
        Address of the rendezvous relay. The agent dials out to it and waits;
        nothing ever dials in, which is the property that makes this work on a
        host with no inbound port and no public address.
      '';
    };

    streamId = lib.mkOption {
      type = lib.types.str;
      example = "i-0abc123def456789";
      description = ''
        The rendezvous id this host announces. An identifier, never a
        credential: anyone may claim one, and the trust that matters comes from
        `orchestratorPublicKey` below.
      '';
    };

    orchestratorPublicKey = lib.mkOption {
      type = lib.types.str;
      description = ''
        The only peer this agent will accept, as a Noise public key.

        A public key, so it is safe in a public repository and safe in the
        world-readable Nix store. That is deliberate: it means the boot image
        carries no secret at all, and the target proves nothing about itself —
        it is identified by the address the cloud API returned.
      '';
    };

    sshPort = lib.mkOption {
      type = lib.types.port;
      default = 22;
      description = "Local port the accepted stream is spliced onto.";
    };
  };

  config = lib.mkIf cfg.enable {
    systemd.services.nivis-tunnel-agent = {
      description = "nivis-tunnel agent";
      wantedBy = [ "multi-user.target" ];
      after = [ "network-online.target" ];
      wants = [ "network-online.target" ];
      serviceConfig = {
        ExecStart = lib.escapeShellArgs [
          (lib.getExe cfg.package)
          "--relay=${cfg.relay}"
          "--stream-id=${cfg.streamId}"
          "--orchestrator-key=${cfg.orchestratorPublicKey}"
          "--local=127.0.0.1:${toString cfg.sshPort}"
        ];
        Restart = "always";
        RestartSec = "5s";
        DynamicUser = true;
        # Outbound only. The unit needs no privileges and opens no port.
        CapabilityBoundingSet = [ "" ];
        NoNewPrivileges = true;
        ProtectSystem = "strict";
        ProtectHome = true;
        PrivateDevices = true;
        RestrictAddressFamilies = [
          "AF_INET"
          "AF_INET6"
        ];
      };
    };
  };
}
