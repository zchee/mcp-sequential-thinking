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
	"strconv"
	"sync/atomic"
	"testing"
)

func BenchmarkProcessThought_NoBranch(b *testing.B) {
	server := NewSequentialThinkingServer()
	input := ThoughtData{
		Thought:       "bench",
		ThoughtNumber: 1,
		TotalThoughts: 1,
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _, err := server.ProcessThought(b.Context(), nil, input)
		if err != nil {
			b.Fatalf("process thought: %v", err)
		}
	}
}

func BenchmarkProcessThought_BranchInsert(b *testing.B) {
	server := NewSequentialThinkingServer()
	branchIDs := make([]string, b.N)
	for i := range branchIDs {
		branchIDs[i] = strconv.Itoa(i)
	}
	input := ThoughtData{
		Thought:           "bench",
		ThoughtNumber:     1,
		TotalThoughts:     1,
		BranchFromThought: -1,
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		input.BranchId = branchIDs[i]
		_, _, err := server.ProcessThought(b.Context(), nil, input)
		if err != nil {
			b.Fatalf("process thought: %v", err)
		}
	}
}

func BenchmarkProcessThought_Parallel(b *testing.B) {
	server := NewSequentialThinkingServer()
	input := ThoughtData{
		Thought:       "bench",
		ThoughtNumber: 1,
		TotalThoughts: 1,
	}
	var counter uint64
	var errCount uint64

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		local := input
		for pb.Next() {
			local.ThoughtNumber = int(atomic.AddUint64(&counter, 1))
			_, _, err := server.ProcessThought(b.Context(), nil, local)
			if err != nil {
				atomic.AddUint64(&errCount, 1)
				return
			}
		}
	})
	if errCount > 0 {
		b.Fatalf("process thought errors: %d", errCount)
	}
}
