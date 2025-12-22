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

// Command mcp-sequential-thinking is a MCP server for sequential thinking and problem solving.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/zchee/dumper"
)

var Version = "0.0.1"

const description = `A detailed tool for dynamic and reflective problem-solving through thoughts.
This tool helps analyze problems through a flexible thinking process that can adapt and evolve.
Each thought can build on, question, or revise previous insights as understanding deepens.

When to use this tool:
- Breaking down complex problems into steps
- Planning and design with room for revision
- Analysis that might need course correction
- Problems where the full scope might not be clear initially
- Problems that require a multi-step solution
- Tasks that need to maintain context over multiple steps
- Situations where irrelevant information needs to be filtered out

Key features:
- You can adjust total_thoughts up or down as you progress
- You can question or revise previous thoughts
- You can add more thoughts even after reaching what seemed like the end
- You can express uncertainty and explore alternative approaches
- Not every thought needs to build linearly - you can branch or backtrack
- Generates a solution hypothesis
- Verifies the hypothesis based on the Chain of Thought steps
- Repeats the process until satisfied
- Provides a correct answer

Parameters explained:
- thought (string): Required. Your current thinking step, which can include:
  * Regular analytical steps
  * Revisions of previous thoughts
  * Questions about previous decisions
  * Realizations about needing more analysis
  * Changes in approach
  * Hypothesis generation
  * Hypothesis verification
- nextThoughtNeeded (boolean): Required. True if you need more thinking, even if at what seemed like the end
- thoughtNumber (integer): Required. Current number in sequence (can go beyond initial total if needed)
- totalThoughts (integer): Required. Current estimate of thoughts needed (can be adjusted up/down)
- isRevision (boolean): Optional. A boolean indicating if this thought revises previous thinking
- revisesThought (integer): Optional. If is_revision is true, which thought number is being reconsidered
- branchFromThought (integer): Optional. If branching, which thought number is the branching point
- branchId (string): Optional. Identifier for the current branch (if any)
- needsMoreThoughts (boolean): Optional. If reaching end but realizing more thoughts needed

You should:
1. Start with an initial estimate of needed thoughts, but be ready to adjust
2. Feel free to question or revise previous thoughts
3. Don't hesitate to add more thoughts if needed, even at the "end"
4. Express uncertainty when present
5. Mark thoughts that revise previous thinking or branch into new paths
6. Ignore information that is irrelevant to the current step
7. Generate a solution hypothesis when appropriate
8. Verify the hypothesis based on the Chain of Thought steps
9. Repeat the process until satisfied with the solution
10. Provide a single, ideally correct answer as the final output
11. Only set nextThoughtNeeded to false when truly done and a satisfactory answer is reached`

func ptr[T any](v T) *T {
	return &v
}

var (
	flagHTTPAddr string
	flagLogPath  string
)

func init() {
	uuid.EnableRandPool()

	flag.StringVar(&flagHTTPAddr, "http", "", "if set, use streamable HTTP at this address, instead of stdin/stdout")
	flag.StringVar(&flagLogPath, "logpath", "", "if set, enable sequential thinking tool logging")
}

func main() {
	flag.Parse()

	dumper.Config = dumper.ConfigState{
		Indent:       " ",
		NumericWidth: 1,
		StringWidth:  1,
		BytesWidth:   16,
		CommentBytes: true,
		OmitZero:     true,
	}

	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	var f io.WriteCloser

	handler := slog.DiscardHandler
	if flagLogPath != "" {
		var err error
		f, err = os.OpenFile(flagLogPath, os.O_RDWR|os.O_CREATE, 0o666)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()

		handler = slog.NewTextHandler(f, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
	}
	logger := slog.New(handler)
	slog.SetDefault(logger)

	srvImpl := &mcp.Implementation{
		Name:       "sequential-thinking",
		Version:    Version,
		WebsiteURL: "",
	}
	opts := &mcp.ServerOptions{
		// TODO(zchee): The [mcp.ServerOptions.Instructions] are usually enough tool description, but set a global prompt such as "Think step by step"
		// Instructions: `Based on the previous thinking, analyze the step-by-step and try to think more about the critical points.`,
		Logger:   logger,
		HasTools: true,
		GetSessionID: func() string {
			// Use UUID instead of [mcp.randText]
			return uuid.NewString()
		},
	}
	srv := mcp.NewServer(srvImpl, opts)

	inputSchema, err := jsonschema.For[ThoughtData](&jsonschema.ForOptions{})
	if err != nil {
		return fmt.Errorf("parse ThoughtData: %w", err)
	}
	inputSchema.Properties["thoughtNumber"].Minimum = ptr(float64(1))
	inputSchema.Properties["totalThoughts"].Minimum = ptr(float64(1))
	inputSchema.Properties["revisesThought"].Minimum = ptr(float64(1))
	inputSchema.Properties["branchFromThought"].Minimum = ptr(float64(1))
	// dumper.Dump(inputSchema)

	outputSchema, err := jsonschema.For[Output](&jsonschema.ForOptions{})
	if err != nil {
		log.Fatal(err)
	}
	// dumper.Dump(outputSchema)

	sequentialThinkingTool := &mcp.Tool{
		Name:         "sequentialthinking",
		Description:  description,
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
	}
	sequentialThinkServer := NewSequentialThinkingServer()

	mcp.AddTool(srv, sequentialThinkingTool, sequentialThinkServer.ProcessThought)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if flagHTTPAddr != "" {
		mcpServer := func(*http.Request) *mcp.Server {
			return srv
		}
		handler := mcp.NewStreamableHTTPHandler(mcpServer, nil)
		httpSrv := &http.Server{
			Addr:    flagHTTPAddr,
			Handler: handler,
			BaseContext: func(net.Listener) context.Context {
				return ctx
			},
		}
		logger.InfoContext(ctx, "sequential thinking MCP server running", slog.String("addr", "http://"+flagHTTPAddr))
		if err := httpSrv.ListenAndServe(); err != nil {
			logger.ErrorContext(ctx, "serve sequential thinking mcp http server", slog.Any("error", err))
			return fmt.Errorf("serve sequential thinking mcp http server: %w", err)
		}
	}

	tr := mcp.Transport(&mcp.StdioTransport{})
	if flagLogPath != "" {
		tr = &mcp.LoggingTransport{
			Transport: tr,
			Writer:    f,
		}
	}

	logger.InfoContext(ctx, "sequential thinking mcp server running on stdio")
	if err := srv.Run(ctx, tr); err != nil {
		logger.ErrorContext(ctx, "serve sequential thinking mcp stdio server", slog.Any("error", err))
		os.Exit(1)
	}

	return nil
}
