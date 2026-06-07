package config

import "testing"

func TestTerminalSecurityContracts(t *testing.T) {
	cases := []sourceSecurityContract{
		{
			name:       "TerminalRouteExposesInteractiveShell",
			path:       "../service/vm/terminal.go",
			vulnerable: []string{`cmd := exec.Command("/bin/sh")`},
			message:    "browser terminal should require step-up authorization before spawning an interactive root shell",
		},
		{
			name:       "TerminalPTYStartsUnscopedShell",
			path:       "../service/vm/terminal.go",
			vulnerable: []string{`ptmx, err := pty.Start(cmd)`},
			message:    "terminal PTY should be scoped to an explicit command/session policy instead of a raw shell",
		},
		{
			name:       "TerminalUpgradeLogsRawError",
			path:       "../service/vm/terminal.go",
			vulnerable: []string{`log.Errorf("failed to init websocket: %s", err)`},
			message:    "terminal websocket upgrade errors should not log raw request-derived details",
		},
		{
			name:       "TerminalStartPTYLogsRawError",
			path:       "../service/vm/terminal.go",
			vulnerable: []string{`log.Errorf("failed to start pty: %s", err)`},
			message:    "terminal PTY startup errors should be mapped to stable public logs",
		},
		{
			name:       "TerminalKillsProcessWithoutGracefulShutdown",
			path:       "../service/vm/terminal.go",
			vulnerable: []string{`_ = cmd.Process.Kill()`},
			message:    "terminal sessions should shut down child processes with a bounded graceful flow",
		},
		{
			name:       "TerminalWaitIgnoresProcessState",
			path:       "../service/vm/terminal.go",
			vulnerable: []string{`_ = cmd.Wait()`},
			message:    "terminal process wait errors should be accounted for and audited",
		},
		{
			name:       "TerminalReadDeadlineDisabled",
			path:       "../service/vm/terminal.go",
			vulnerable: []string{`var zeroTime time.Time`, `ws.SetReadDeadline(zeroTime)`},
			message:    "terminal websocket reads should have idle timeouts",
		},
		{
			name:       "TerminalBinaryMessageControlsPTYResize",
			path:       "../service/vm/terminal.go",
			vulnerable: []string{`if msgType == websocket.BinaryMessage {`, `pty.Setsize(ptmx`},
			message:    "terminal resize messages should validate dimensions before applying PTY changes",
		},
		{
			name:       "TerminalResizeAcceptsZeroRows",
			path:       "../service/vm/terminal.go",
			vulnerable: []string{`Rows: winSize.Rows`},
			message:    "terminal resize should enforce minimum and maximum rows",
		},
		{
			name:       "TerminalResizeAcceptsZeroColumns",
			path:       "../service/vm/terminal.go",
			vulnerable: []string{`Cols: winSize.Cols`},
			message:    "terminal resize should enforce minimum and maximum columns",
		},
		{
			name:       "TerminalWritesRawWebSocketPayloadToPTY",
			path:       "../service/vm/terminal.go",
			vulnerable: []string{`_, err = ptmx.Write(p)`},
			message:    "terminal input should be filtered or scoped before writing raw websocket payloads to a shell",
		},
		{
			name:       "TerminalPTYWriteLogsRawError",
			path:       "../service/vm/terminal.go",
			vulnerable: []string{`log.Errorf("failed to write to pty: %s", err)`},
			message:    "terminal PTY write errors should not log raw low-level details",
		},
		{
			name:       "TerminalFrontendAutoRunsPicocomFromURL",
			path:       "../../web/src/pages/terminal/index.tsx",
			vulnerable: []string{`setTimeout(runPicocom, 300)`},
			message:    "terminal frontend should not auto-run shell commands from URL parameters",
		},
		{
			name:       "TerminalFrontendReadsCommandArgumentsFromURL",
			path:       "../../web/src/pages/terminal/index.tsx",
			vulnerable: []string{`const searchParams = new URLSearchParams(urls[1])`},
			message:    "terminal frontend should not derive command arguments from URL query parameters",
		},
		{
			name:       "TerminalFrontendSendsPicocomCommandString",
			path:       "../../web/src/pages/terminal/index.tsx",
			vulnerable: []string{`ws.send(`, `picocom ${port} --baud ${baud}`},
			message:    "terminal frontend should request serial sessions through a typed API instead of sending shell text",
		},
		{
			name:       "TerminalFrontendSendsControlBytesOnUnload",
			path:       "../../web/src/pages/terminal/index.tsx",
			vulnerable: []string{`ws.send('\x01\x18')`},
			message:    "terminal frontend should not send raw terminal control bytes during cleanup",
		},
		{
			name:       "TerminalFrontendOpensAmbientCookieWebSocket",
			path:       "../../web/src/pages/terminal/index.tsx",
			vulnerable: []string{`const ws = new WebSocket(url)`},
			message:    "terminal websocket should use explicit session authorization instead of ambient cookies",
		},
	}

	runSourceSecurityContracts(t, cases, 17, "terminal")
}
