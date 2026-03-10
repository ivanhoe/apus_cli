# Connecting MCP Clients to Apus

The durable contract for `apus` is simple:

- your app must be running in the simulator
- Apus must be integrated into that app
- your MCP client must connect to the HTTP endpoint printed by `apus`

Default URL:

```text
http://localhost:9847/mcp
```

If you created the app with a custom port, use that port instead.

## Recommended flow

### Existing project

```bash
apus doctor --path /path/to/project --target MyApp
apus init --path /path/to/project --target MyApp
```

Then build and run the app in the simulator.

### New project

```bash
apus new MyApp
```

`apus new` already builds, launches, and waits for MCP before printing success.

## What to configure in your client

Different MCP clients change their UI and config format over time, but the important fields stay the same:

- server name: `apus`
- transport: HTTP
- URL: `http://localhost:9847/mcp`

Conceptual example:

```json
{
  "name": "apus",
  "transport": "http",
  "url": "http://localhost:9847/mcp"
}
```

Treat that JSON as an example of the connection shape, not a client-specific config file.

## Quick connectivity check

Once the client is connected, try one of these first:

- `get_logs`
- `get_screenshot`
- `get_view_hierarchy`

If those work, the MCP bridge is up.

## Common connection issues

### The client cannot reach `localhost:9847`

Check:

- the app is running right now in the simulator
- the URL includes `/mcp`
- you are using the same port that `apus` printed

### Multiple simulator apps are running

Only one app can own a given localhost port in the same simulator environment.

If needed, restart with a custom port:

```bash
apus new MyApp --port 9999
```

Then point the client at:

```text
http://localhost:9999/mcp
```

### Your client UI does not mention HTTP MCP explicitly

Client UIs move around frequently. Look for one of these terms:

- MCP server
- external tools
- remote MCP
- HTTP transport

The important part is still the same URL contract.

## After the client is connected

Typical workflow:

1. ask for logs or a screenshot
2. inspect the current view hierarchy
3. drive the UI with `ui_interact`
4. inspect network history
5. use hot reload while iterating on Swift changes

If the client connects but the tools do not behave as expected, go back to [Troubleshooting](./troubleshooting.md).
