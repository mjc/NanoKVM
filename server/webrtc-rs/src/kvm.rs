use anyhow::{anyhow, Context};
use std::path::{Path, PathBuf};
use std::slice;
use std::time::Duration;

#[derive(Debug)]
pub struct EncodedFrame {
    pub data: Vec<u8>,
    pub result: i32,
}

fn finalize_h264_frame<F>(
    result: i32,
    data_ptr: *mut u8,
    data_size: u32,
    mut free_data: F,
) -> anyhow::Result<Option<EncodedFrame>>
where
    F: FnMut(*mut *mut u8) -> i32,
{
    if result < 0 {
        return Ok(Some(EncodedFrame {
            data: Vec::new(),
            result,
        }));
    }

    let data = if data_ptr.is_null() || data_size == 0 {
        Vec::new()
    } else {
        let data = unsafe { slice::from_raw_parts(data_ptr, data_size as usize).to_vec() };
        let mut data_ptr = data_ptr;
        let free_result = free_data(&mut data_ptr);
        if free_result < 0 {
            return Err(anyhow!("free_kvmv_data returned {free_result}"))
                .context("release KVM H.264 frame");
        }
        data
    };

    Ok(Some(EncodedFrame { data, result }))
}

fn dl_lib_path_from_exe(exe: &Path, library_name: &str) -> anyhow::Result<PathBuf> {
    Ok(exe
        .parent()
        .ok_or_else(|| anyhow!("current executable has no parent directory"))?
        .join("dl_lib")
        .join(library_name))
}

fn initialize_kvm_hardware<FInit, FControl, FControlReturn, FSleep>(
    hdmi_disabled: bool,
    mut init: FInit,
    mut hdmi_control: FControl,
    mut sleep: FSleep,
)
where
    FInit: FnMut(u8),
    FControl: FnMut(u8) -> FControlReturn,
    FSleep: FnMut(Duration),
{
    init(0);
    let _ = hdmi_control(0);
    sleep(Duration::from_millis(10));

    if !hdmi_disabled {
        let _ = hdmi_control(1);
        sleep(Duration::from_secs(2));
    }
}

#[cfg(target_arch = "riscv64")]
mod imp {
    use std::env;
    use std::fs;
    use std::path::PathBuf;
    use std::ptr;
    use std::sync::mpsc;
    use std::sync::Mutex;
    use std::sync::OnceLock;
    use std::thread;

    use anyhow::{anyhow, Context};
    use libloading::Library;

    use super::{dl_lib_path_from_exe, finalize_h264_frame, initialize_kvm_hardware, EncodedFrame};

    const IMG_H264_TYPE: u8 = 1;

    static WORKER: OnceLock<Result<KvmWorker, String>> = OnceLock::new();

    type KvmvInit = unsafe extern "C" fn(debug_info_en: u8);
    type KvmvReadImg = unsafe extern "C" fn(
        width: u16,
        height: u16,
        img_type: u8,
        quality: u16,
        data: *mut *mut u8,
        data_size: *mut u32,
    ) -> i32;
    type FreeKvmvData = unsafe extern "C" fn(data: *mut *mut u8) -> i32;
    type KvmvHdmiControl = unsafe extern "C" fn(enable: u8) -> u8;

    enum KvmRequest {
        Read {
            width: u16,
            height: u16,
            bitrate: u16,
            reply: mpsc::Sender<anyhow::Result<Option<EncodedFrame>>>,
        },
    }

    struct KvmWorker {
        tx: Mutex<mpsc::Sender<KvmRequest>>,
    }

    impl KvmWorker {
        fn start() -> anyhow::Result<Self> {
            let lib_path = library_path().context("locate libkvm.so")?;
            let (tx, rx) = mpsc::channel();
            let (ready_tx, ready_rx) = mpsc::channel();

            thread::Builder::new()
                .name("nanokvm-kvm-vision".to_string())
                .spawn(move || {
                    let api = match KvmApi::load(&lib_path) {
                        Ok(api) => {
                            let _ = ready_tx.send(Ok(()));
                            api
                        }
                        Err(err) => {
                            let _ = ready_tx.send(Err(format!("{err:#}")));
                            return;
                        }
                    };

                    while let Ok(request) = rx.recv() {
                        match request {
                            KvmRequest::Read {
                                width,
                                height,
                                bitrate,
                                reply,
                            } => {
                                let _ = reply.send(api.read_h264(width, height, bitrate));
                            }
                        }
                    }
                })
                .context("start KVM vision worker thread")?;

            match ready_rx
                .recv()
                .context("wait for KVM vision worker initialization")?
            {
                Ok(()) => Ok(Self { tx: Mutex::new(tx) }),
                Err(err) => Err(anyhow!(err)),
            }
        }

        fn read_h264(
            &self,
            width: u16,
            height: u16,
            bitrate: u16,
        ) -> anyhow::Result<Option<EncodedFrame>> {
            let (reply, rx) = mpsc::channel();
            self.tx
                .lock()
                .map_err(|_| anyhow!("KVM worker lock poisoned"))?
                .send(KvmRequest::Read {
                    width,
                    height,
                    bitrate,
                    reply,
                })
                .context("send KVM read request")?;
            rx.recv().context("receive KVM read response")?
        }
    }

    struct KvmApi {
        _lib: Library,
        read_img: KvmvReadImg,
        free_data: FreeKvmvData,
    }

    unsafe impl Send for KvmApi {}

    impl KvmApi {
        fn load(path: &PathBuf) -> anyhow::Result<Self> {
            let lib = unsafe { Library::new(path) }
                .with_context(|| format!("load KVM vision library from {}", path.display()))?;
            let init = unsafe { *lib.get::<KvmvInit>(b"kvmv_init")? };
            let read_img = unsafe { *lib.get::<KvmvReadImg>(b"kvmv_read_img")? };
            let free_data = unsafe { *lib.get::<FreeKvmvData>(b"free_kvmv_data")? };
            let hdmi_control = unsafe { *lib.get::<KvmvHdmiControl>(b"kvmv_hdmi_control")? };

            let hdmi_disabled = fs::metadata("/etc/kvm/hdmi_disable").is_ok();
            initialize_kvm_hardware(
                hdmi_disabled,
                |debug_info_en| unsafe { init(debug_info_en) },
                |enable| unsafe { hdmi_control(enable) },
                thread::sleep,
            );

            Ok(Self {
                _lib: lib,
                read_img,
                free_data,
            })
        }

        fn read_h264(
            &self,
            width: u16,
            height: u16,
            bitrate: u16,
        ) -> anyhow::Result<Option<EncodedFrame>> {
            let mut data_ptr: *mut u8 = ptr::null_mut();
            let mut data_size: u32 = 0;
            let result = unsafe {
                (self.read_img)(
                    width,
                    height,
                    IMG_H264_TYPE,
                    bitrate,
                    &mut data_ptr,
                    &mut data_size,
                )
            };

            if result < 0 {
                return finalize_h264_frame(result, data_ptr, data_size, |_| 0);
            }

            finalize_h264_frame(result, data_ptr, data_size, |ptr| unsafe {
                (self.free_data)(ptr)
            })
        }
    }

    #[derive(Clone, Copy)]
    pub struct KvmVision {
        worker: &'static KvmWorker,
    }

    impl KvmVision {
        pub fn new() -> anyhow::Result<Self> {
            match WORKER.get_or_init(|| KvmWorker::start().map_err(|err| format!("{err:#}"))) {
                Ok(worker) => Ok(Self { worker }),
                Err(err) => Err(anyhow!(err.clone())),
            }
        }

        pub fn read_h264(
            &self,
            width: u16,
            height: u16,
            bitrate: u16,
        ) -> anyhow::Result<Option<EncodedFrame>> {
            self.worker.read_h264(width, height, bitrate)
        }
    }

    fn library_path() -> anyhow::Result<PathBuf> {
        let exe = env::current_exe().context("read current executable path")?;
        dl_lib_path_from_exe(&exe, "libkvm.so")
    }

    pub use KvmVision as PlatformKvmVision;
}

#[cfg(not(target_arch = "riscv64"))]
mod imp {
    use super::EncodedFrame;

    #[derive(Debug, Default, Clone, Copy)]
    pub struct KvmVision;

    impl KvmVision {
        pub fn new() -> anyhow::Result<Self> {
            Ok(Self)
        }

        pub fn read_h264(
            &self,
            _width: u16,
            _height: u16,
            _bitrate: u16,
        ) -> anyhow::Result<Option<EncodedFrame>> {
            Ok(None)
        }
    }

    pub use KvmVision as PlatformKvmVision;
}

pub use imp::PlatformKvmVision as KvmVision;

#[cfg(test)]
mod tests {
    use super::*;
    use std::cell::RefCell;
    use std::path::Path;
    use std::sync::atomic::{AtomicUsize, Ordering};

    #[test]
    fn positive_h264_frame_is_copied_and_freed_once() {
        let mut data = vec![0x67, 0x11, 0x22, 0x33];
        let data_ptr = data.as_mut_ptr();
        let data_len = data.len() as u32;
        std::mem::forget(data);

        let free_calls = AtomicUsize::new(0);
        let frame = finalize_h264_frame(3, data_ptr, data_len, |_| {
            free_calls.fetch_add(1, Ordering::SeqCst);
            0
        })
        .unwrap()
        .unwrap();

        assert_eq!(frame.result, 3);
        assert_eq!(frame.data, vec![0x67, 0x11, 0x22, 0x33]);
        assert_eq!(free_calls.load(Ordering::SeqCst), 1);
    }

    #[test]
    fn zero_length_h264_frame_skips_free_and_returns_empty_payload() {
        let free_calls = AtomicUsize::new(0);
        let frame = finalize_h264_frame(4, std::ptr::null_mut(), 0, |_| {
            free_calls.fetch_add(1, Ordering::SeqCst);
            0
        })
        .unwrap()
        .unwrap();

        assert_eq!(frame.result, 4);
        assert!(frame.data.is_empty());
        assert_eq!(free_calls.load(Ordering::SeqCst), 0);
    }

    #[test]
    fn negative_h264_frame_skips_free_and_returns_empty_payload() {
        let free_calls = AtomicUsize::new(0);
        let frame = finalize_h264_frame(-3, 0xdead_beef as *mut u8, 4, |_| {
            free_calls.fetch_add(1, Ordering::SeqCst);
            0
        })
        .unwrap()
        .unwrap();

        assert_eq!(frame.result, -3);
        assert!(frame.data.is_empty());
        assert_eq!(free_calls.load(Ordering::SeqCst), 0);
    }

    #[test]
    fn dl_lib_path_from_exe_uses_sibling_dl_lib_directory() {
        assert_eq!(
            dl_lib_path_from_exe(Path::new("/data/nanokvm-webrtc-rs"), "libkvm.so").unwrap(),
            Path::new("/data/dl_lib/libkvm.so")
        );
    }

    #[test]
    fn dl_lib_path_from_exe_maps_companion_library_name() {
        assert_eq!(
            dl_lib_path_from_exe(Path::new("/data/nanokvm-webrtc-rs"), "libkvm_mmf.so").unwrap(),
            Path::new("/data/dl_lib/libkvm_mmf.so")
        );
    }

    #[test]
    fn initialize_kvm_hardware_matches_go_prep_sequence_when_hdmi_is_enabled() {
        let calls = RefCell::new(Vec::new());
        initialize_kvm_hardware(
            false,
            |debug_info_en| calls.borrow_mut().push(format!("init:{debug_info_en}")),
            |enable| calls.borrow_mut().push(format!("hdmi:{enable}")),
            |duration| calls.borrow_mut().push(format!("sleep:{}", duration.as_millis())),
        );

        assert_eq!(
            calls.into_inner(),
            vec![
                "init:0".to_owned(),
                "hdmi:0".to_owned(),
                "sleep:10".to_owned(),
                "hdmi:1".to_owned(),
                "sleep:2000".to_owned(),
            ]
        );
    }

    #[test]
    fn initialize_kvm_hardware_skips_hdmi_reenable_when_disabled() {
        let calls = RefCell::new(Vec::new());
        initialize_kvm_hardware(
            true,
            |debug_info_en| calls.borrow_mut().push(format!("init:{debug_info_en}")),
            |enable| calls.borrow_mut().push(format!("hdmi:{enable}")),
            |duration| calls.borrow_mut().push(format!("sleep:{}", duration.as_millis())),
        );

        assert_eq!(
            calls.into_inner(),
            vec![
                "init:0".to_owned(),
                "hdmi:0".to_owned(),
                "sleep:10".to_owned(),
            ]
        );
    }

    #[test]
    fn initialize_kvm_hardware_accepts_hdmi_control_status_return() {
        let calls = RefCell::new(Vec::new());
        initialize_kvm_hardware(
            false,
            |debug_info_en| calls.borrow_mut().push(format!("init:{debug_info_en}")),
            |enable| {
                calls.borrow_mut().push(format!("hdmi:{enable}"));
                enable
            },
            |duration| calls.borrow_mut().push(format!("sleep:{}", duration.as_millis())),
        );

        assert_eq!(
            calls.into_inner(),
            vec![
                "init:0".to_owned(),
                "hdmi:0".to_owned(),
                "sleep:10".to_owned(),
                "hdmi:1".to_owned(),
                "sleep:2000".to_owned(),
            ]
        );
    }
}
