# nixos-reboot-required-collector

Prometheus textfile collector for reporting whether a NixOS host needs a reboot.

It resolves the booted system and current system profile, then compares only the
parts that matter for a reboot:

- `kernel`
- `initrd`
- `kernel-params`
- `systemd`

If any of those paths exists on both sides and resolves to a different target,
`nixos_reboot_required` is `1`. Otherwise it is `0`.

## Metric

```prometheus
# HELP nixos_reboot_required Whether a reboot is required for reboot-relevant parts of the current NixOS system profile to become active.
# TYPE nixos_reboot_required gauge
nixos_reboot_required 0
```

## Usage

```sh
nixos-reboot-required-collector
```

Defaults:

```text
-booted-system  /run/booted-system
-current-system /nix/var/nix/profiles/system
-output         /var/lib/node_exporter/textfile_collector/nixos_reboot_required.prom
```

## Development

Enter the development shell:

```sh
nix develop
```

Run tests:

```sh
go test ./...
```

Build with Nix:

```sh
nix build
```
