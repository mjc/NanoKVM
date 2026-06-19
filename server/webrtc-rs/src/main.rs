mod config;
mod kvm;
mod prep;
mod screen;
mod signaling;
mod webrtc_session;

use std::net::{IpAddr, Ipv4Addr, SocketAddr};
use std::sync::Arc;

use anyhow::Context;
use axum::extract::ws::WebSocketUpgrade;
use axum::extract::State;
use axum::response::Html;
use axum::response::IntoResponse;
use axum::routing::get;
use axum::Json;
use axum::Router;
use serde::Serialize;
use str0m::crypto::from_feature_flags;
use tokio::net::TcpListener;
use tracing::{error, info};

#[derive(Clone)]
struct AppState {
    config: Arc<config::NanoKvmConfig>,
}

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    tracing_subscriber::fmt()
        .with_env_filter(tracing_env_filter(std::env::var("RUST_LOG").ok().as_deref()))
        .init();
    from_feature_flags().install_process_default();

    let config = Arc::new(config::NanoKvmConfig::load()?);
    prep::run();
    prewarm_kvm_vision().await;
    let state = AppState { config };
    let addr = websocket_bind_addr();

    let app = Router::new()
        .route("/", get(index))
        .route("/debug/capture", get(debug_capture))
        .route("/api/stream/h264", get(h264_ws))
        .with_state(state);
    let listener = TcpListener::bind(addr)
        .await
        .with_context(|| format!("bind Rust WebRTC sidecar on {addr}"))?;

    info!("NanoKVM Rust WebRTC sidecar listening on {addr}");
    axum::serve(listener, app)
        .with_graceful_shutdown(shutdown_signal())
        .await
        .context("serve Rust WebRTC sidecar")
}

async fn prewarm_kvm_vision() {
    info!("initializing KVM vision");
    match tokio::task::spawn_blocking(kvm::KvmVision::new).await {
        Ok(Ok(_)) => info!("KVM vision initialized"),
        Ok(Err(err)) => error!("failed to initialize KVM vision: {err:#}"),
        Err(err) => error!("KVM vision initialization task failed: {err}"),
    }
}

async fn h264_ws(ws: WebSocketUpgrade, State(state): State<AppState>) -> axum::response::Response {
    ws.on_upgrade(move |socket| async move {
        if let Err(err) = webrtc_session::serve(socket, state.config).await {
            error!("WebRTC session ended with error: {err:#}");
        }
    })
}

#[derive(Debug, Serialize)]
struct DebugCapture {
    ok: bool,
    width: u16,
    height: u16,
    fps: u64,
    bitrate: u16,
    result: Option<i32>,
    bytes: usize,
    prefix: Vec<String>,
    error: Option<String>,
}

async fn debug_capture() -> impl IntoResponse {
    let screen = screen::Screen::read();
    let mut response = DebugCapture {
        ok: false,
        width: screen.width,
        height: screen.height,
        fps: screen.fps,
        bitrate: screen.bitrate,
        result: None,
        bytes: 0,
        prefix: Vec::new(),
        error: None,
    };

    match tokio::task::spawn_blocking(move || {
        let vision = kvm::KvmVision::new()?;
        vision.read_h264(screen.width, screen.height, screen.bitrate)
    })
    .await
    {
        Ok(Ok(Some(frame))) => {
            response.ok = frame.result >= 0 && !frame.data.is_empty();
            response.result = Some(frame.result);
            response.bytes = frame.data.len();
            response.prefix = frame
                .data
                .iter()
                .take(16)
                .map(|byte| format!("{byte:02x}"))
                .collect();
        }
        Ok(Ok(None)) => {
            response.error = Some("capture unsupported on this architecture".to_string());
        }
        Ok(Err(err)) => {
            response.error = Some(format!("{err:#}"));
        }
        Err(err) => {
            response.error = Some(format!("capture task failed: {err}"));
        }
    }

    Json(response)
}

fn websocket_bind_addr() -> SocketAddr {
    SocketAddr::new(IpAddr::V4(Ipv4Addr::UNSPECIFIED), 6040)
}

fn tracing_env_filter(spec: Option<&str>) -> tracing_subscriber::EnvFilter {
    spec.and_then(|spec| tracing_subscriber::EnvFilter::try_new(spec).ok())
        .unwrap_or_else(|| tracing_subscriber::EnvFilter::new("debug"))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn websocket_bind_addr_uses_unspecified_ipv4_on_port_6040() {
        assert_eq!(
            websocket_bind_addr(),
            SocketAddr::new(IpAddr::V4(Ipv4Addr::UNSPECIFIED), 6040)
        );
    }

    #[test]
    fn tracing_env_filter_defaults_to_debug_when_missing() {
        assert_eq!(tracing_env_filter(None).to_string(), "debug");
    }

    #[test]
    fn tracing_env_filter_uses_requested_log_level() {
        assert_eq!(tracing_env_filter(Some("info")).to_string(), "info");
    }
}

async fn index() -> Html<&'static str> {
    Html(
        r#"<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>NanoKVM Rust WebRTC</title>
  <style>
    html, body {
      height: 100%;
      margin: 0;
      background: #111;
      color: #eee;
      font: 14px system-ui, sans-serif;
    }
    body {
      display: grid;
      grid-template-rows: auto 1fr;
    }
    header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 16px;
      padding: 10px 12px;
      background: #191919;
      border-bottom: 1px solid #333;
    }
    video {
      width: 100%;
      height: 100%;
      object-fit: contain;
      background: #000;
    }
    #status {
      color: #9fd;
      white-space: nowrap;
    }
  </style>
</head>
<body>
  <header>
    <strong>NanoKVM Rust WebRTC</strong>
    <span id="status">connecting</span>
  </header>
  <video id="screen" autoplay muted playsinline controls></video>
  <script>
    const statusEl = document.getElementById("status");
    const videoEl = document.getElementById("screen");
    const wsUrl = `${location.protocol === "https:" ? "wss" : "ws"}://${location.host}/api/stream/h264`;
    const ws = new WebSocket(wsUrl);
    let peer = null;
    let offerSent = false;
    let heartbeatTimer = null;
    const pendingCandidates = [];

    function setStatus(value) {
      statusEl.textContent = value;
    }

    function send(event, data) {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ event, data }));
      }
    }

    function parseData(data) {
      return data ? JSON.parse(data) : null;
    }

    function startVideo(iceServers) {
      if (peer) {
        return;
      }

      peer = new RTCPeerConnection({ iceServers });
      peer.onconnectionstatechange = () => setStatus(`peer ${peer.connectionState}`);
      peer.oniceconnectionstatechange = () => setStatus(`ice ${peer.iceConnectionState}`);
      peer.ontrack = event => {
        if (event.track.kind === "video") {
          videoEl.srcObject = new MediaStream([event.track]);
          setStatus("video");
        }
      };
      peer.onicecandidate = event => {
        if (event.candidate) {
          send("video-candidate", JSON.stringify(event.candidate));
        }
      };
      peer.onnegotiationneeded = async () => {
        if (offerSent || peer.signalingState !== "stable") {
          return;
        }

        offerSent = true;
        setStatus("offering");
        try {
          const offer = await peer.createOffer({
            offerToReceiveVideo: true,
            offerToReceiveAudio: false
          });
          await peer.setLocalDescription(offer);
          send("video-offer", JSON.stringify(peer.localDescription));
        } catch (err) {
          offerSent = false;
          setStatus(`offer failed: ${err.message}`);
        }
      };
      peer.addTransceiver("video", { direction: "recvonly" });
    }

    async function handleAnswer(data) {
      if (!peer || peer.signalingState !== "have-local-offer") {
        offerSent = false;
        return;
      }

      await peer.setRemoteDescription(new RTCSessionDescription(data));
      offerSent = false;
      while (pendingCandidates.length) {
        await peer.addIceCandidate(pendingCandidates.shift());
      }
    }

    async function handleCandidate(data) {
      if (!peer || !data || !data.candidate) {
        return;
      }

      const candidate = new RTCIceCandidate(data);
      if (peer.remoteDescription) {
        await peer.addIceCandidate(candidate);
      } else {
        pendingCandidates.push(candidate);
      }
    }

    ws.onopen = () => {
      setStatus("websocket");
      heartbeatTimer = setInterval(() => send("heartbeat", ""), 60000);
    };
    ws.onclose = () => {
      setStatus("websocket closed");
      if (heartbeatTimer) {
        clearInterval(heartbeatTimer);
      }
      if (peer) {
        peer.close();
      }
    };
    ws.onerror = () => setStatus("websocket error");
    ws.onmessage = async event => {
      try {
        const msg = JSON.parse(event.data);
        switch (msg.event) {
          case "ice-servers":
            startVideo(parseData(msg.data) || []);
            break;
          case "video-answer":
            await handleAnswer(parseData(msg.data));
            break;
          case "video-candidate":
            await handleCandidate(parseData(msg.data));
            break;
          case "heartbeat":
            break;
          default:
            console.log("Unhandled event", msg);
        }
      } catch (err) {
        console.error(err);
        setStatus(`error: ${err.message}`);
      }
    };
  </script>
</body>
</html>"#,
    )
}

async fn shutdown_signal() {
    let _ = tokio::signal::ctrl_c().await;
}
