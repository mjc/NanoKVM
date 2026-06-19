#[derive(Debug)]
pub struct EncodedFrame {
    pub data: Vec<u8>,
    pub result: i32,
}

#[cfg(target_arch = "riscv64")]
mod imp {
    use std::env;
    use std::fs;
    use std::path::PathBuf;
    use std::ptr;
    use std::slice;
    use std::sync::mpsc;
    use std::sync::Mutex;
    use std::sync::OnceLock;
    use std::thread;
    use std::time::Duration;

    use anyhow::{anyhow, Context};
    use libloading::Library;

    use super::EncodedFrame;

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

            unsafe {
                init(0);
                hdmi_control(0);
            }
            thread::sleep(Duration::from_millis(10));
            if fs::metadata("/etc/kvm/hdmi_disable").is_err() {
                unsafe {
                    hdmi_control(1);
                }
                thread::sleep(Duration::from_secs(2));
            }

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
                return Ok(Some(EncodedFrame {
                    data: Vec::new(),
                    result,
                }));
            }

            let data = if data_ptr.is_null() || data_size == 0 {
                Vec::new()
            } else {
                let data = unsafe { slice::from_raw_parts(data_ptr, data_size as usize).to_vec() };
                let free_result = unsafe { (self.free_data)(&mut data_ptr) };
                if free_result < 0 {
                    return Err(anyhow!("free_kvmv_data returned {free_result}"))
                        .context("release KVM H.264 frame");
                }
                data
            };

            Ok(Some(EncodedFrame { data, result }))
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
        Ok(exe
            .parent()
            .ok_or_else(|| anyhow!("current executable has no parent directory"))?
            .join("dl_lib/libkvm.so"))
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
