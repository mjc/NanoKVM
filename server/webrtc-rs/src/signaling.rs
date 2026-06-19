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
}
