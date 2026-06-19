use util::{Marshal, MarshalSize};

pub const URI: &str = "http://www.webrtc.org/experiments/rtp-hdrext/playout-delay";

#[derive(Debug, Clone, Copy)]
pub struct PlayoutDelay {
    pub min_delay: u16,
    pub max_delay: u16,
}

impl Default for PlayoutDelay {
    fn default() -> Self {
        Self {
            min_delay: 0,
            max_delay: 0,
        }
    }
}

impl MarshalSize for PlayoutDelay {
    fn marshal_size(&self) -> usize {
        3
    }
}

impl Marshal for PlayoutDelay {
    fn marshal_to(&self, buf: &mut [u8]) -> util::Result<usize> {
        if buf.len() < self.marshal_size() {
            return Err(util::Error::ErrBufferShort);
        }

        let min_delay = self.min_delay & 0x0fff;
        let max_delay = self.max_delay & 0x0fff;
        let value = ((min_delay as u32) << 12) | max_delay as u32;
        buf[0] = ((value >> 16) & 0xff) as u8;
        buf[1] = ((value >> 8) & 0xff) as u8;
        buf[2] = (value & 0xff) as u8;
        Ok(3)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn zero_delay_marshals_to_three_zero_bytes() {
        let mut buf = [0xff; 3];
        let written = PlayoutDelay::default().marshal_to(&mut buf).unwrap();
        assert_eq!(written, 3);
        assert_eq!(buf, [0, 0, 0]);
    }

    #[test]
    fn packs_min_and_max_as_twelve_bit_values() {
        let mut buf = [0; 3];
        PlayoutDelay {
            min_delay: 0x012,
            max_delay: 0x345,
        }
        .marshal_to(&mut buf)
        .unwrap();
        assert_eq!(buf, [0x01, 0x23, 0x45]);
    }
}
