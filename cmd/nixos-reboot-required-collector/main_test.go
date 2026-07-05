package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckRebootRequired(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want bool
		make func(t *testing.T, dir string) (bootedPath string, currentPath string)
	}{
		{
			name: "same system does not require reboot",
			want: false,
			make: func(t *testing.T, dir string) (string, string) {
				system := mkdir(t, filepath.Join(dir, "system"))

				return system, system
			},
		},
		{
			name: "different systems without reboot-relevant paths do not require reboot",
			want: false,
			make: func(t *testing.T, dir string) (string, string) {
				bootedSystem := mkdir(t, filepath.Join(dir, "booted-system-target"))
				currentSystem := mkdir(t, filepath.Join(dir, "current-system-target"))

				return bootedSystem, currentSystem
			},
		},
		{
			name: "same reboot-relevant paths do not require reboot",
			want: false,
			make: func(t *testing.T, dir string) (string, string) {
				bootedSystem := mkdir(t, filepath.Join(dir, "booted-system-target"))
				currentSystem := mkdir(t, filepath.Join(dir, "current-system-target"))
				sharedKernel := touch(t, filepath.Join(dir, "shared-kernel"))

				symlink(t, sharedKernel, filepath.Join(bootedSystem, "kernel"))
				symlink(t, sharedKernel, filepath.Join(currentSystem, "kernel"))

				return bootedSystem, currentSystem
			},
		},
		{
			name: "different reboot-relevant path requires reboot",
			want: true,
			make: func(t *testing.T, dir string) (string, string) {
				bootedSystem := mkdir(t, filepath.Join(dir, "booted-system-target"))
				currentSystem := mkdir(t, filepath.Join(dir, "current-system-target"))
				bootedKernel := touch(t, filepath.Join(dir, "booted-kernel"))
				currentKernel := touch(t, filepath.Join(dir, "current-kernel"))

				symlink(t, bootedKernel, filepath.Join(bootedSystem, "kernel"))
				symlink(t, currentKernel, filepath.Join(currentSystem, "kernel"))

				return bootedSystem, currentSystem
			},
		},
		{
			name: "non reboot-relevant difference does not require reboot",
			want: false,
			make: func(t *testing.T, dir string) (string, string) {
				bootedSystem := mkdir(t, filepath.Join(dir, "booted-system-target"))
				currentSystem := mkdir(t, filepath.Join(dir, "current-system-target"))

				mkdir(t, filepath.Join(bootedSystem, "etc"))
				mkdir(t, filepath.Join(currentSystem, "etc"))
				touch(t, filepath.Join(bootedSystem, "etc", "os-release"))
				touch(t, filepath.Join(currentSystem, "etc", "os-release"))

				return bootedSystem, currentSystem
			},
		},
		{
			name: "reboot-relevant path on only one side does not require reboot",
			want: false,
			make: func(t *testing.T, dir string) (string, string) {
				bootedSystem := mkdir(t, filepath.Join(dir, "booted-system-target"))
				currentSystem := mkdir(t, filepath.Join(dir, "current-system-target"))

				touch(t, filepath.Join(bootedSystem, "kernel"))

				return bootedSystem, currentSystem
			},
		},
		{
			name: "same kernel params content does not require reboot",
			want: false,
			make: func(t *testing.T, dir string) (string, string) {
				bootedSystem := mkdir(t, filepath.Join(dir, "booted-system-target"))
				currentSystem := mkdir(t, filepath.Join(dir, "current-system-target"))

				writeFile(t, filepath.Join(bootedSystem, "kernel-params"), "console=ttyS0 quiet")
				writeFile(t, filepath.Join(currentSystem, "kernel-params"), "console=ttyS0 quiet")

				return bootedSystem, currentSystem
			},
		},
		{
			name: "different kernel params content requires reboot",
			want: true,
			make: func(t *testing.T, dir string) (string, string) {
				bootedSystem := mkdir(t, filepath.Join(dir, "booted-system-target"))
				currentSystem := mkdir(t, filepath.Join(dir, "current-system-target"))

				writeFile(t, filepath.Join(bootedSystem, "kernel-params"), "console=ttyS0 quiet")
				writeFile(t, filepath.Join(currentSystem, "kernel-params"), "console=ttyS0")

				return bootedSystem, currentSystem
			},
		},
		{
			name: "different kernel params files with same content do not require reboot",
			want: false,
			make: func(t *testing.T, dir string) (string, string) {
				bootedSystem := mkdir(t, filepath.Join(dir, "booted-system-target"))
				currentSystem := mkdir(t, filepath.Join(dir, "current-system-target"))
				bootedParams := writeFile(t, filepath.Join(dir, "booted-kernel-params"), "console=ttyS0 quiet")
				currentParams := writeFile(t, filepath.Join(dir, "current-kernel-params"), "console=ttyS0 quiet")

				symlink(t, bootedParams, filepath.Join(bootedSystem, "kernel-params"))
				symlink(t, currentParams, filepath.Join(currentSystem, "kernel-params"))

				return bootedSystem, currentSystem
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			bootedPath, currentPath := tt.make(t, dir)

			result, err := checkRebootRequired(bootedPath, currentPath)
			if err != nil {
				t.Fatalf("checkRebootRequired() error = %v", err)
			}

			if result.RebootRequired != tt.want {
				t.Fatalf("RebootRequired = %v, want %v", result.RebootRequired, tt.want)
			}
		})
	}
}

func TestCheckRebootRequiredReturnsErrorForBrokenRebootRelevantSymlink(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	bootedSystem := mkdir(t, filepath.Join(dir, "booted-system-target"))
	currentSystem := mkdir(t, filepath.Join(dir, "current-system-target"))

	symlink(t, filepath.Join(dir, "missing-kernel"), filepath.Join(bootedSystem, "kernel"))
	touch(t, filepath.Join(currentSystem, "kernel"))

	_, err := checkRebootRequired(bootedSystem, currentSystem)
	if err == nil {
		t.Fatal("checkRebootRequired() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "check kernel") {
		t.Fatalf("checkRebootRequired() error = %q, want it to contain %q", err, "check kernel")
	}
}

func TestPrometheusOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		result Result
		want   string
	}{
		{
			name:   "reboot not required",
			result: Result{RebootRequired: false},
			want: `# HELP nixos_reboot_required Whether a reboot is required for reboot-relevant parts of the current NixOS system profile to become active.
# TYPE nixos_reboot_required gauge
nixos_reboot_required 0
`,
		},
		{
			name:   "reboot required",
			result: Result{RebootRequired: true},
			want: `# HELP nixos_reboot_required Whether a reboot is required for reboot-relevant parts of the current NixOS system profile to become active.
# TYPE nixos_reboot_required gauge
nixos_reboot_required 1
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := prometheusOutput(tt.result)
			if got != tt.want {
				t.Fatalf("prometheusOutput() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWritePrometheusFile(t *testing.T) {
	t.Parallel()

	outputFile := filepath.Join(t.TempDir(), "nested", "collector", "nixos_reboot_required.prom")
	result := Result{RebootRequired: true}

	if err := writePrometheusFile(outputFile, result); err != nil {
		t.Fatalf("writePrometheusFile() error = %v", err)
	}

	contents, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}

	wantContents := prometheusOutput(result)
	if string(contents) != wantContents {
		t.Fatalf("output file contents = %q, want %q", contents, wantContents)
	}

	info, err := os.Stat(outputFile)
	if err != nil {
		t.Fatalf("stat output file: %v", err)
	}

	if got, want := info.Mode().Perm(), os.FileMode(0o644); got != want {
		t.Fatalf("output file mode = %v, want %v", got, want)
	}
}

func mkdir(t *testing.T, path string) string {
	t.Helper()

	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatalf("create directory %q: %v", path, err)
	}

	return path
}

func symlink(t *testing.T, target, link string) string {
	t.Helper()

	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("create symlink %q -> %q: %v", link, target, err)
	}

	return link
}

func touch(t *testing.T, path string) string {
	t.Helper()

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create file %q: %v", path, err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close file %q: %v", path, err)
	}

	return path
}

func writeFile(t *testing.T, path, contents string) string {
	t.Helper()

	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write file %q: %v", path, err)
	}

	return path
}
