#include "kvm_vision.h"
#include "maix_basic.hpp"

#include <signal.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

using namespace maix;
using namespace maix::sys;

struct smoke_config {
    uint16_t width = 1920;
    uint16_t height = 1080;
    uint8_t type = 1;
    uint16_t quality = 3000;
    uint8_t gop = 30;
    int frames = 30;
    int warmup = 5;
    int max_failures = 60;
    int delay_ms = 20;
    uint8_t debug = 0;
    bool cycle_hdmi = true;
};

static void usage(const char *argv0)
{
    printf("Usage: %s [options]\n", argv0);
    printf("\n");
    printf("Options:\n");
    printf("  --frames N        Successful encoded frames required. Default: 30\n");
    printf("  --warmup N        Initial successful frames to ignore. Default: 5\n");
    printf("  --max-failures N  Failed/empty reads allowed before failing. Default: 60\n");
    printf("  --width N         Requested output width. Default: 1920\n");
    printf("  --height N        Requested output height. Default: 1080\n");
    printf("  --type mjpeg|h264 Encode type. Default: h264\n");
    printf("  --quality N       MJPEG quality or H264 bitrate. Default: 3000\n");
    printf("  --gop N           H264 GOP. Default: 30\n");
    printf("  --delay-ms N      Delay between read attempts. Default: 20\n");
    printf("  --debug           Enable kvm_vision debug logging\n");
    printf("  --no-hdmi-cycle   Do not power-cycle HDMI before init\n");
    printf("  -h, --help        Show this help\n");
}

static int parse_int_arg(const char *name, const char *value, int min_value, int max_value)
{
    char *end = NULL;
    long parsed;

    if (value == NULL || value[0] == '\0') {
        fprintf(stderr, "missing value for %s\n", name);
        exit(2);
    }

    parsed = strtol(value, &end, 10);
    if (*end != '\0' || parsed < min_value || parsed > max_value) {
        fprintf(stderr, "invalid %s: %s\n", name, value);
        exit(2);
    }

    return (int)parsed;
}

static smoke_config parse_args(int argc, char **argv)
{
    smoke_config cfg;

    for (int i = 1; i < argc; i++) {
        if (strcmp(argv[i], "--frames") == 0) {
            cfg.frames = parse_int_arg("--frames", argv[++i], 1, 100000);
        } else if (strcmp(argv[i], "--warmup") == 0) {
            cfg.warmup = parse_int_arg("--warmup", argv[++i], 0, 100000);
        } else if (strcmp(argv[i], "--max-failures") == 0) {
            cfg.max_failures = parse_int_arg("--max-failures", argv[++i], 0, 100000);
        } else if (strcmp(argv[i], "--width") == 0) {
            cfg.width = (uint16_t)parse_int_arg("--width", argv[++i], 0, 65535);
        } else if (strcmp(argv[i], "--height") == 0) {
            cfg.height = (uint16_t)parse_int_arg("--height", argv[++i], 0, 65535);
        } else if (strcmp(argv[i], "--type") == 0) {
            const char *type = argv[++i];
            if (type == NULL) {
                fprintf(stderr, "missing value for --type\n");
                exit(2);
            }
            if (strcmp(type, "mjpeg") == 0) {
                cfg.type = 0;
                cfg.quality = 60;
            } else if (strcmp(type, "h264") == 0) {
                cfg.type = 1;
            } else {
                fprintf(stderr, "invalid --type: %s\n", type);
                exit(2);
            }
        } else if (strcmp(argv[i], "--quality") == 0) {
            cfg.quality = (uint16_t)parse_int_arg("--quality", argv[++i], 1, 65535);
        } else if (strcmp(argv[i], "--gop") == 0) {
            cfg.gop = (uint8_t)parse_int_arg("--gop", argv[++i], 1, 255);
        } else if (strcmp(argv[i], "--delay-ms") == 0) {
            cfg.delay_ms = parse_int_arg("--delay-ms", argv[++i], 0, 60000);
        } else if (strcmp(argv[i], "--debug") == 0) {
            cfg.debug = 1;
        } else if (strcmp(argv[i], "--no-hdmi-cycle") == 0) {
            cfg.cycle_hdmi = false;
        } else if (strcmp(argv[i], "-h") == 0 || strcmp(argv[i], "--help") == 0) {
            usage(argv[0]);
            exit(0);
        } else {
            fprintf(stderr, "unknown option: %s\n", argv[i]);
            usage(argv[0]);
            exit(2);
        }
    }

    return cfg;
}

static bool ret_has_frame(int ret, uint8_t type)
{
    if (type == 0) {
        return ret == IMG_MJPEG_TYPE;
    }
    return ret == IMG_H264_TYPE_IF || ret == IMG_H264_TYPE_PF;
}

int main(int argc, char *argv[])
{
    smoke_config cfg = parse_args(argc, argv);
    int successful = 0;
    int warmup_seen = 0;
    int failures = 0;
    uint32_t min_size = UINT32_MAX;
    uint32_t max_size = 0;
    uint64_t total_size = 0;

    signal(SIGINT, [](int) {
        app::set_exit_flag(true);
    });

    printf("kvm_vision_smoke: frames=%d warmup=%d type=%s width=%u height=%u quality=%u gop=%u\n",
           cfg.frames,
           cfg.warmup,
           cfg.type == 0 ? "mjpeg" : "h264",
           cfg.width,
           cfg.height,
           cfg.quality,
           cfg.gop);

    if (cfg.cycle_hdmi) {
        printf("kvm_vision_smoke: cycling HDMI\n");
        kvmv_hdmi_control(0);
        time::sleep_ms(100);
        kvmv_hdmi_control(1);
        time::sleep_ms(100);
    }

    kvmv_init(cfg.debug);
    set_h264_gop(cfg.gop);
    set_venc_auto_recyc(1);

    while (!app::need_exit() && successful < cfg.frames) {
        uint8_t *data = NULL;
        uint32_t size = 0;
        uint64_t start = time::time_ms();
        int ret = kvmv_read_img(cfg.width, cfg.height, cfg.type, cfg.quality, &data, &size);
        uint64_t elapsed = time::time_ms() - start;

        if (ret_has_frame(ret, cfg.type) && data != NULL && size > 0) {
            if (warmup_seen < cfg.warmup) {
                warmup_seen++;
                printf("kvm_vision_smoke: warmup ret=%d size=%u elapsed_ms=%llu\n",
                       ret,
                       size,
                       (unsigned long long)elapsed);
            } else {
                successful++;
                total_size += size;
                if (size < min_size) {
                    min_size = size;
                }
                if (size > max_size) {
                    max_size = size;
                }
                printf("kvm_vision_smoke: frame=%d ret=%d size=%u elapsed_ms=%llu\n",
                       successful,
                       ret,
                       size,
                       (unsigned long long)elapsed);
            }
            free_kvmv_data(&data);
        } else {
            failures++;
            printf("kvm_vision_smoke: read_failed ret=%d size=%u data=%p failures=%d/%d elapsed_ms=%llu\n",
                   ret,
                   size,
                   data,
                   failures,
                   cfg.max_failures,
                   (unsigned long long)elapsed);
            if (data != NULL) {
                free_kvmv_data(&data);
            }
            if (failures > cfg.max_failures) {
                fprintf(stderr, "kvm_vision_smoke: too many read failures\n");
                kvmv_deinit();
                return 1;
            }
        }

        if (cfg.delay_ms > 0) {
            time::sleep_ms(cfg.delay_ms);
        }
    }

    printf("kvm_vision_smoke: deinit begin\n");
    fflush(stdout);
    kvmv_deinit();
    printf("kvm_vision_smoke: deinit end\n");
    fflush(stdout);

    if (successful < cfg.frames) {
        fprintf(stderr, "kvm_vision_smoke: interrupted after %d/%d frames\n", successful, cfg.frames);
        return 1;
    }

    printf("kvm_vision_smoke: ok frames=%d failures=%d min_size=%u max_size=%u avg_size=%llu\n",
           successful,
           failures,
           min_size,
           max_size,
           (unsigned long long)(total_size / successful));
    return 0;
}
