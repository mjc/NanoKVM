package config

import "testing"

func TestNetworkControlSecurityContracts(t *testing.T) {
	cases := []sourceSecurityContract{
		{
			name:       "WOLCommandOutputReturnedToBrowser",
			path:       "../service/network/wol.go",
			vulnerable: []string{`rsp.ErrRsp(c, -3, string(output))`},
			message:    "wake-on-LAN command output should not be returned to the browser",
		},
		{
			name:       "WOLLogsRequestedMAC",
			path:       "../service/network/wol.go",
			vulnerable: []string{`log.Debugf("wake on lan: %s", mac)`},
			message:    "wake-on-LAN should avoid logging target device identifiers",
		},
		{
			name:       "WOLStoreCreatesWorldSearchableDirectory",
			path:       "../service/network/wol.go",
			vulnerable: []string{`os.MkdirAll(filepath.Dir(WolMacFile), 0o755)`},
			message:    "stored wake-on-LAN MAC directory should not be world-searchable",
		},
		{
			name:       "WOLStoreCreatesWorldReadableFile",
			path:       "../service/network/wol.go",
			vulnerable: []string{`os.OpenFile(WolMacFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)`},
			message:    "stored wake-on-LAN MAC file should not be world-readable",
		},
		{
			name:       "WOLStoreReadsWholeFile",
			path:       "../service/network/wol.go",
			vulnerable: []string{`content, err := os.ReadFile(WolMacFile)`},
			message:    "wake-on-LAN MAC store should cap file size before reading",
		},
		{
			name:       "WOLWriteStoresWorldReadableFile",
			path:       "../service/network/wol.go",
			vulnerable: []string{`os.WriteFile(WolMacFile, []byte(data), 0o644)`},
			message:    "wake-on-LAN MAC writes should use owner-only permissions",
		},
		{
			name:       "WOLSetNameLogsRawRequestValues",
			path:       "../service/network/wol.go",
			vulnerable: []string{`log.Debugf("set wol mac name: %s %s", req.Mac, req.Name)`},
			message:    "wake-on-LAN labels should not log raw request values",
		},
		{
			name:       "WOLDeleteLogsRawRequestMAC",
			path:       "../service/network/wol.go",
			vulnerable: []string{`log.Debugf("delete wol mac: %s", req.Mac)`},
			message:    "wake-on-LAN deletion should not log raw device identifiers",
		},
		{
			name:       "DNSGetLogsFullNetworkConfig",
			path:       "../service/network/dns.go",
			vulnerable: []string{`log.Debugf("get dns config: mode=%s servers=%v effective=%v dhcp=%v info=%+v"`},
			message:    "DNS reads should not log full resolver and interface configuration",
		},
		{
			name:       "DNSSetLogsRequestedServers",
			path:       "../service/network/dns.go",
			vulnerable: []string{`log.Debugf("set dns config: mode=%s servers=%v", req.Mode, req.Servers)`},
			message:    "DNS writes should not log raw requested resolver lists",
		},
		{
			name:       "DNSManualCreatesWorldSearchableDirectory",
			path:       "../service/network/dns.go",
			vulnerable: []string{`os.MkdirAll(dnsConfigDir, 0o755)`},
			message:    "DNS config directory should not be world-searchable",
		},
		{
			name:       "DNSServersWrittenWorldReadable",
			path:       "../service/network/dns.go",
			vulnerable: []string{`os.WriteFile(dnsServersFile, []byte(strings.Join(normalized, "\n")+"\n"), 0o644)`},
			message:    "DNS server files should not be world-readable by default",
		},
		{
			name:       "DNSModeWrittenWorldReadable",
			path:       "../service/network/dns.go",
			vulnerable: []string{`os.WriteFile(dnsModeFile, []byte(mode+"\n"), 0o644)`},
			message:    "DNS mode files should not be world-readable by default",
		},
		{
			name:       "DNSPreserveManualServersWorldReadable",
			path:       "../service/network/dns.go",
			vulnerable: []string{`os.WriteFile(dnsServersFile, []byte(strings.Join(servers, "\n")+"\n"), 0o644)`},
			message:    "preserved DNS server files should not be world-readable",
		},
		{
			name:       "DNSRenderCreatesWorldSearchableDirectory",
			path:       "../service/network/dns.go",
			vulnerable: []string{`os.MkdirAll(filepath.Dir(path), 0o755)`},
			message:    "DNS render path should avoid world-searchable directories for generated network config",
		},
		{
			name:       "DNSRenderWritesWorldReadableFiles",
			path:       "../service/network/dns.go",
			vulnerable: []string{`os.WriteFile(path, []byte(builder.String()), 0o644)`},
			message:    "rendered DNS files should use explicit least-privilege modes",
		},
		{
			name:       "DNSBackupBootResolvWorldReadable",
			path:       "../service/network/dns.go",
			vulnerable: []string{`os.WriteFile(bootResolvBackup, data, 0o644)`},
			message:    "boot resolver backups should not be world-readable",
		},
		{
			name:       "DNSRemovesBootResolvDirectly",
			path:       "../service/network/dns.go",
			vulnerable: []string{`os.Remove(bootResolvFile)`},
			message:    "DNS mode changes should avoid removing boot resolver config without atomic backup verification",
		},
		{
			name:       "DNSInstallsHookWorldExecutable",
			path:       "../service/network/dns.go",
			vulnerable: []string{`os.WriteFile(udhcpcHookFile, []byte(udhcpcDNSHook), 0o755)`},
			message:    "installed DHCP hook should use controlled ownership and executable mode only when required",
		},
		{
			name:       "DNSSearchDomainsNotValidated",
			path:       "../service/network/dns.go",
			vulnerable: []string{`func normalizeSearchDomains(domains []string) []string`, `normalized = append(normalized, domain)`},
			message:    "DNS search domains should be validated before being reflected into resolver config and UI",
		},
		{
			name:       "DNSDefaultModeBasedOnBootFileExistence",
			path:       "../service/network/dns.go",
			vulnerable: []string{`if _, err := os.Stat(bootResolvFile); err == nil {`, `return dnsModeManual`},
			message:    "DNS default mode should not infer policy solely from boot file existence",
		},
	}

	runSourceSecurityContracts(t, cases, 21, "network-control")
}
