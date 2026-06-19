use std::fs;
use std::time::Duration;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct Screen {
    pub width: u16,
    pub height: u16,
    pub fps: u64,
    pub bitrate: u16,
}

impl Screen {
    pub fn read() -> Self {
        let height = read_u16("/kvmapp/kvm/res").unwrap_or(0);
        let (width, height) = resolution(height).unwrap_or((0, 0));
        let fps = read_u64("/kvmapp/kvm/fps").map(validate_fps).unwrap_or(30);
        let bitrate = read_u16("/kvmapp/kvm/qlty")
            .filter(|value| *value > 100)
            .filter(|value| matches!(*value, 1000 | 2000 | 3000 | 5000))
            .unwrap_or(3000);

        Self {
            width,
            height,
            fps,
            bitrate,
        }
    }

    pub fn frame_duration(self) -> Duration {
        Duration::from_millis(1000 / self.fps.max(1))
    }
}

fn read_u16(path: &str) -> Option<u16> {
    fs::read_to_string(path).ok()?.trim().parse().ok()
}

fn read_u64(path: &str) -> Option<u64> {
    fs::read_to_string(path).ok()?.trim().parse().ok()
}

fn resolution(height: u16) -> Option<(u16, u16)> {
    match height {
        1080 => Some((1920, 1080)),
        720 => Some((1280, 720)),
        600 => Some((800, 600)),
        480 => Some((640, 480)),
        0 => Some((0, 0)),
        _ => None,
    }
}

fn validate_fps(fps: u64) -> u64 {
    fps.clamp(10, 60)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn validates_known_resolutions_and_fps() {
        assert_eq!(resolution(1080), Some((1920, 1080)));
        assert_eq!(resolution(123), None);
        assert_eq!(validate_fps(5), 10);
        assert_eq!(validate_fps(120), 60);
    }

    #[test]
    fn frame_duration_never_divides_by_zero() {
        assert_eq!(Screen { width: 0, height: 0, fps: 0, bitrate: 0 }.frame_duration(), Duration::from_millis(1000));
    }

    #[test]
    fn frame_duration_tracks_fps() {
        assert_eq!(Screen { width: 1920, height: 1080, fps: 25, bitrate: 3000 }.frame_duration(), Duration::from_millis(40));
    }
}
