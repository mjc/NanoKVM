package config

import "testing"

func TestWiFiInitSecurityContracts(t *testing.T) {
	cases := []sourceSecurityContract{
		{
			name:       "S30WiFiAPFlagUsesPredictableTmpPath",
			path:       "../../kvmapp/system/init.d/S30wifi",
			vulnerable: []string{`APFLAG_FILE=/tmp/wifiap`},
			message:    "AP-mode state should not be represented by a predictable /tmp flag",
		},
		{
			name:       "S30WiFiHostapdConfigEchoesRawSSID",
			path:       "../../kvmapp/system/init.d/S30wifi",
			vulnerable: []string{`echo "ssid=${ssid}"`},
			message:    "hostapd config generation should escape or validate SSID values before writing config syntax",
		},
		{
			name:       "S30WiFiHostapdConfigEchoesRawPassphrase",
			path:       "../../kvmapp/system/init.d/S30wifi",
			vulnerable: []string{`echo "wpa_passphrase=${pass}"`},
			message:    "hostapd config generation should escape or validate AP passphrases before writing config syntax",
		},
		{
			name:       "S30WiFiAPAllowsTooManyStations",
			path:       "../../kvmapp/system/init.d/S30wifi",
			vulnerable: []string{`echo "max_num_sta=255"`},
			message:    "AP mode should cap associated clients to a small management surface",
		},
		{
			name:       "S30WiFiDHCPLeaseTooLongForAPMode",
			path:       "../../kvmapp/system/init.d/S30wifi",
			vulnerable: []string{`echo "option lease 864000"`},
			message:    "AP-mode DHCP leases should be short-lived for recovery networks",
		},
		{
			name:       "S30WiFiBootMigrationDeletesExistingCredentials",
			path:       "../../kvmapp/system/init.d/S30wifi",
			vulnerable: []string{`rm -f /etc/kvm/wifi.ssid /etc/kvm/wifi.pass`},
			message:    "boot credential migration should not delete existing credentials before validating replacements",
		},
		{
			name:       "S30WiFiBootSSIDMoveNotSymlinkSafe",
			path:       "../../kvmapp/system/init.d/S30wifi",
			vulnerable: []string{`mv /boot/wifi.ssid /etc/kvm/wifi.ssid`},
			message:    "boot SSID migration should validate file type and ownership before replacing /etc/kvm state",
		},
		{
			name:       "S30WiFiBootPasswordMoveNotSymlinkSafe",
			path:       "../../kvmapp/system/init.d/S30wifi",
			vulnerable: []string{`mv /boot/wifi.pass /etc/kvm/wifi.pass`},
			message:    "boot Wi-Fi password migration should validate file type and ownership before replacing secrets",
		},
		{
			name:       "S30WiFiBootPasswordWorldReadable",
			path:       "../../kvmapp/system/init.d/S30wifi",
			vulnerable: []string{`chmod 644 /etc/kvm/wifi.ssid /etc/kvm/wifi.pass`},
			message:    "Wi-Fi password migration should not set secrets world-readable",
		},
		{
			name:       "S30WiFiReadsStationPasswordWithCat",
			path:       "../../kvmapp/system/init.d/S30wifi",
			vulnerable: []string{`pass=$(cat /etc/kvm/wifi.pass)`},
			message:    "station password reads should verify a regular root-owned secret file before reading",
		},
		{
			name:       "S30WiFiWritesWPASupplicantWithPlainPSK",
			path:       "../../kvmapp/system/init.d/S30wifi",
			vulnerable: []string{`wpa_passphrase "$ssid" "$pass" >>/etc/wpa_supplicant.conf`},
			message:    "wpa_supplicant output containing PSK material should be written atomically with restricted permissions",
		},
		{
			name:       "S30WiFiReadsAPPasswordFromKvmApp",
			path:       "../../kvmapp/system/init.d/S30wifi",
			vulnerable: []string{`pass=$(cat /kvmapp/kvm/ap.pass)`},
			message:    "AP password reads should validate a protected secret file instead of blindly catting app storage",
		},
		{
			name:       "S30WiFiWritesHostapdConfigWithPlainPassphrase",
			path:       "../../kvmapp/system/init.d/S30wifi",
			vulnerable: []string{`gen_hostapd_conf "$ssid" "$pass" >/etc/hostapd.conf`},
			message:    "hostapd config containing AP passphrase should be written atomically with mode 0600",
		},
		{
			name:       "S30WiFiAPDeletesDefaultRoute",
			path:       "../../kvmapp/system/init.d/S30wifi",
			vulnerable: []string{`ip route del default || true`},
			message:    "AP mode should not delete default routes without preserving and restoring network state",
		},
		{
			name:       "S30WiFiAPFlushesInterfaceAddresses",
			path:       "../../kvmapp/system/init.d/S30wifi",
			vulnerable: []string{`ip add flush dev wlan0`},
			message:    "AP mode should avoid broad address flushes without an audited network transition",
		},
		{
			name:       "S30WiFiStartsHostapdWithGeneratedConfig",
			path:       "../../kvmapp/system/init.d/S30wifi",
			vulnerable: []string{`hostapd -B -i wlan0 /etc/hostapd.conf`},
			message:    "AP service start should verify generated config ownership and permissions before launching",
		},
		{
			name:       "S30WiFiStartsUDHCPDWithoutLeaseIsolation",
			path:       "../../kvmapp/system/init.d/S30wifi",
			vulnerable: []string{`udhcpd -S /etc/udhcpd.wlan0.conf`},
			message:    "AP DHCP service should use isolated lease/pid files with protected permissions",
		},
		{
			name:       "S30WiFiStopKillsHostapdByGrep",
			path:       "../../kvmapp/system/init.d/S30wifi",
			vulnerable: []string{`ps -ef | grep hostapd | grep -v grep | awk '{print $1}' | xargs kill -2 || true`},
			message:    "Wi-Fi stop should use tracked pid files instead of grep-based process killing",
		},
		{
			name:       "S30WiFiStopKillsUDHCPDByGrep",
			path:       "../../kvmapp/system/init.d/S30wifi",
			vulnerable: []string{`ps -ef | grep "udhcpd -S /etc/udhcpd.wlan0.conf" | grep -v grep | awk '{print $1}' | xargs kill -2 || true`},
			message:    "DHCP stop should use tracked pid files instead of grep-based process killing",
		},
		{
			name:       "S30WiFiStopKillsPidFileWithoutValidation",
			path:       "../../kvmapp/system/init.d/S30wifi",
			vulnerable: []string{`kill $(cat /run/udhcpc.wlan0.pid) || true`},
			message:    "Wi-Fi stop should validate pid files before passing their contents to kill",
		},
		{
			name:       "S30WiFiStopRunsAirmonOnPredictableInterface",
			path:       "../../kvmapp/system/init.d/S30wifi",
			vulnerable: []string{`airmon-ng stop wlan0mon || true`},
			message:    "Wi-Fi stop should avoid unaudited monitor-mode cleanup on predictable interface names",
		},
	}

	runSourceSecurityContracts(t, cases, 21, "wifi-init")
}
