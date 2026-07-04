# nixos-reboot-required-collector

Prometheus textfile collector for reporting whether a NixOS host needs a reboot.

It resolves the booted system and current system profile, then compares only the parts that matter for a reboot:

- `kernel`
- `initrd`
- `kernel-params`
- `systemd`

If any of those paths exists on both sides and resolves to a different target, `nixos_reboot_required` is `1`. Otherwise it is `0`.

This is useful for NixOS machines using automatic upgrades without automatic reboots:

```nix
system.autoUpgrade = {
  enable = true;
  allowReboot = false;
};
```

The collector is not a long-running exporter. It writes a Prometheus `.prom` file that can be exposed by the Prometheus node-exporter textfile collector.

## Metric

```text
# HELP nixos_reboot_required Whether a reboot is required for reboot-relevant parts of the current NixOS system profile to become active.
# TYPE nixos_reboot_required gauge
nixos_reboot_required 0
```

## Usage

Run the collector manually:

```bash
nixos-reboot-required-collector
```

Defaults:

```text
-booted-system   /run/booted-system
-current-system  /nix/var/nix/profiles/system
-output          /var/lib/node_exporter/textfile_collector/nixos_reboot_required.prom
```

Run with a custom output path:

```bash
nixos-reboot-required-collector \
  -output /tmp/nixos_reboot_required.prom
```

Inspect the generated metric:

```bash
cat /tmp/nixos_reboot_required.prom
```

## Usage on a NixOS machine

The recommended setup is:

```text
nixos-reboot-required-collector
  writes a .prom file
    ↓
node-exporter textfile collector
  exposes it on /metrics
    ↓
Prometheus
  scrapes node-exporter
```

### Add the flake input

In your NixOS flake:

```nix
{
  inputs.nixos-reboot-required-collector = {
    url = "github:maveonair/nixos-reboot-required-collector";
    inputs.nixpkgs.follows = "nixpkgs";
  };
}
```

### Pass the input to your NixOS system

Example:

```nix
{
  outputs =
    inputs@{ nixpkgs, nixos-reboot-required-collector, ... }:
    {
      nixosConfigurations.myhost = nixpkgs.lib.nixosSystem {
        system = "x86_64-linux";

        specialArgs = {
          inherit inputs;
        };

        modules = [
          ./configuration.nix
        ];
      };
    };
}
```

### Add a NixOS module

Create a module, for example:

```text
modules/monitoring/nixos-reboot-required-collector.nix
```

```nix
{
  inputs,
  pkgs,
  ...
}:

let
  collector =
    inputs.nixos-reboot-required-collector.packages.${pkgs.system}.default;

  textfileDirectory =
    "/var/lib/node_exporter/textfile_collector";
in
{
  services.prometheus.exporters.node = {
    enable = true;

    enabledCollectors = [
      "textfile"
    ];

    extraFlags = [
      "--collector.textfile.directory=${textfileDirectory}"
    ];
  };

  systemd.tmpfiles.rules = [
    "d ${textfileDirectory} 0755 root root -"
  ];

  systemd.services.nixos-reboot-required-collector = {
    description = "Write NixOS reboot-required metric for node-exporter";

    serviceConfig = {
      Type = "oneshot";
      ExecStart = "${collector}/bin/nixos-reboot-required-collector -output ${textfileDirectory}/nixos_reboot_required.prom";
    };
  };

  systemd.timers.nixos-reboot-required-collector = {
    wantedBy = [
      "timers.target"
    ];

    timerConfig = {
      OnBootSec = "2m";
      OnUnitActiveSec = "5m";
      AccuracySec = "1m";
      Unit = "nixos-reboot-required-collector.service";
    };
  };
}
```

Then import it in your host configuration:

```nix
{
  imports = [
    ./modules/monitoring/nixos-reboot-required-collector.nix
  ];
}
```

### Prometheus alert

Example alert rule:

```yaml
groups:
  - name: nixos
    rules:
      - alert: NixOSRebootRequired
        expr: nixos_reboot_required == 1
        for: 30m
        labels:
          severity: warning
        annotations:
          summary: "NixOS reboot required on {{ $labels.instance }}"
          description: "A reboot is required for reboot-relevant parts of the current NixOS system profile to become active."
```

## Development

Enter the development shell:

```bash
nix develop
```

Run tests:

```bash
go test ./...
```

Build with Nix:

```bash
nix build
```
