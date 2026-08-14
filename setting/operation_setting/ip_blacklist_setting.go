package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

type IPBlacklistSetting struct {
	Enabled bool `json:"enabled"`
}

var ipBlacklistSetting = IPBlacklistSetting{Enabled: false}

func init() {
	config.GlobalConfig.Register("ip_blacklist_setting", &ipBlacklistSetting)
}

func IsIPBlacklistEnabled() bool {
	return ipBlacklistSetting.Enabled
}

func SetIPBlacklistEnabled(enabled bool) {
	ipBlacklistSetting.Enabled = enabled
}
