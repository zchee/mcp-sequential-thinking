// Copyright 2025 The mcp-sequential-thinking Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func restoreRunGlobals(t *testing.T) func() {
	t.Helper()

	oldAddr := flagHTTPAddr
	oldLogPath := flagLogPath
	oldLogger := slog.Default()

	return func() {
		flagHTTPAddr = oldAddr
		flagLogPath = oldLogPath
		slog.SetDefault(oldLogger)
	}
}

func TestPtrInt(t *testing.T) {
	tests := map[string]struct {
		value int
	}{
		"success: non-zero": {
			value: 42,
		},
		"success: zero": {
			value: 0,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := ptr(tt.value)
			if diff := cmp.Diff(true, got != nil); diff != "" {
				t.Fatalf("pointer nil mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.value, *got); diff != "" {
				t.Fatalf("value mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPtrString(t *testing.T) {
	tests := map[string]struct {
		value string
	}{
		"success: non-empty": {
			value: "value",
		},
		"success: empty": {
			value: "",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := ptr(tt.value)
			if diff := cmp.Diff(true, got != nil); diff != "" {
				t.Fatalf("pointer nil mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.value, *got); diff != "" {
				t.Fatalf("value mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRunInvalidHTTPAddr(t *testing.T) {
	tests := map[string]struct {
		addr       string
		wantSubstr string
	}{
		"error: invalid address": {
			addr:       "127.0.0.1:99999",
			wantSubstr: "serve sequential thinking mcp http server",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Cleanup(restoreRunGlobals(t))

			flagHTTPAddr = tt.addr
			flagLogPath = ""

			err := run()
			if diff := cmp.Diff(true, err != nil); diff != "" {
				t.Fatalf("error presence mismatch (-want +got):\n%s", diff)
			}
			if err != nil {
				if diff := cmp.Diff(true, strings.Contains(err.Error(), tt.wantSubstr)); diff != "" {
					t.Fatalf("error text mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestRunLogPathOpenError(t *testing.T) {
	t.Cleanup(restoreRunGlobals(t))

	flagHTTPAddr = ""
	flagLogPath = t.TempDir()

	err := run()
	if diff := cmp.Diff(true, err != nil); diff != "" {
		t.Fatalf("error presence mismatch (-want +got):\n%s", diff)
	}
	if err != nil {
		if diff := cmp.Diff(true, strings.Contains(err.Error(), "open")); diff != "" {
			t.Fatalf("error text mismatch (-want +got):\n%s", diff)
		}
	}
}

func TestRunStdioInvalidInput(t *testing.T) {
	t.Cleanup(restoreRunGlobals(t))

	tmpDir := t.TempDir()
	stdinFile, err := os.CreateTemp(tmpDir, "stdin")
	if err != nil {
		t.Fatalf("create stdin temp file: %v", err)
	}
	if _, err := stdinFile.WriteString("not-json"); err != nil {
		t.Fatalf("write stdin temp file: %v", err)
	}
	if _, err := stdinFile.Seek(0, 0); err != nil {
		t.Fatalf("seek stdin temp file: %v", err)
	}
	stdoutFile, err := os.CreateTemp(tmpDir, "stdout")
	if err != nil {
		t.Fatalf("create stdout temp file: %v", err)
	}

	oldStdin := os.Stdin
	oldStdout := os.Stdout
	t.Cleanup(func() {
		os.Stdin = oldStdin
		os.Stdout = oldStdout
		if closeErr := stdinFile.Close(); closeErr != nil && !errors.Is(closeErr, os.ErrClosed) {
			t.Errorf("close stdin temp file: %v", closeErr)
		}
		if closeErr := stdoutFile.Close(); closeErr != nil && !errors.Is(closeErr, os.ErrClosed) {
			t.Errorf("close stdout temp file: %v", closeErr)
		}
	})

	os.Stdin = stdinFile
	os.Stdout = stdoutFile

	flagHTTPAddr = ""
	flagLogPath = filepath.Join(tmpDir, "server.log")

	err = run()
	if diff := cmp.Diff(true, err != nil); diff != "" {
		t.Fatalf("error presence mismatch (-want +got):\n%s", diff)
	}
	if err != nil {
		if diff := cmp.Diff(true, strings.Contains(err.Error(), "serve sequential thinking mcp stdio server")); diff != "" {
			t.Fatalf("error text mismatch (-want +got):\n%s", diff)
		}
	}
	if _, statErr := os.Stat(flagLogPath); statErr != nil {
		t.Fatalf("expected log file to exist: %v", statErr)
	}
}

func TestMainHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	main()
}

func TestMainExitsOnRunError(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=TestMainHelperProcess", "-http=127.0.0.1:99999")
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")

	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit status")
	}
	exitErr, ok := err.(*exec.ExitError)
	if diff := cmp.Diff(true, ok); diff != "" {
		t.Fatalf("exit error type mismatch (-want +got):\n%s", diff)
	}
	if ok {
		if diff := cmp.Diff(true, exitErr.ExitCode() != 0); diff != "" {
			t.Fatalf("exit code mismatch (-want +got):\n%s", diff)
		}
	}
	if diff := cmp.Diff(true, strings.Contains(string(output), "serve sequential thinking mcp http server")); diff != "" {
		t.Fatalf("output text mismatch (-want +got):\n%s", diff)
	}
}
