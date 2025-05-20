package common

import (
	"github.com/tuan-dd/go-pkg/common/response"

	"github.com/spf13/viper"
)

func LoadConfig[T any](configName string) (*T, *response.AppError) {
	if configName == "" {
		configName = "config"
	}
	viper := viper.New()
	viper.AddConfigPath("configs/")
	viper.SetConfigName(configName)
	viper.SetConfigType("yaml")
	if err := viper.ReadInConfig(); err != nil {
		return nil, response.ServerError(err.Error())
	}

	config := new(T)
	if err := viper.Unmarshal(config); err != nil {
		return nil, response.ServerError(err.Error())
	}

	return config, nil
}
