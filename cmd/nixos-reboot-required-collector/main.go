package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

const (
	defaultBootedSystem  = "/run/booted-system"
	defaultCurrentSystem = "/nix/var/nix/profiles/system"
	defaultOutputFile    = "/var/lib/node_exporter/textfile_collector/nixos_reboot_required.prom"
)

var rebootRelevantPaths = []rebootRelevantPath{
	{path: "kernel", compare: resolvedPathsDifferIfBothExist},
	{path: "initrd", compare: resolvedPathsDifferIfBothExist},
	{path: "kernel-params", compare: fileContentsDifferIfBothExist},
}

type rebootRelevantPath struct {
	path    string
	compare func(leftPath, rightPath string) (bool, error)
}

type Result struct {
	RebootRequired bool
}

func main() {
	bootedSystem := flag.String(
		"booted-system",
		defaultBootedSystem,
		"Path to the booted NixOS system",
	)

	currentSystem := flag.String(
		"current-system",
		defaultCurrentSystem,
		"Path to the current NixOS system profile",
	)

	outputFile := flag.String(
		"output",
		defaultOutputFile,
		"Path to the Prometheus textfile output",
	)

	flag.Parse()

	result, err := checkRebootRequired(*bootedSystem, *currentSystem)
	if err != nil {
		log.Fatalf("failed to check reboot state: %v", err)
	}

	if err := writePrometheusFile(*outputFile, result); err != nil {
		log.Fatalf("failed to write prometheus file: %v", err)
	}
}

func checkRebootRequired(bootedSystemPath, currentSystemPath string) (Result, error) {
	bootedSystem, err := filepath.EvalSymlinks(bootedSystemPath)
	if err != nil {
		return Result{}, fmt.Errorf("resolve booted system %q: %w", bootedSystemPath, err)
	}

	currentSystem, err := filepath.EvalSymlinks(currentSystemPath)
	if err != nil {
		return Result{}, fmt.Errorf("resolve current system %q: %w", currentSystemPath, err)
	}

	for _, relevantPath := range rebootRelevantPaths {
		changed, err := relevantPath.compare(
			filepath.Join(bootedSystem, relevantPath.path),
			filepath.Join(currentSystem, relevantPath.path),
		)
		if err != nil {
			return Result{}, fmt.Errorf("check %s: %w", relevantPath.path, err)
		}

		if changed {
			return Result{
				RebootRequired: true,
			}, nil
		}
	}

	return Result{
		RebootRequired: false,
	}, nil
}

func resolvedPathsDifferIfBothExist(leftPath, rightPath string) (bool, error) {
	leftExists, err := pathExists(leftPath)
	if err != nil {
		return false, err
	}

	rightExists, err := pathExists(rightPath)
	if err != nil {
		return false, err
	}

	if !leftExists || !rightExists {
		return false, nil
	}

	leftResolved, err := resolvePath(leftPath)
	if err != nil {
		return false, fmt.Errorf("resolve %q: %w", leftPath, err)
	}

	rightResolved, err := resolvePath(rightPath)
	if err != nil {
		return false, fmt.Errorf("resolve %q: %w", rightPath, err)
	}

	return leftResolved != rightResolved, nil
}

func fileContentsDifferIfBothExist(leftPath, rightPath string) (bool, error) {
	leftExists, err := pathExists(leftPath)
	if err != nil {
		return false, err
	}

	rightExists, err := pathExists(rightPath)
	if err != nil {
		return false, err
	}

	if !leftExists || !rightExists {
		return false, nil
	}

	leftContents, err := os.ReadFile(leftPath)
	if err != nil {
		return false, fmt.Errorf("read %q: %w", leftPath, err)
	}

	rightContents, err := os.ReadFile(rightPath)
	if err != nil {
		return false, fmt.Errorf("read %q: %w", rightPath, err)
	}

	return !bytes.Equal(leftContents, rightContents), nil
}

func resolvePath(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved, nil
	}

	return "", err
}

func pathExists(path string) (bool, error) {
	_, err := os.Lstat(path)
	if err == nil {
		return true, nil
	}

	if os.IsNotExist(err) {
		return false, nil
	}

	return false, fmt.Errorf("stat %q: %w", path, err)
}

func writePrometheusFile(outputFile string, result Result) error {
	outputDir := filepath.Dir(outputFile)

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create output directory %q: %w", outputDir, err)
	}

	tmpFile, err := os.CreateTemp(outputDir, ".nixos_reboot_required_*.prom")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	tmpFileName := tmpFile.Name()
	renamed := false

	defer func() {
		if !renamed {
			_ = os.Remove(tmpFileName)
		}
	}()

	if _, err := tmpFile.WriteString(prometheusOutput(result)); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("write temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Chmod(tmpFileName, 0o644); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}

	if err := os.Rename(tmpFileName, outputFile); err != nil {
		return fmt.Errorf("rename temp file to %q: %w", outputFile, err)
	}

	renamed = true

	return nil
}

func prometheusOutput(result Result) string {
	return fmt.Sprintf(`# HELP nixos_reboot_required Whether a reboot is required for reboot-relevant parts of the current NixOS system profile to become active.
# TYPE nixos_reboot_required gauge
nixos_reboot_required %d
`,
		boolToInt(result.RebootRequired),
	)
}

func boolToInt(value bool) int {
	if value {
		return 1
	}

	return 0
}
