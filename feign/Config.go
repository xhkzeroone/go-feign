package feign

import (
	"github.com/spf13/viper"
	"strings"
	"time"
)

type Config struct {
	Url        string            `mapstructure:"url" yaml:"url"`
	Timeout    time.Duration     `mapstructure:"timeout" yaml:"timeout"`
	RetryCount int               `mapstructure:"retry_count" yaml:"retry_count"`
	RetryWait  time.Duration     `mapstructure:"retry_wait" yaml:"retry_wait"`
	Headers    map[string]string `mapstructure:"headers" yaml:"headers"`
	Debug      bool              `mapstructure:"debug" yaml:"debug"`
}

func DefaultConfig() *Config {
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	viper.SetDefault("feign.timeout", "30s")
	viper.SetDefault("feign.retry_count", "0")
	viper.SetDefault("feign.retry_wait", "1s")
	viper.SetDefault("feign.debug", false)
	return &Config{
		Timeout:    viper.GetDuration("feign.timeout"),
		RetryCount: viper.GetInt("feign.retry_count"),
		RetryWait:  viper.GetDuration("feign.retry_wait"),
		Debug:      viper.GetBool("feign.debug"),
	}
}
