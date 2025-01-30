package main

import (
	"github.com/spf13/viper"
)

func LoadConfig() {
	viper.SetDefault("doReload", true)

	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	viper.AddConfigPath("$HOME/.config/valfs")
	viper.ReadInConfig()
}
