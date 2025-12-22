# Sequential Thinking MCP Server (Go)

A Go implementation of the Model Context Protocol (MCP) sequential thinking tool for structured, reflective problem-solving.

## Features

- Step-by-step thinking with revisions and branching
- Dynamic adjustment of total thought count
- Optional thought logging
- Stdio or streamable HTTP transport

## Tool

### sequentialthinking

Facilitates a detailed, step-by-step thinking process for problem-solving and analysis.

Inputs:
- `thought` (string): Current thinking step
- `nextThoughtNeeded` (bool): Whether another thought step is needed
- `thoughtNumber` (int): Current thought number (>= 1)
- `totalThoughts` (int): Estimated total thoughts (>= 1)
- `isRevision` (bool, optional): Whether this revises a previous thought
- `revisesThought` (int, optional): Which thought is being reconsidered
- `branchFromThought` (int, optional): Branching point thought number
- `branchId` (string, optional): Branch identifier
- `needsMoreThoughts` (bool, optional): Whether more thoughts are needed

Outputs:
- `thoughtNumber` (int)
- `totalThoughts` (int)
- `nextThoughtNeeded` (bool)
- `branches` ([]string): Known branch IDs
- `thoughtHistoryLength` (int)

## Usage

The sequential thinking tool is designed for:
- Breaking down complex problems into steps
- Planning and design with room for revision
- Analysis that might need course correction
- Problems where the full scope might not be clear initially
- Tasks that need to maintain context over multiple steps
- Situations where irrelevant information needs to be filtered out

## Build and Run

Requirements:
- Go 1.25+

Build:

```bash
go install github.com/zchee/mcp-sequential-thinking@latest
```

Run over stdio (default):

```bash
mcp-sequential-thinking
```

Run with streamable HTTP:

```bash
mcp-sequential-thinking -http 127.0.0.1:8080
```

Enable tool logging:
- Use `-logpath /path/to/log.txt` to write server logs.
- Set `ENABLE_SEQUENTIA_LTHINKING_LOG=true` to print formatted thought frames to stderr.

## Client Configuration

This server uses stdio by default. Configure your MCP client to run the built binary.

Claude Desktop example (`claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "sequential-thinking": {
      "command": "mcp-sequential-thinking",
      "args": []
    }
  }
}
```

VS Code example (`mcp.json`):

```json
{
  "servers": {
    "sequential-thinking": {
      "command": "mcp-sequential-thinking",
      "args": []
    }
  }
}
```

If your client supports streamable HTTP, run the server with `-http` and point the client at the address.

## Project Layout

- `main.go`: server setup, transport selection, CLI flags
- `server.go`: sequential thinking tool implementation

## Development

```bash
go test ./...
```

## License

Apache-2.0. See `LICENSE`.
