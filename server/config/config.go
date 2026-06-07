package config

import (
	"bytes"
	"errors"
	"log"
	"sync"

	"NanoKVM-Server/utils"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var (
	instance Config
	once     sync.Once
)

func GetInstance() *Config {
	once.Do(initialize)

	return &instance
}

func initialize() {
	if err := readByFile(); err != nil {
		if errors.As(err, &viper.ConfigFileNotFoundError{}) {
			create()
		}

		if err = readByDefault(); err != nil {
			log.Fatalf("Failed to read default configuration!")
		}

		log.Println("using default configuration")
	}

	if err := validate(); err != nil {
		log.Fatalf("Failed to validate configuration!")
	}

	if err := viper.Unmarshal(&instance); err != nil {
		log.Fatalf("Failed to parse configuration: %s", err)
	}

	checkDefaultValue()

	if instance.Authentication == "disable" {
		log.Println("NOTICE: Authentication is disabled! Please ensure your service is secure!")
	}

	log.Println("config loaded successfully")
}

func readByFile() error {
	viper.SetConfigName("server")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("/etc/kvm/")

	return viper.ReadInConfig()
}

func readByDefault() error {
	data, err := yaml.Marshal(defaultConfig)
	if err != nil {
		log.Printf("failed to marshal default config: %s", err)
		return err
	}

	return viper.ReadConfig(bytes.NewBuffer(data))
}

// Create configuration file.
func create() {
	var (
		data []byte
		err  error
	)

	if data, err = yaml.Marshal(defaultConfig); err != nil {
		log.Printf("failed to marshal default config: %s", err)
		return
	}

	if err = utils.WritePrivateFile("/etc/kvm/server.yaml", data); err != nil {
		log.Printf("save config failed: %s", err)
		return
	}
	log.Println("create file /etc/kvm/server.yaml with default configuration")
}

// Validate the configuration. This is to ensure compatibility with earlier versions.
func validate() error {
	if viper.GetInt("port.http") > 0 && viper.GetInt("port.https") > 0 {
		return nil
	}

	create()

	return readByDefault()
}
