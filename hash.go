package main

import (
	"fmt"
	"os/exec"
	"strings"
)

func nixHash(hashType, path string, flat bool) (string, error) {
	args := []string{"--type", hashType, "--base32"}
	if flat {
		args = append(args, "--flat")
	}
	args = append(args, path)

	cmd := exec.Command("nix-hash", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("nix-hash failed: %w\noutput: %s", err, string(out))
	}

	hash := strings.TrimSpace(string(out))
	if hash == "" {
		return "", fmt.Errorf("nix-hash returned empty result")
	}

	return hash, nil
}
