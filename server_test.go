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
	"encoding/json/jsontext"
	"encoding/json/v2"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const loggingEnvKey = "ENABLE_SEQUENTIA_LTHINKING_LOG"

func decodeOutput(t *testing.T, text string) Output {
	t.Helper()

	dec := jsontext.NewDecoder(strings.NewReader(text))
	var out Output
	if err := json.UnmarshalDecode(dec, &out); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	return out
}

func resultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()

	if diff := cmp.Diff(true, result != nil); diff != "" {
		t.Fatalf("result nil mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(1, len(result.Content)); diff != "" {
		t.Fatalf("content length mismatch (-want +got):\n%s", diff)
	}
	content, ok := result.Content[0].(*mcp.TextContent)
	if diff := cmp.Diff(true, ok); diff != "" {
		t.Fatalf("content type mismatch (-want +got):\n%s", diff)
	}
	return content.Text
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	original := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stderr: %v", err)
	}
	os.Stderr = w
	defer func() {
		os.Stderr = original
	}()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("close reader: %v", err)
	}
	return string(data)
}

func TestNewSequentialThinkingServer(t *testing.T) {
	tests := map[string]struct {
		envValue        string
		wantLogging     bool
		wantHistorySize int
		wantBranchSize  int
		wantNilHistory  bool
		wantNilBranches bool
	}{
		"default: logging disabled": {
			envValue:        "",
			wantLogging:     false,
			wantHistorySize: 0,
			wantBranchSize:  0,
			wantNilHistory:  false,
			wantNilBranches: false,
		},
		"enabled: logging enabled": {
			envValue:        "true",
			wantLogging:     true,
			wantHistorySize: 0,
			wantBranchSize:  0,
			wantNilHistory:  false,
			wantNilBranches: false,
		},
		"invalid: logging disabled": {
			envValue:        "not-bool",
			wantLogging:     false,
			wantHistorySize: 0,
			wantBranchSize:  0,
			wantNilHistory:  false,
			wantNilBranches: false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Setenv(loggingEnvKey, tt.envValue)
			server := NewSequentialThinkingServer()

			if diff := cmp.Diff(tt.wantLogging, server.enableThoughtLogging); diff != "" {
				t.Fatalf("logging flag mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantHistorySize, len(server.thoughtHistory)); diff != "" {
				t.Fatalf("history size mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantBranchSize, len(server.branches)); diff != "" {
				t.Fatalf("branch size mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantNilHistory, server.thoughtHistory == nil); diff != "" {
				t.Fatalf("history nil mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantNilBranches, server.branches == nil); diff != "" {
				t.Fatalf("branches nil mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSequentialThinkingServerValidateThoughtData(t *testing.T) {
	server := NewSequentialThinkingServer()

	tests := map[string]struct {
		input    ThoughtData
		wantErr  bool
		wantText string
	}{
		"error: empty thought": {
			input: ThoughtData{
				Thought:       "",
				ThoughtNumber: 1,
				TotalThoughts: 1,
			},
			wantErr:  true,
			wantText: "invalid thought: must be a string",
		},
		"error: invalid thoughtNumber": {
			input: ThoughtData{
				Thought:       "ok",
				ThoughtNumber: 0,
				TotalThoughts: 1,
			},
			wantErr:  true,
			wantText: "invalid thoughtNumber: must be a number > 0",
		},
		"error: invalid totalThoughts": {
			input: ThoughtData{
				Thought:       "ok",
				ThoughtNumber: 1,
				TotalThoughts: 0,
			},
			wantErr:  true,
			wantText: "invalid totalThoughts: must be a number > 0",
		},
		"success: valid input": {
			input: ThoughtData{
				Thought:       "ok",
				ThoughtNumber: 1,
				TotalThoughts: 2,
			},
			wantErr:  false,
			wantText: "",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := server.validateThoughtData(tt.input)
			if diff := cmp.Diff(tt.wantErr, err != nil); diff != "" {
				t.Fatalf("error presence mismatch (-want +got):\n%s", diff)
			}
			if err != nil {
				if diff := cmp.Diff(tt.wantText, err.Error()); diff != "" {
					t.Fatalf("error text mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestSequentialThinkingServerFormatThought(t *testing.T) {
	server := NewSequentialThinkingServer()

	tests := map[string]struct {
		input        ThoughtData
		wantContains []string
	}{
		"format: revision": {
			input: ThoughtData{
				Thought:        "revise",
				ThoughtNumber:  1,
				TotalThoughts:  2,
				IsRevision:     true,
				RevisesThought: -2,
			},
			wantContains: []string{"Revision", "revising thought -2", "revise"},
		},
		"format: branch": {
			input: ThoughtData{
				Thought:           "branch",
				ThoughtNumber:     2,
				TotalThoughts:     3,
				BranchFromThought: -1,
				BranchId:          "b1",
			},
			wantContains: []string{"Branch", "from thought -1, ID: b1", "branch"},
		},
		"format: default": {
			input: ThoughtData{
				Thought:       "think",
				ThoughtNumber: 3,
				TotalThoughts: 3,
			},
			wantContains: []string{"Thought", "think"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := server.formatThought(tt.input)
			for _, want := range tt.wantContains {
				if diff := cmp.Diff(true, strings.Contains(got, want)); diff != "" {
					t.Fatalf("expected content missing (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestSequentialThinkingServerProcessThoughtValidation(t *testing.T) {
	server := NewSequentialThinkingServer()

	tests := map[string]struct {
		input   ThoughtData
		wantErr string
	}{
		"error: invalid thought": {
			input: ThoughtData{
				Thought:       "",
				ThoughtNumber: 1,
				TotalThoughts: 1,
			},
			wantErr: "invalid thought: must be a string",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			result, _, err := server.ProcessThought(t.Context(), nil, tt.input)
			if diff := cmp.Diff(true, err != nil); diff != "" {
				t.Fatalf("error presence mismatch (-want +got):\n%s", diff)
			}
			if err != nil {
				if diff := cmp.Diff(tt.wantErr, err.Error()); diff != "" {
					t.Fatalf("error text mismatch (-want +got):\n%s", diff)
				}
			}
			if diff := cmp.Diff(true, result == nil); diff != "" {
				t.Fatalf("result nil mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSequentialThinkingServerProcessThoughtSuccess(t *testing.T) {
	tests := map[string]struct {
		inputs      []ThoughtData
		wantOutputs []Output
	}{
		"success: branches sorted and history tracked": {
			inputs: []ThoughtData{
				{
					Thought:           "first",
					NextThoughtNeeded: true,
					ThoughtNumber:     2,
					TotalThoughts:     1,
					BranchFromThought: -1,
					BranchId:          "b",
				},
				{
					Thought:           "second",
					NextThoughtNeeded: false,
					ThoughtNumber:     3,
					TotalThoughts:     3,
					BranchFromThought: -2,
					BranchId:          "a",
				},
			},
			wantOutputs: []Output{
				{
					ThoughtNumber:        2,
					TotalThoughts:        2,
					NextThoughtNeeded:    true,
					Branches:             []string{"b"},
					ThoughtHistoryLength: 1,
				},
				{
					ThoughtNumber:        3,
					TotalThoughts:        3,
					NextThoughtNeeded:    false,
					Branches:             []string{"a", "b"},
					ThoughtHistoryLength: 2,
				},
			},
		},
		"success: no branch recorded when branchFromThought non-negative": {
			inputs: []ThoughtData{
				{
					Thought:           "third",
					NextThoughtNeeded: false,
					ThoughtNumber:     1,
					TotalThoughts:     1,
					BranchFromThought: 1,
					BranchId:          "ignored",
				},
			},
			wantOutputs: []Output{
				{
					ThoughtNumber:        1,
					TotalThoughts:        1,
					NextThoughtNeeded:    false,
					Branches:             nil,
					ThoughtHistoryLength: 1,
				},
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			server := NewSequentialThinkingServer()

			for i, input := range tt.inputs {
				result, _, err := server.ProcessThought(t.Context(), nil, input)
				if err != nil {
					t.Fatalf("process thought: %v", err)
				}
				got := decodeOutput(t, resultText(t, result))
				if diff := cmp.Diff(tt.wantOutputs[i], got); diff != "" {
					t.Fatalf("output mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestSequentialThinkingServerProcessThoughtLogging(t *testing.T) {
	tests := map[string]struct {
		input        ThoughtData
		wantContains []string
	}{
		"success: logging writes formatted thought": {
			input: ThoughtData{
				Thought:       "log this",
				ThoughtNumber: 1,
				TotalThoughts: 1,
			},
			wantContains: []string{"log this", "Thought"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Setenv(loggingEnvKey, "true")
			server := NewSequentialThinkingServer()
			var result *mcp.CallToolResult
			var err error

			output := captureStderr(t, func() {
				result, _, err = server.ProcessThought(t.Context(), nil, tt.input)
			})

			if err != nil {
				t.Fatalf("process thought: %v", err)
			}
			if diff := cmp.Diff(true, result != nil); diff != "" {
				t.Fatalf("result nil mismatch (-want +got):\n%s", diff)
			}
			for _, want := range tt.wantContains {
				if diff := cmp.Diff(true, strings.Contains(output, want)); diff != "" {
					t.Fatalf("logging output missing content (-want +got):\n%s", diff)
				}
			}
		})
	}
}
