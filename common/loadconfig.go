package common

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
	"github.com/tuan-dd/go-pkg/common/response"
)

func LoadConfig[T any](configName string) (*T, *response.AppError) {
	_ = godotenv.Load()
	if configName == "" {
		configName = os.Getenv("CONFIG_NAME")
		if configName == "" {
			configName = "config"
		}
	}

	cfgLoader := viper.New()
	cfgLoader.AddConfigPath("configs/")
	cfgLoader.SetConfigName(configName)
	cfgLoader.SetConfigType("yaml")
	if err := cfgLoader.ReadInConfig(); err != nil {
		return nil, response.ServerError(fmt.Sprintf("failed to read config %q: %v", configName, err))
	}

	config := new(T)
	if err := cfgLoader.Unmarshal(config); err != nil {
		return nil, response.ServerError(fmt.Sprintf("failed to read config %q: %v", configName, err))
	}

	return config, nil
}
