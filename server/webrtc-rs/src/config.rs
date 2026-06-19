use std::fs;

use anyhow::Context;
use serde::Deserialize;

#[derive(Debug, Clone, Default, Deserialize)]
pub struct NanoKvmConfig {
    #[serde(default)]
    pub stun: String,
    #[serde(default)]
    pub turn: TurnConfig,
}

#[derive(Debug, Clone, Default, Deserialize)]
pub struct TurnConfig {
    #[serde(default, rename = "turnAddr")]
    pub turn_addr: String,
    #[serde(default, rename = "turnUser")]
    pub turn_user: String,
    #[serde(default, rename = "turnCred")]
    pub turn_cred: String,
}

#[derive(Debug, Clone, PartialEq, Eq, serde::Serialize)]
pub struct ClientIceServer {
    pub urls: Vec<String>,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub username: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub credential: String,
}

impl NanoKvmConfig {
    pub fn load() -> anyhow::Result<Self> {
        let path = std::path::Path::new("/etc/kvm/server.yaml");

        match fs::read_to_string(path) {
            Ok(raw) => serde_yaml::from_str(&raw)
                .with_context(|| format!("parse NanoKVM config {}", path.display())),
            Err(err) if err.kind() == std::io::ErrorKind::NotFound => Ok(Self {
                stun: "stun.l.google.com:19302".to_owned(),
                ..Self::default()
            }),
            Err(err) => Err(err).with_context(|| format!("read NanoKVM config {}", path.display())),
        }
    }

    pub fn client_ice_servers(&self) -> Vec<ClientIceServer> {
        let mut servers = Vec::new();

        if !self.stun.is_empty() && self.stun != "disable" {
            servers.push(ClientIceServer {
                urls: vec![format!("stun:{}", self.stun)],
                username: String::new(),
                credential: String::new(),
            });
        }

        if !self.turn.turn_addr.is_empty()
            && !self.turn.turn_user.is_empty()
            && !self.turn.turn_cred.is_empty()
        {
            servers.push(ClientIceServer {
                urls: vec![format!("turn:{}", self.turn.turn_addr)],
                username: self.turn.turn_user.clone(),
                credential: self.turn.turn_cred.clone(),
            });
        }

        servers
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn converts_stun_turn_for_browser_and_webrtc() {
        let config = NanoKvmConfig {
            stun: "stun.example:19302".to_owned(),
            turn: TurnConfig {
                turn_addr: "turn.example:3478".to_owned(),
                turn_user: "user".to_owned(),
                turn_cred: "pass".to_owned(),
            },
        };

        let client = config.client_ice_servers();
        assert_eq!(client[0].urls, vec!["stun:stun.example:19302"]);
        assert_eq!(client[1].urls, vec!["turn:turn.example:3478"]);
        assert_eq!(client[1].username, "user");
        assert_eq!(client[1].credential, "pass");
    }

    #[test]
    fn disable_stun_omits_stun_server() {
        let config = NanoKvmConfig {
            stun: "disable".to_owned(),
            ..Default::default()
        };
        assert!(config.client_ice_servers().is_empty());
    }
}
