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
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/bytedance/sonic"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ThoughtData represents the input data for a thought.
type ThoughtData struct {
	Thought           string `json:"thought" jsonschema:"Your current thinking step"`
	NextThoughtNeeded bool   `json:"nextThoughtNeeded" jsonschema:"Whether another thought step is needed"`
	ThoughtNumber     int    `json:"thoughtNumber" jsonschema:"Current thought number (numeric value, e.g., 1, 2, 3)"`
	TotalThoughts     int    `json:"totalThoughts" jsonschema:"Estimated total thoughts needed (numeric value, e.g., 5, 10)"`
	IsRevision        bool   `json:"isRevision,omitzero" jsonschema:"Estimated Whether this revises previous thinking"`
	RevisesThought    int    `json:"revisesThought,omitzero" jsonschema:"Which thought is being reconsidered"`
	BranchFromThought int    `json:"branchFromThought,omitzero" jsonschema:"Branching point thought number"`
	BranchId          string `json:"branchId,omitzero" jsonschema:"Branch identifier"`
	NeedsMoreThoughts bool   `json:"needsMoreThoughts,omitzero" jsonschema:"If more thoughts are needed"`
}

// Output represents the output data for a thought.
type Output struct {
	ThoughtNumber        int      `json:"thoughtNumber"`
	TotalThoughts        int      `json:"totalThoughts"`
	NextThoughtNeeded    bool     `json:"nextThoughtNeeded"`
	Branches             []string `json:"branches"`
	ThoughtHistoryLength int      `json:"thoughtHistoryLength"`
}

// SequentialThinkingServer implements the sequential thinking logic.
type SequentialThinkingServer struct {
	thoughtHistory       []struct{}
	branches             map[string]struct{}
	branchKeys           []string
	enableThoughtLogging bool
	mu                   sync.Mutex
}

// NewSequentialThinkingServer creates a new instance of the server.
func NewSequentialThinkingServer() *SequentialThinkingServer {
	enableLogging := false
	val := os.Getenv("ENABLE_SEQUENTIA_LTHINKING_LOG")
	if ok, err := strconv.ParseBool(val); err == nil && ok {
		enableLogging = true
	}

	return &SequentialThinkingServer{
		thoughtHistory:       make([]struct{}, 0),
		branches:             make(map[string]struct{}),
		enableThoughtLogging: enableLogging,
	}
}

// validateThoughtData validates the input thought data.
func (s *SequentialThinkingServer) validateThoughtData(input ThoughtData) error {
	if input.Thought == "" {
		return errors.New("invalid thought: must be a string")
	}
	if input.ThoughtNumber <= 0 {
		return errors.New("invalid thoughtNumber: must be a number > 0")
	}
	if input.TotalThoughts <= 0 {
		return errors.New("invalid totalThoughts: must be a number > 0")
	}
	return nil
}

// formatThought formats the thought for logging.
func (s *SequentialThinkingServer) formatThought(thoughtData ThoughtData) string {
	// Plain text components
	prefixText := ""
	context := ""

	switch {
	case thoughtData.IsRevision:
		prefixText = "🔄 Revision"
		if thoughtData.RevisesThought < 0 {
			context = fmt.Sprintf(" (revising thought %d)", thoughtData.RevisesThought)
		}

	case thoughtData.BranchFromThought < 0:
		prefixText = "🌿 Branch"
		branchID := ""
		if thoughtData.BranchId != "" {
			branchID = thoughtData.BranchId
		}
		context = fmt.Sprintf(" (from thought %d, ID: %s)", thoughtData.BranchFromThought, branchID)

	default:
		prefixText = "💭 Thought"
		context = ""
	}

	headerContent := fmt.Sprintf("%s %d/%d%s", prefixText, thoughtData.ThoughtNumber, thoughtData.TotalThoughts, context)

	// Colors
	const (
		yellow = `\033[33m`
		green  = `\033[32m`
		blue   = `\033[34m`
		reset  = `\033[0m`
	)

	coloredPrefix := ""
	switch {
	case thoughtData.IsRevision:
		coloredPrefix = yellow + prefixText + reset
	case thoughtData.BranchFromThought < 0:
		coloredPrefix = green + prefixText + reset
	default:
		coloredPrefix = blue + prefixText + reset
	}

	// Reconstruct header with colors, but use headerContent length for layout
	coloredHeader := strings.Replace(headerContent, prefixText, coloredPrefix, 1)

	borderLen := int(math.Max(float64(len(headerContent)), float64(len(thoughtData.Thought)))) + 4
	border := strings.Repeat("─", borderLen)

	return fmt.Sprintf(`
┌%s┐
│ %s%s │
├%s┤
│ %s%s │
└%s┘`,
		border,
		coloredHeader,
		strings.Repeat(" ", borderLen-len(headerContent)-2),
		border,
		thoughtData.Thought,
		strings.Repeat(" ", borderLen-len(thoughtData.Thought)-2),
		border,
	)
}

// ProcessThought processes a thought request.
func (s *SequentialThinkingServer) ProcessThought(ctx context.Context, request *mcp.CallToolRequest, input ThoughtData) (*mcp.CallToolResult, any, error) {
	if err := s.validateThoughtData(input); err != nil {
		return nil, nil, err
	}

	if input.ThoughtNumber > input.TotalThoughts {
		input.TotalThoughts = input.ThoughtNumber
	}

	var (
		branchesSnapshot []string
		historyLen       int
	)

	s.mu.Lock()
	s.thoughtHistory = append(s.thoughtHistory, struct{}{})

	if input.BranchFromThought < 0 && input.BranchId != "" {
		branchID := input.BranchId
		if _, exists := s.branches[branchID]; !exists {
			s.branches[branchID] = struct{}{}
			insertAt := sort.SearchStrings(s.branchKeys, branchID)
			if insertAt == len(s.branchKeys) {
				s.branchKeys = append(s.branchKeys, branchID)
			} else if s.branchKeys[insertAt] != branchID {
				s.branchKeys = append(s.branchKeys, "")
				copy(s.branchKeys[insertAt+1:], s.branchKeys[insertAt:])
				s.branchKeys[insertAt] = branchID
			}
		}
	}

	historyLen = len(s.thoughtHistory)
	if len(s.branchKeys) > 0 {
		branchesSnapshot = append([]string(nil), s.branchKeys...)
	}

	s.mu.Unlock()

	if s.enableThoughtLogging {
		formatted := s.formatThought(input)
		fmt.Fprintln(os.Stderr, formatted)
	}

	// Prepare response
	output := Output{
		ThoughtNumber:        input.ThoughtNumber,
		TotalThoughts:        input.TotalThoughts,
		NextThoughtNeeded:    input.NextThoughtNeeded,
		Branches:             branchesSnapshot,
		ThoughtHistoryLength: historyLen,
	}

	data, err := sonic.ConfigFastest.MarshalToString(&output)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal response: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: data,
			},
		},
	}, nil, nil
}
