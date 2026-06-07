package config

import "testing"

func TestStreamWebSocketSecurityContracts(t *testing.T) {
	cases := []sourceSecurityContract{
		{
			name:       "WebRTCUpgradeLogsClientIP",
			path:       "../service/stream/webrtc/h264.go",
			vulnerable: []string{`log.Debugf("h264 websocket connected: %s", c.ClientIP())`},
			message:    "WebRTC stream logs should not record client IPs on authenticated media connections",
		},
		{
			name:       "WebRTCDisconnectLogsClientIP",
			path:       "../service/stream/webrtc/h264.go",
			vulnerable: []string{`log.Debugf("h264 websocket disconnected: %s", c.ClientIP())`},
			message:    "WebRTC disconnect logs should not record client IPs on authenticated media connections",
		},
		{
			name:       "WebRTCReadDeadlineDisabled",
			path:       "../service/stream/webrtc/h264.go",
			vulnerable: []string{`var zeroTime time.Time`, `wsConn.SetReadDeadline(zeroTime)`},
			message:    "WebRTC signaling websocket should have idle and handshake deadlines",
		},
		{
			name:       "WebRTCSTUNUsesPlainScheme",
			path:       "../service/stream/webrtc/h264.go",
			vulnerable: []string{`URLs: []string{"stun:" + conf.Stun}`},
			message:    "WebRTC STUN config should validate and constrain configured ICE URLs",
		},
		{
			name:       "WebRTCTURNSendsCredentialToClient",
			path:       "../service/stream/webrtc/h264.go",
			vulnerable: []string{`Credential: conf.Turn.TurnCred`},
			message:    "TURN credentials should be ephemeral and scoped before sending to browser clients",
		},
		{
			name:       "WebRTCICEServersSerializeCredential",
			path:       "../service/stream/webrtc/h264.go",
			vulnerable: []string{`Credential interface{} ` + "`json:\"credential,omitempty\"`"},
			message:    "ICE server responses should not expose long-lived TURN credentials",
		},
		{
			name:       "WebRTCMarshalICEServersToClient",
			path:       "../service/stream/webrtc/h264.go",
			vulnerable: []string{`data, err := json.Marshal(clientServers)`},
			message:    "ICE server serialization should redact or mint temporary credentials",
		},
		{
			name:       "WebRTCPlayoutDelayUsesPlainHTTPURI",
			path:       "../service/stream/webrtc/h264.go",
			vulnerable: []string{`URI: "http://www.webrtc.org/experiments/rtp-hdrext/playout-delay"`},
			message:    "WebRTC extension URIs should be pinned through constants and reviewed as protocol identifiers",
		},
		{
			name:       "WebRTCClientLogsRawReadErrors",
			path:       "../service/stream/webrtc/client.go",
			vulnerable: []string{`log.Errorf("failed to read message: %v", err)`},
			message:    "WebRTC signaling read errors should not log raw websocket details",
		},
		{
			name:       "WebRTCClientIgnoresMalformedJSON",
			path:       "../service/stream/webrtc/client.go",
			vulnerable: []string{`log.Errorf("failed to unmarshal message: %v", err)`, `return nil, nil`},
			message:    "WebRTC signaling should close or reject malformed JSON instead of ignoring it",
		},
		{
			name:       "WebRTCClientLogsOutboundEventNames",
			path:       "../service/stream/webrtc/client.go",
			vulnerable: []string{`log.Debugf("sent message %s", event)`},
			message:    "WebRTC signaling should avoid logging event names from authenticated sessions",
		},
		{
			name:       "WebRTCSignalingLogsUnhandledEvents",
			path:       "../service/stream/webrtc/signaling.go",
			vulnerable: []string{`log.Debugf("Unhandled message event: %s", message.Event)`},
			message:    "WebRTC signaling should not log untrusted event names",
		},
		{
			name:       "WebRTCOfferLogsRawUnmarshalError",
			path:       "../service/stream/webrtc/signaling.go",
			vulnerable: []string{`log.Errorf("failed to unmarshal video offer: %s", err)`},
			message:    "WebRTC offer parsing should map errors without logging raw parse details",
		},
		{
			name:       "WebRTCCandidateLogsRawUnmarshalError",
			path:       "../service/stream/webrtc/signaling.go",
			vulnerable: []string{`log.Errorf("failed to unmarshal candidate: %s", err)`},
			message:    "WebRTC candidate parsing should map errors without logging raw parse details",
		},
		{
			name:       "WebRTCCandidateAcceptsRawJSONCandidate",
			path:       "../service/stream/webrtc/signaling.go",
			vulnerable: []string{`json.Unmarshal([]byte(data), &candidate)`},
			message:    "WebRTC candidates should be size-limited and schema-validated before AddICECandidate",
		},
		{
			name:       "DirectH264LogsRemoteAddr",
			path:       "../service/stream/direct/h264.go",
			vulnerable: []string{`log.Debugf("h264 websocket connected: %s", ws.RemoteAddr())`},
			message:    "direct H264 websocket logs should not record remote addresses",
		},
		{
			name:       "DirectH264DisconnectLogsRemoteAddr",
			path:       "../service/stream/direct/h264.go",
			vulnerable: []string{`log.Debugf("h264 websocket disconnected: %s", ws.RemoteAddr())`},
			message:    "direct H264 disconnect logs should not record remote addresses",
		},
		{
			name:       "DirectH264ReadDeadlineDisabled",
			path:       "../service/stream/direct/h264.go",
			vulnerable: []string{`ws.SetReadDeadline(time.Time{})`},
			message:    "direct H264 websocket should have read deadlines",
		},
		{
			name:       "DirectH264ReadsAndIgnoresClientMessages",
			path:       "../service/stream/direct/h264.go",
			vulnerable: []string{`ws.NextReader()`},
			message:    "direct H264 websocket should not accept arbitrary client messages beyond heartbeat/close",
		},
		{
			name:       "DirectStreamerHasUnboundedClientsMap",
			path:       "../service/stream/direct/streamer.go",
			vulnerable: []string{`clients: make(map[*websocket.Conn]bool)`},
			message:    "direct H264 streamer should enforce per-user and global client limits",
		},
		{
			name:       "DirectStreamerWritesFrameToEveryClient",
			path:       "../service/stream/direct/streamer.go",
			vulnerable: []string{`for _, client := range clients {`, `client.WriteMessage(websocket.BinaryMessage, buf.Bytes())`},
			message:    "direct H264 streamer should account for slow clients and authorization freshness before every frame write",
		},
		{
			name:       "DirectStreamerLogsClientRemoteAddrOnWriteFailure",
			path:       "../service/stream/direct/streamer.go",
			vulnerable: []string{`log.Errorf("failed to write message to client %s: %s.", client.RemoteAddr(), err)`},
			message:    "direct H264 write failures should not log remote addresses",
		},
		{
			name:       "GenericWebSocketConnectHasNoSessionBinding",
			path:       "../service/ws/service.go",
			vulnerable: []string{`client := NewClient(ws)`},
			message:    "desktop control websocket should bind connections to an explicit auth/session context",
		},
		{
			name:       "GenericWebSocketReadDeadlineDisabled",
			path:       "../service/ws/client.go",
			vulnerable: []string{`var zeroTime time.Time`, `c.ws.SetReadDeadline(zeroTime)`},
			message:    "desktop control websocket should have idle read deadlines",
		},
		{
			name:       "GenericWebSocketLogsRawHIDPayload",
			path:       "../service/ws/client.go",
			vulnerable: []string{`log.Debugf("received message %d: %v", messageType, data)`},
			message:    "desktop control websocket should not log raw keyboard/mouse payload bytes",
		},
		{
			name:       "GenericWebSocketKeyboardQueueAcceptsArbitraryBytes",
			path:       "../service/ws/client.go",
			vulnerable: []string{`writeQueue(c.keyboard, data[1:])`},
			message:    "keyboard websocket payloads should validate HID report length and shape",
		},
		{
			name:       "GenericWebSocketMouseQueueAcceptsArbitraryBytes",
			path:       "../service/ws/client.go",
			vulnerable: []string{`writeQueue(c.mouse, data[1:])`},
			message:    "mouse websocket payloads should validate HID report length and shape",
		},
		{
			name:       "GenericWebSocketHIDOpenOnConnect",
			path:       "../service/ws/client.go",
			vulnerable: []string{`client.hid.Open()`},
			message:    "desktop control websocket should not open HID devices before validating message intent",
		},
	}

	runSourceSecurityContracts(t, cases, 28, "stream-websocket")
}
