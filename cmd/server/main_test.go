package main

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func Test_Server_prints_version_when_version_flag_provided(t *testing.T) {
	// Given
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	command := exec.CommandContext(ctx, "go", "run", ".", "--version")

	// When
	output, err := command.CombinedOutput()

	// Then
	if err != nil {
		t.Fatalf("run server with version flag: %v\n%s", err, output)
	}
	if got, want := strings.TrimSpace(string(output)), "jovepoxy 0.0.1"; got != want {
		t.Fatalf("version output = %q, want %q", got, want)
	}
}
