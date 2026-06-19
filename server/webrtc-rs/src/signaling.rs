use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct Message {
    pub event: String,
    pub data: String,
}

impl Message {
    pub fn new(event: impl Into<String>, data: impl Into<String>) -> Self {
        Self {
            event: event.into(),
            data: data.into(),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn message_wire_shape_matches_frontend_contract() {
        let msg = Message::new("video-offer", "{\"type\":\"offer\"}");
        let json = serde_json::to_string(&msg).unwrap();
        assert_eq!(
            json,
            r#"{"event":"video-offer","data":"{\"type\":\"offer\"}"}"#
        );
        assert_eq!(serde_json::from_str::<Message>(&json).unwrap(), msg);
    }

    #[test]
    fn message_keeps_empty_payloads_as_empty_strings() {
        let msg = Message::new("heartbeat", "");
        let json = serde_json::to_string(&msg).unwrap();
        assert_eq!(json, r#"{"event":"heartbeat","data":""}"#);
    }

    #[test]
    fn message_round_trips_unicode_and_escaped_json() {
        let msg = Message::new("video-answer", r#"{"type":"answer","sdp":"a=b\nc"}"#);
        let decoded: Message = serde_json::from_str(&serde_json::to_string(&msg).unwrap()).unwrap();
        assert_eq!(decoded, msg);
    }
}
