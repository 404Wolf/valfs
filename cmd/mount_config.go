package cmd

import (
	common "github.com/404wolf/valfs/common"
	"github.com/spf13/viper"
)

func LoadConfig() *common.ValfsConfig {
	// Set default values
	viper.SetDefault("doReload", true)
	viper.SetDefault("root", "")
	viper.SetDefault("denoCache", true)
	viper.SetDefault("denoJson", true)
	viper.SetDefault("autoRefresh", true)
	viper.SetDefault("autoUnmountOnExit", true)
	viper.SetDefault("autoRefreshInterval", 300) // 5 minutes default
	viper.SetDefault("enableValsDirectory", true)
	viper.SetDefault("goFuseDebug", false)

	// Load config from environment and file
	viper.AutomaticEnv()
	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	viper.AddConfigPath("$HOME/.config/valfs")
	viper.ReadInConfig()

	// Map to config struct
	config := &common.ValfsConfig{
		APIKey:               viper.GetString("apiKey"),
		MountPoint:           viper.GetString("root"),
		DenoCache:            viper.GetBool("denoCache"),
		DenoJson:             viper.GetBool("denoJson"),
		AutoRefresh:          viper.GetBool("autoRefresh"),
		AutoUnmountOnExit:    viper.GetBool("autoUnmountOnExit"),
		AutoRefreshInterval:  viper.GetInt("autoRefreshInterval"),
		EnableValsDirectory:  viper.GetBool("enableValsDirectory"),
		EnableBlobsDirectory: viper.GetBool("enableBlobsDirectory"),
		GoFuseDebug:          viper.GetBool("goFuseDebug"),
	}

	return config
}
