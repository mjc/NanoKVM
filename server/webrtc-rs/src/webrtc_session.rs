use std::net::{IpAddr, Ipv4Addr, SocketAddr};
use std::sync::{Arc, Mutex};
use std::time::Instant;

use anyhow::{anyhow, Context};
use axum::extract::ws::{Message as WsMessage, WebSocket};
use futures_util::{SinkExt, StreamExt};
use get_if_addrs::{get_if_addrs, IfAddr};
use str0m::change::SdpOffer;
use str0m::format::Codec;
use str0m::media::{Direction, MediaKind, MediaTime, Mid, Pt};
use str0m::net::{Protocol, Receive};
use str0m::rtp::Extension;
use str0m::{Candidate, Event, IceConnectionState, Input, Output, Rtc, RtcConfig};
use tokio::net::UdpSocket;
use tokio::sync::mpsc;
use tokio::time::{self, Instant as TokioInstant};
use tracing::{debug, info, warn};

use crate::config::NanoKvmConfig;
use crate::kvm::KvmVision;
use crate::signaling::Message;

static CAPTURE_LOCK: Mutex<()> = Mutex::new(());

#[derive(Debug)]
enum SessionEvent {
    Browser(Message),
    Udp {
        source: SocketAddr,
        destination: SocketAddr,
        data: Vec<u8>,
    },
    Closed,
}

#[derive(Debug)]
struct CaptureState {
    video_mid: Option<Mid>,
    video_pt: Option<Pt>,
    capture_active: bool,
    next_capture_at: Option<Instant>,
    media_time: MediaTime,
    frames_seen: u64,
    frames_sent: u64,
}

impl Default for CaptureState {
    fn default() -> Self {
        Self {
            video_mid: None,
            video_pt: None,
            capture_active: false,
            next_capture_at: None,
            media_time: MediaTime::ZERO,
            frames_seen: 0,
            frames_sent: 0,
        }
    }
}

pub async fn serve(socket: WebSocket, config: Arc<NanoKvmConfig>) -> anyhow::Result<()> {
    let (ws_tx, ws_rx) = socket.split();
    let (event_tx, mut event_rx) = mpsc::unbounded_channel::<SessionEvent>();
    let (signaling_tx, mut signaling_rx) = mpsc::unbounded_channel::<Message>();

    let ws_writer = tokio::spawn(async move {
        let mut ws_tx = ws_tx;
        while let Some(message) = signaling_rx.recv().await {
            let raw = match serde_json::to_string(&message) {
                Ok(raw) => raw,
                Err(err) => {
                    warn!("failed to encode signaling message: {err}");
                    continue;
                }
            };

            if ws_tx.send(WsMessage::Text(raw)).await.is_err() {
                break;
            }
        }
    });

    let ws_reader = {
        let event_tx = event_tx.clone();
        tokio::spawn(async move {
            let mut ws_rx = ws_rx;
            while let Some(message) = ws_rx.next().await {
                match message {
                    Ok(WsMessage::Text(raw)) => match serde_json::from_str::<Message>(&raw) {
                        Ok(message) => {
                            if event_tx.send(SessionEvent::Browser(message)).is_err() {
                                break;
                            }
                        }
                        Err(err) => warn!("failed to parse signaling message: {err}"),
                    },
                    Ok(WsMessage::Close(_)) => break,
                    Ok(WsMessage::Ping(_)) | Ok(WsMessage::Pong(_)) | Ok(WsMessage::Binary(_)) => {}
                    Err(err) => {
                        warn!("websocket read failed: {err}");
                        break;
                    }
                }
            }

            let _ = event_tx.send(SessionEvent::Closed);
        })
    };

    let bind_ip = select_bind_ip()?;
    let udp_socket = Arc::new(
        UdpSocket::bind(SocketAddr::new(bind_ip, 0))
            .await
            .with_context(|| format!("bind UDP socket for WebRTC on {bind_ip}"))?,
    );
    let local_udp_addr = udp_socket
        .local_addr()
        .context("read local WebRTC UDP socket address")?;
    info!("Rust WebRTC UDP socket bound on {local_udp_addr}");

    let udp_reader = {
        let udp_socket = Arc::clone(&udp_socket);
        let event_tx = event_tx.clone();
        tokio::spawn(async move {
            let mut buf = vec![0_u8; 2048];
            loop {
                let (len, source) = match udp_socket.recv_from(&mut buf).await {
                    Ok(v) => v,
                    Err(err) => {
                        warn!("UDP receive failed: {err}");
                        break;
                    }
                };

                let destination = match udp_socket.local_addr() {
                    Ok(addr) => addr,
                    Err(err) => {
                        warn!("failed to read UDP local address: {err}");
                        break;
                    }
                };

                if event_tx
                    .send(SessionEvent::Udp {
                        source,
                        destination,
                        data: buf[..len].to_vec(),
                    })
                    .is_err()
                {
                    break;
                }
            }

            let _ = event_tx.send(SessionEvent::Closed);
        })
    };

    let mut vision = None;
    let mut rtc = build_rtc();
    rtc.add_local_candidate(
        Candidate::host(local_udp_addr, "udp").context("add local ICE candidate")?,
    );

    let mut state = CaptureState::default();
    let mut next_rtc_timeout = drain_rtc(&mut rtc, &udp_socket, &mut state).await?;

    signaling_tx.send(ice_servers_message(&config)?)?;

    loop {
        sync_capture_state(&mut rtc, &mut state)?;
        process_due_timers(
            &mut vision,
            &udp_socket,
            &mut rtc,
            &mut state,
            &mut next_rtc_timeout,
        )
        .await?;

        let next_deadline = state
            .next_capture_at
            .map(|capture_at| next_rtc_timeout.min(capture_at))
            .or(Some(next_rtc_timeout));

        let event = match next_deadline {
            Some(deadline) => {
                tokio::select! {
                    event = event_rx.recv() => event,
                    _ = time::sleep_until(TokioInstant::from_std(deadline)) => continue,
                }
            }
            None => event_rx.recv().await,
        };

        match event {
            Some(SessionEvent::Browser(message)) => {
                handle_browser_message(
                    message,
                    &udp_socket,
                    &signaling_tx,
                    &mut rtc,
                    &mut state,
                    &mut next_rtc_timeout,
                )
                .await?;
            }
            Some(SessionEvent::Udp {
                source,
                destination,
                data,
            }) => {
                let receive = Receive::new(Protocol::Udp, source, destination, &data)
                    .context("parse incoming WebRTC UDP packet")?;
                rtc.handle_input(Input::Receive(Instant::now(), receive))
                    .context("handle UDP input")?;
                next_rtc_timeout = drain_rtc(&mut rtc, &udp_socket, &mut state).await?;
            }
            Some(SessionEvent::Closed) | None => break,
        }
    }

    ws_reader.abort();
    ws_writer.abort();
    udp_reader.abort();
    Ok(())
}

async fn handle_browser_message(
    message: Message,
    udp_socket: &Arc<UdpSocket>,
    signaling_tx: &mpsc::UnboundedSender<Message>,
    rtc: &mut Rtc,
    state: &mut CaptureState,
    next_rtc_timeout: &mut Instant,
) -> anyhow::Result<()> {
    match message.event.as_str() {
        "video-offer" => {
            let offer: SdpOffer =
                serde_json::from_str(&message.data).context("parse video offer from browser")?;
            let answer = rtc
                .sdp_api()
                .accept_offer(offer)
                .context("accept video offer")?;
            signaling_tx.send(Message::new(
                "video-answer",
                serde_json::to_string(&answer).context("serialize video answer")?,
            ))?;
            *next_rtc_timeout = drain_rtc(rtc, udp_socket, state).await?;
        }
        "video-candidate" => match parse_browser_candidate(&message.data) {
            Ok(Some(candidate)) => {
                rtc.add_remote_candidate(candidate);
                *next_rtc_timeout = drain_rtc(rtc, udp_socket, state).await?;
            }
            Ok(None) => {}
            Err(err) => warn!("ignoring browser ICE candidate: {err:#}"),
        },
        "heartbeat" => {
            signaling_tx.send(Message::new("heartbeat", ""))?;
        }
        other => debug!("unhandled signaling event: {other}"),
    }

    Ok(())
}

async fn process_due_timers(
    vision: &mut Option<KvmVision>,
    udp_socket: &Arc<UdpSocket>,
    rtc: &mut Rtc,
    state: &mut CaptureState,
    next_rtc_timeout: &mut Instant,
) -> anyhow::Result<()> {
    loop {
        let now = Instant::now();

        if state
            .next_capture_at
            .is_some_and(|deadline| deadline <= now)
        {
            capture_frame(vision, udp_socket, rtc, state, next_rtc_timeout).await?;
            continue;
        }

        if *next_rtc_timeout <= now {
            rtc.handle_input(Input::Timeout(now))
                .context("handle RTC timeout")?;
            *next_rtc_timeout = drain_rtc(rtc, udp_socket, state).await?;
            continue;
        }

        break;
    }

    Ok(())
}

async fn capture_frame(
    vision: &mut Option<KvmVision>,
    udp_socket: &Arc<UdpSocket>,
    rtc: &mut Rtc,
    state: &mut CaptureState,
    next_rtc_timeout: &mut Instant,
) -> anyhow::Result<()> {
    let screen = crate::screen::Screen::read();
    let frame_duration = screen.frame_duration();
    let wallclock = Instant::now();
    let media_time = advance_media_time(state, frame_duration);
    if vision.is_none() {
        info!("initializing KVM vision for H264 capture");
        match KvmVision::new() {
            Ok(new_vision) => {
                *vision = Some(new_vision);
            }
            Err(err) => {
                warn!("failed to initialize KVM vision: {err:#}");
                state.next_capture_at = Some(Instant::now() + frame_duration);
                return Ok(());
            }
        }
    }
    let vision = vision.as_ref().expect("vision initialized above");

    state.frames_seen += 1;
    if should_log_frame(state.frames_seen, 0) {
        info!(
            "H264 frame read begin width={} height={} bitrate={}",
            screen.width, screen.height, screen.bitrate
        );
    }

    let frame = {
        let _guard = CAPTURE_LOCK
            .lock()
            .map_err(|_| anyhow!("KVM capture lock poisoned"))?;
        vision.read_h264(screen.width, screen.height, screen.bitrate)
    };

    match frame {
        Ok(Some(frame)) => {
            if should_log_frame(state.frames_seen, frame.result) {
                info!(
                    "H264 frame read result={} bytes={} {}",
                    frame.result,
                    frame.data.len(),
                    describe_h264(&frame.data)
                );
            }

            if frame.data.is_empty() {
                state.next_capture_at = Some(Instant::now() + frame_duration);
                return Ok(());
            }

            let mid = match state.video_mid {
                Some(mid) => mid,
                None => {
                    warn!("dropping H264 frame because no negotiated video mid is available");
                    return Ok(());
                }
            };
            let pt = match state.video_pt {
                Some(pt) => pt,
                None => {
                    warn!(
                        "dropping H264 frame because no negotiated H264 payload type is available"
                    );
                    return Ok(());
                }
            };

            match rtc.writer(mid) {
                Some(writer) => {
                    if let Err(err) = writer
                        .playout_delay(MediaTime::ZERO, MediaTime::ZERO)
                        .write(pt, wallclock, media_time, frame.data)
                    {
                        warn!("failed to write H264 frame: {err}");
                    } else {
                        state.frames_sent += 1;
                        if should_log_frame(state.frames_sent, frame.result) {
                            info!(
                                "H264 frame queued result={} sent_frames={}",
                                frame.result, state.frames_sent
                            );
                        }
                        *next_rtc_timeout = drain_rtc(rtc, udp_socket, state).await?;
                    }
                }
                None => {
                    warn!("dropping H264 frame because no RTP writer exists for mid {mid}");
                }
            }
        }
        Ok(None) => {
            if should_log_frame(state.frames_seen, -999) {
                info!("H264 frame read returned no frame");
            }
        }
        Err(err) => warn!("failed to read H264 frame: {err:#}"),
    }

    state.next_capture_at = Some(Instant::now() + frame_duration);
    Ok(())
}

fn advance_media_time(state: &mut CaptureState, frame_duration: std::time::Duration) -> MediaTime {
    let media_time = state.media_time;
    state.media_time += frame_duration.into();
    media_time
}

async fn drain_rtc(
    rtc: &mut Rtc,
    udp_socket: &Arc<UdpSocket>,
    state: &mut CaptureState,
) -> anyhow::Result<Instant> {
    loop {
        match rtc.poll_output().context("poll RTC output")? {
            Output::Timeout(timeout) => {
                sync_capture_state(rtc, state)?;
                return Ok(timeout);
            }
            Output::Transmit(transmit) => {
                udp_socket
                    .send_to(&transmit.contents, transmit.destination)
                    .await
                    .with_context(|| format!("send UDP packet to {}", transmit.destination))?;
            }
            Output::Event(event) => {
                handle_rtc_event(event, state);
            }
        }
    }
}

fn handle_rtc_event(event: Event, state: &mut CaptureState) {
    match event {
        Event::Connected => {
            info!("RTCPeer connected");
        }
        Event::IceConnectionStateChange(state_change) => {
            info!("ICE connection state changed to {:?}", state_change);
            if matches!(state_change, IceConnectionState::Disconnected) {
                state.capture_active = false;
                state.next_capture_at = None;
            }
        }
        Event::MediaAdded(added) => {
            if matches!(added.kind, MediaKind::Video) {
                info!("video media added with mid {}", added.mid);
                state.video_mid = Some(added.mid);
                state.video_pt = None;
                state.capture_active = false;
                state.next_capture_at = None;
            }
        }
        Event::MediaChanged(changed) => {
            debug!("media changed on mid {}", changed.mid);
            if state.video_mid == Some(changed.mid)
                && !matches!(changed.direction, Direction::SendOnly | Direction::SendRecv)
            {
                state.capture_active = false;
                state.next_capture_at = None;
            }
        }
        Event::MediaData(_) => {}
        _ => {}
    }
}

fn sync_capture_state(rtc: &mut Rtc, state: &mut CaptureState) -> anyhow::Result<()> {
    sync_capture_state_for_connection(rtc.is_connected(), rtc, state)
}

fn sync_capture_state_for_connection(
    connected: bool,
    rtc: &mut Rtc,
    state: &mut CaptureState,
) -> anyhow::Result<()> {
    if connected {
        if state.video_mid.is_some() && state.video_pt.is_none() {
            state.video_pt = resolve_video_pt(rtc, state.video_mid.unwrap());
            if let Some(pt) = state.video_pt {
                info!("negotiated H264 payload type {pt}");
            } else {
                warn!("video media is connected but H264 payload type is not negotiated yet");
            }
        }

        if state.video_mid.is_some() && state.video_pt.is_some() && !state.capture_active {
            info!("starting H264 capture");
            state.capture_active = true;
            state.media_time = MediaTime::ZERO;
            state.frames_seen = 0;
            state.frames_sent = 0;
            state.next_capture_at = Some(Instant::now());
        } else if state.capture_active && state.next_capture_at.is_none() {
            state.next_capture_at = Some(Instant::now());
        }
    } else if state.capture_active {
        info!("stopping H264 capture");
        state.capture_active = false;
        state.next_capture_at = None;
    }

    Ok(())
}

fn should_log_frame(frame_index: u64, result: i32) -> bool {
    frame_index <= 16 || frame_index % 120 == 0 || result == 3
}

fn describe_h264(data: &[u8]) -> String {
    let prefix_len = data.len().min(16);
    let prefix = data[..prefix_len]
        .iter()
        .map(|byte| format!("{byte:02x}"))
        .collect::<Vec<_>>()
        .join(" ");
    let nals = annex_b_nal_types(data);
    if nals.is_empty() {
        return format!("prefix=[{prefix}] annexb_nals=[]");
    }

    format!("prefix=[{prefix}] annexb_nals={nals:?}")
}

fn annex_b_nal_types(data: &[u8]) -> Vec<u8> {
    let mut types = Vec::new();
    let mut index = 0;
    while let Some((start, len)) = find_start_code(data, index) {
        let nal_start = start + len;
        if let Some(header) = data.get(nal_start) {
            types.push(header & 0x1f);
            if types.len() == 12 {
                break;
            }
        }
        index = nal_start.saturating_add(1);
    }

    types
}

fn find_start_code(data: &[u8], from: usize) -> Option<(usize, usize)> {
    let mut index = from;
    while index + 3 <= data.len() {
        if data[index..].starts_with(&[0, 0, 1]) {
            return Some((index, 3));
        }
        if index + 4 <= data.len() && data[index..].starts_with(&[0, 0, 0, 1]) {
            return Some((index, 4));
        }
        index += 1;
    }

    None
}

fn resolve_video_pt(rtc: &mut Rtc, mid: Mid) -> Option<Pt> {
    let pt = {
        let writer = rtc.writer(mid)?;
        let pt = writer
            .payload_params()
            .find(|params| params.spec().codec == Codec::H264)
            .map(|params| params.pt());
        pt
    };

    pt
}

fn parse_browser_candidate(raw: &str) -> anyhow::Result<Option<Candidate>> {
    let Some(candidate) = browser_candidate_line(raw)? else {
        return Ok(None);
    };

    let mut parts = candidate.split_whitespace();
    let foundation = parts
        .next()
        .and_then(|part| part.strip_prefix("candidate:"))
        .ok_or_else(|| anyhow!("missing ICE candidate foundation"))?;
    let _component = parts
        .next()
        .ok_or_else(|| anyhow!("missing ICE candidate component"))?
        .parse::<u16>()
        .context("parse ICE candidate component")?;
    let protocol = parts
        .next()
        .ok_or_else(|| anyhow!("missing ICE candidate protocol"))?;
    let _priority = parts
        .next()
        .ok_or_else(|| anyhow!("missing ICE candidate priority"))?
        .parse::<u32>()
        .context("parse ICE candidate priority")?;
    let address_raw = parts
        .next()
        .ok_or_else(|| anyhow!("missing ICE candidate address"))?;
    let address = match address_raw.parse::<IpAddr>() {
        Ok(address) => address,
        Err(_) => {
            debug!("ignoring non-IP browser ICE candidate address {address_raw}");
            return Ok(None);
        }
    };
    let port = parts
        .next()
        .ok_or_else(|| anyhow!("missing ICE candidate port"))?
        .parse::<u16>()
        .context("parse ICE candidate port")?;

    match parts.next() {
        Some("typ") => {}
        other => return Err(anyhow!("invalid ICE candidate type marker: {other:?}")),
    }

    let candidate_type = parts
        .next()
        .ok_or_else(|| anyhow!("missing ICE candidate type"))?;
    debug!("browser candidate {foundation} typ {candidate_type} {address}:{port}");

    Candidate::host(SocketAddr::new(address, port), protocol)
        .map(Some)
        .context("create ICE host candidate")
}

#[derive(serde::Deserialize)]
struct BrowserIceCandidate {
    #[serde(default)]
    candidate: String,
}

fn browser_candidate_line(raw: &str) -> anyhow::Result<Option<String>> {
    let raw = raw.trim();
    if raw.is_empty() {
        return Ok(None);
    }

    if raw.starts_with('{') {
        let candidate: BrowserIceCandidate =
            serde_json::from_str(raw).context("parse browser ICE candidate JSON")?;
        let candidate = candidate.candidate.trim();
        if candidate.is_empty() {
            return Ok(None);
        }

        return Ok(Some(candidate.to_owned()));
    }

    Ok(Some(raw.to_owned()))
}

fn build_rtc() -> Rtc {
    let mut config = RtcConfig::new()
        .set_ice_lite(true)
        .clear_codecs()
        .enable_h264(true);
    config.extension_map().set(5, Extension::PlayoutDelay);
    config.build(Instant::now())
}

fn ice_servers_message(config: &NanoKvmConfig) -> anyhow::Result<Message> {
    Ok(Message::new(
        "ice-servers",
        serde_json::to_string(&config.client_ice_servers())?,
    ))
}

fn select_bind_ip() -> anyhow::Result<IpAddr> {
    let candidates = get_if_addrs()?.into_iter().filter_map(|iface| match iface.addr {
        IfAddr::V4(v4) => Some(IpAddr::V4(v4.ip)),
        _ => None,
    });

    Ok(select_bind_ip_from_candidates(candidates))
}

fn select_bind_ip_from_candidates<I>(candidates: I) -> IpAddr
where
    I: IntoIterator<Item = IpAddr>,
{
    candidates
        .into_iter()
        .find(|addr| matches!(addr, IpAddr::V4(v4) if !v4.is_loopback() && !v4.is_unspecified()))
        .unwrap_or(IpAddr::V4(Ipv4Addr::LOCALHOST))
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::config::TurnConfig;
    use std::sync::Arc;
    use std::time::Duration;
    use str0m::change::SdpAnswer;
    use str0m::format::Codec;
    use str0m::media::{Direction, MediaKind};
    use str0m::media::{MediaTime, Mid, Pt};
    use str0m::Candidate;
    use tokio::net::UdpSocket;
    use tokio::sync::mpsc;

    #[test]
    fn parses_browser_candidate_string() {
        let candidate = parse_browser_candidate(
            "candidate:1 1 udp 2122260223 192.0.2.10 5000 typ host generation 0",
        )
        .unwrap()
        .unwrap();

        assert!(format!("{candidate:?}").contains("192.0.2.10"));
    }

    #[test]
    fn parses_browser_candidate_json() {
        let candidate = parse_browser_candidate(
            r#"{"candidate":"candidate:2 1 udp 2122260223 192.0.2.11 5001 typ host generation 0 ufrag abc network-id 1","sdpMid":"0","sdpMLineIndex":0}"#,
        )
        .unwrap()
        .unwrap();

        assert!(format!("{candidate:?}").contains("192.0.2.11"));
    }

    #[test]
    fn parses_browser_candidate_ipv6_host() {
        let candidate = parse_browser_candidate(
            "candidate:4 1 udp 2122260223 2001:db8::10 5004 typ host generation 0",
        )
        .unwrap()
        .unwrap();

        assert!(format!("{candidate:?}").contains("2001:db8::10"));
    }

    #[test]
    fn ignores_browser_end_of_candidates() {
        assert!(
            parse_browser_candidate(r#"{"candidate":"","sdpMid":"0","sdpMLineIndex":0}"#)
                .unwrap()
                .is_none()
        );
    }

    #[test]
    fn ignores_browser_mdns_candidate() {
        assert!(parse_browser_candidate(
            "candidate:3 1 udp 2122260223 abcdef.local 5002 typ host generation 0",
        )
        .unwrap()
        .is_none());
    }

    #[test]
    fn browser_candidate_line_accepts_trimmed_raw_candidate() {
        assert_eq!(
            browser_candidate_line("  candidate:9 1 udp 2122260223 192.0.2.99 5009 typ host generation 0  ")
                .unwrap(),
            Some("candidate:9 1 udp 2122260223 192.0.2.99 5009 typ host generation 0".to_owned())
        );
    }

    #[test]
    fn browser_candidate_line_ignores_empty_and_whitespace() {
        assert!(browser_candidate_line("").unwrap().is_none());
        assert!(browser_candidate_line("   ").unwrap().is_none());
    }

    #[test]
    fn browser_candidate_line_rejects_bad_json() {
        assert!(browser_candidate_line(r#"{"candidate":}"#).is_err());
    }

    #[test]
    fn ice_servers_message_matches_frontend_contract() {
        let config = NanoKvmConfig {
            stun: "stun.example:19302".to_owned(),
            turn: TurnConfig {
                turn_addr: "turn.example:3478".to_owned(),
                turn_user: "user".to_owned(),
                turn_cred: "pass".to_owned(),
            },
        };

        let msg = ice_servers_message(&config).unwrap();
        assert_eq!(msg.event, "ice-servers");
        let servers: serde_json::Value = serde_json::from_str(&msg.data).unwrap();
        let servers = servers.as_array().expect("ice-servers array");
        assert_eq!(servers.len(), 2);
        assert_eq!(servers[0]["urls"], serde_json::json!(["stun:stun.example:19302"]));
        assert_eq!(servers[1]["urls"], serde_json::json!(["turn:turn.example:3478"]));
        assert_eq!(servers[1]["username"], serde_json::json!("user"));
        assert_eq!(servers[1]["credential"], serde_json::json!("pass"));
    }

    #[test]
    fn select_bind_ip_from_candidates_prefers_first_routable_ipv4() {
        let selected = select_bind_ip_from_candidates([
            IpAddr::V4(Ipv4Addr::LOCALHOST),
            IpAddr::V4(Ipv4Addr::UNSPECIFIED),
            IpAddr::V4(Ipv4Addr::new(192, 0, 2, 55)),
        ]);

        assert_eq!(selected, IpAddr::V4(Ipv4Addr::new(192, 0, 2, 55)));
    }

    #[test]
    fn select_bind_ip_from_candidates_falls_back_to_localhost() {
        let selected = select_bind_ip_from_candidates([
            IpAddr::V4(Ipv4Addr::LOCALHOST),
            IpAddr::V4(Ipv4Addr::UNSPECIFIED),
            IpAddr::V6("2001:db8::1".parse().unwrap()),
        ]);

        assert_eq!(selected, IpAddr::V4(Ipv4Addr::LOCALHOST));
    }

    #[tokio::test]
    async fn heartbeat_round_trips_without_disturbing_capture_state() {
        let socket = Arc::new(
            UdpSocket::bind((Ipv4Addr::LOCALHOST, 0))
                .await
                .expect("bind test UDP socket"),
        );
        let mut rtc = build_rtc();
        let mut state = CaptureState::default();
        let mut next_rtc_timeout = Instant::now();
        let (signaling_tx, mut signaling_rx) = mpsc::unbounded_channel();

        handle_browser_message(
            Message::new("heartbeat", ""),
            &socket,
            &signaling_tx,
            &mut rtc,
            &mut state,
            &mut next_rtc_timeout,
        )
        .await
        .expect("heartbeat event");

        assert_eq!(
            signaling_rx.recv().await,
            Some(Message::new("heartbeat", ""))
        );
        assert!(state.video_mid.is_none());
        assert!(state.video_pt.is_none());
        assert!(!state.capture_active);
    }

    #[tokio::test]
    async fn unknown_browser_event_is_ignored_without_emitting_signaling() {
        let socket = Arc::new(
            UdpSocket::bind((Ipv4Addr::LOCALHOST, 0))
                .await
                .expect("bind test UDP socket"),
        );
        let mut rtc = build_rtc();
        let mut state = CaptureState::default();
        let mut next_rtc_timeout = Instant::now();
        let (signaling_tx, mut signaling_rx) = mpsc::unbounded_channel();

        handle_browser_message(
            Message::new("something-else", "{}"),
            &socket,
            &signaling_tx,
            &mut rtc,
            &mut state,
            &mut next_rtc_timeout,
        )
        .await
        .expect("unknown event");

        assert!(signaling_rx.try_recv().is_err());
        assert!(state.video_mid.is_none());
        assert!(state.video_pt.is_none());
        assert!(!state.capture_active);
    }

    #[tokio::test]
    async fn video_offer_emits_browser_compatible_answer() {
        let socket = Arc::new(
            UdpSocket::bind((Ipv4Addr::LOCALHOST, 0))
                .await
                .expect("bind test UDP socket"),
        );
        let local_addr = socket.local_addr().expect("read local UDP socket address");
        let mut rtc = build_rtc();
        rtc.add_local_candidate(Candidate::host(local_addr, "udp").expect("local candidate"));

        let mut browser_offer_rtc = RtcConfig::new().clear_codecs().enable_h264(true).build(Instant::now());
        let mut change = browser_offer_rtc.sdp_api();
        change.add_media(MediaKind::Video, Direction::SendRecv, None, None, None);
        let (offer, _) = change.apply().expect("create offer");

        let offer_json = serde_json::to_string(&offer).expect("serialize offer");
        let mut state = CaptureState::default();
        let mut next_rtc_timeout = Instant::now();
        let (signaling_tx, mut signaling_rx) = mpsc::unbounded_channel();

        handle_browser_message(
            Message::new("video-offer", offer_json),
            &socket,
            &signaling_tx,
            &mut rtc,
            &mut state,
            &mut next_rtc_timeout,
        )
        .await
        .expect("video offer");

        let answer = signaling_rx.recv().await.expect("answer message");
        assert_eq!(answer.event, "video-answer");
        let parsed: SdpAnswer = serde_json::from_str(&answer.data).unwrap();
        assert!(!answer.data.is_empty());
        assert!(parsed.to_string().contains("m=video"));
    }

    #[test]
    fn advance_media_time_progresses_by_frame_duration_without_reset() {
        let mut state = CaptureState::default();

        let first = advance_media_time(&mut state, Duration::from_millis(40));
        let second = advance_media_time(&mut state, Duration::from_millis(40));
        let third = advance_media_time(&mut state, Duration::from_millis(40));

        assert_eq!(first, MediaTime::ZERO);
        assert_eq!(second, MediaTime::from_micros(40_000));
        assert_eq!(third, MediaTime::from_micros(80_000));
        assert_eq!(state.media_time, MediaTime::from_micros(120_000));
    }

    #[test]
    fn resolve_video_pt_selects_the_negotiated_h264_payload_type() {
        let now = Instant::now();
        let mut offerer = RtcConfig::new().clear_codecs().enable_h264(true).build(now);
        let mut answerer = RtcConfig::new().clear_codecs().enable_h264(true).build(now);

        let mut change = offerer.sdp_api();
        let mid = change.add_media(MediaKind::Video, Direction::SendRecv, None, None, None);
        let (offer, _) = change.apply().expect("create H264 offer");
        let answer = answerer
            .sdp_api()
            .accept_offer(offer)
            .expect("accept H264 offer");
        let _ = answer;

        let pt = resolve_video_pt(&mut answerer, mid).expect("negotiated video PT");
        let writer = answerer.writer(mid).expect("writer for negotiated video mid");
        let negotiated = writer
            .payload_params()
            .find(|params| params.spec().codec == Codec::H264)
            .expect("negotiated H264 payload");

        assert_eq!(pt, negotiated.pt());
        assert_eq!(negotiated.spec().codec, Codec::H264);
    }

    #[test]
    fn should_log_frame_limits_chatter_but_keeps_boundary_counts_visible() {
        assert!(should_log_frame(1, 0));
        assert!(should_log_frame(16, 0));
        assert!(should_log_frame(120, 0));
        assert!(should_log_frame(2, 3));
        assert!(!should_log_frame(17, 0));
        assert!(!should_log_frame(119, 0));
    }

    #[test]
    fn disconnected_peer_stops_capture_and_clears_next_deadline() {
        let mut state = CaptureState {
            video_mid: Some(Mid::from("screen")),
            video_pt: Some(Pt::from(127u8)),
            capture_active: true,
            next_capture_at: Some(Instant::now()),
            media_time: MediaTime::ZERO,
            frames_seen: 12,
            frames_sent: 8,
        };

        handle_rtc_event(
            Event::IceConnectionStateChange(IceConnectionState::Disconnected),
            &mut state,
        );

        assert!(!state.capture_active);
        assert!(state.next_capture_at.is_none());
    }

    #[test]
    fn sync_capture_state_for_connection_rearms_capture_after_idle() {
        let mut rtc = Rtc::new(Instant::now());
        let mut state = CaptureState {
            video_mid: Some(Mid::from("screen")),
            video_pt: Some(Pt::from(127u8)),
            capture_active: false,
            next_capture_at: None,
            media_time: MediaTime::from_micros(123_000),
            frames_seen: 12,
            frames_sent: 8,
        };

        sync_capture_state_for_connection(true, &mut rtc, &mut state)
            .expect("rearm capture on reconnect");

        assert!(state.capture_active);
        assert_eq!(state.media_time, MediaTime::ZERO);
        assert_eq!(state.frames_seen, 0);
        assert_eq!(state.frames_sent, 0);
        assert!(state.next_capture_at.is_some());
    }

    #[test]
    fn describes_annex_b_nal_types() {
        assert_eq!(
            annex_b_nal_types(&[0, 0, 0, 1, 0x67, 1, 2, 0, 0, 1, 0x68, 3]),
            vec![7, 8]
        );
    }

    #[test]
    fn annex_b_nal_types_ignores_leading_noise_and_non_annex_b_bytes() {
        assert_eq!(
            annex_b_nal_types(&[9, 9, 0, 0, 1, 0x65, 0xaa, 0xbb, 0, 0, 0, 1, 0x41]),
            vec![5, 1]
        );
    }
}
