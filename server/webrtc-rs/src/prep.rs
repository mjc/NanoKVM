use std::fs;
use std::path::Path;
use std::process::Command;

use tracing::{debug, error, info, warn};

const EXPECTED_MIPI_RX_MD5: [&str; 2] = [
    "086ed01749188975afaa40fb569374f8",
    "69be7eeded3777f750480a5dd5a1aa26",
];

pub fn run() {
    if !Path::new("/kvmapp").exists() {
        debug!("skipping NanoKVM hardware prep outside target filesystem");
        return;
    }

    info!("running NanoKVM hardware prep");
    copy_sensor_config();
    write_file("/kvmapp/kvm/type", "h264\n");
    remove_path("/kvmapp/jpg_stream");
    remove_path("/kvmapp/kvm_system/kvm_stream");
    ensure_mipi_rx_module();
    run_script("/kvmapp/system/init.d/S15kvmhwd", "get_hdmi_version");
    run_script("/kvmapp/system/init.d/S15kvmhwd", "start");
}

fn copy_sensor_config() {
    let source = Path::new("/mnt/data/sensor_cfg.ini.LT");
    let target = Path::new("/mnt/data/sensor_cfg.ini");
    if !source.exists() {
        return;
    }

    if let Err(err) = fs::copy(source, target) {
        warn!(
            "failed to copy {} to {}: {err}",
            source.display(),
            target.display()
        );
    }
}

fn write_file(path: &str, content: &str) {
    if let Err(err) = fs::write(path, content) {
        warn!("failed to write {path}: {err}");
    }
}

fn remove_path(path: &str) {
    let path = Path::new(path);
    if !path.exists() {
        return;
    }

    let result = if path.is_dir() {
        fs::remove_dir_all(path)
    } else {
        fs::remove_file(path)
    };

    if let Err(err) = result {
        warn!("failed to remove {}: {err}", path.display());
    }
}

fn ensure_mipi_rx_module() {
    let target = Path::new("/mnt/system/ko/soph_mipi_rx.ko");
    let source = Path::new("/kvmapp/system/ko/soph_mipi_rx.ko");

    if !source.exists() {
        return;
    }

    match md5sum(target) {
        Some(md5) if EXPECTED_MIPI_RX_MD5.contains(&md5.as_str()) => {
            debug!("soph_mipi_rx.ko already has expected md5 {md5}");
        }
        Some(md5) => {
            warn!("replacing soph_mipi_rx.ko with unexpected md5 {md5}");
            copy_module(source, target);
        }
        None => {
            warn!("replacing soph_mipi_rx.ko because md5 could not be read");
            copy_module(source, target);
        }
    }
}

fn copy_module(source: &Path, target: &Path) {
    if let Err(err) = fs::copy(source, target) {
        warn!(
            "failed to copy {} to {}: {err}",
            source.display(),
            target.display()
        );
    }
}

fn md5sum(path: &Path) -> Option<String> {
    let output = Command::new("md5sum").arg(path).output().ok()?;
    if !output.status.success() {
        return None;
    }

    let stdout = String::from_utf8(output.stdout).ok()?;
    stdout.split_whitespace().next().map(ToString::to_string)
}

fn run_script(script: &str, arg: &str) {
    if !Path::new(script).exists() {
        return;
    }

    match Command::new(script).arg(arg).status() {
        Ok(status) if status.success() => debug!("{script} {arg} completed"),
        Ok(status) => warn!("{script} {arg} exited with {status}"),
        Err(err) => error!("failed to run {script} {arg}: {err}"),
    }
}
