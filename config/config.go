package config

import (
	"bytes"
	"embed"
	"fmt"

	"github.com/spf13/viper"
)

//go:embed score_1.yaml
var configFS embed.FS

type Config struct {
	Name    string
	Metrics []Metric `mapstructure:"metrics"`
}

type Metric struct {
	Name      string    `mapstructure:"name"`
	Operation Operation `mapstructure:"operation"`
}

type Operation struct {
	Type       string      `mapstructure:"type"`
	Parameters []Parameter `mapstructure:"parameters"`
}

type Parameter struct {
	Source string `mapstructure:"source"`
	Param  string `mapstructure:"param,omitempty"`
}

func InitScoreConfig(fileName string) (*Config, error) {
	//viper.SetConfigName("score_1")
	//viper.SetConfigType("yaml")
	//viper.AddConfigPath(".")
	//viper.AddConfigPath("/app/config")
	//
	//if err := viper.ReadInConfig(); err != nil {
	//	return nil, fmt.Errorf("error reading config file: %v", err)
	//}

	fileData, err := configFS.ReadFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("error reading embedded config file: %v", err)
	}
	viper.SetConfigType("yaml")
	if err := viper.ReadConfig(bytes.NewReader(fileData)); err != nil {
		return nil, fmt.Errorf("error loading config: %v", err)
	}
	config := &Config{}
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshalling config: %v", err)
	}

	//fmt.Printf("Parsed Config: %+v\n", config)
	return config, nil
}
