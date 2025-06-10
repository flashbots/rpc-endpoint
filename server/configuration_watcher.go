package server

import (
	"errors"
	"os"
	"slices"

	"gopkg.in/yaml.v3"
)

var ErrCustomerNotConfigured = errors.New("customer is not configured")

type CustomersConfig struct {
	URLs map[string][]string `yaml:"urls"`
}

// ConfigurationWatcher
// all params are normilized
type ConfigurationWatcher struct {
	// customersConfig represents config for each custom with allowed list of configuration parameters
	customersConfig CustomersConfig
}

func NewConfigurationWatcher(customersConfig CustomersConfig) *ConfigurationWatcher {
	return &ConfigurationWatcher{customersConfig: customersConfig}
}

func ReadCustomerConfigFromFile(fileName string) (*ConfigurationWatcher, error) {
	if fileName == "" {
		return &ConfigurationWatcher{customersConfig: CustomersConfig{URLs: make(map[string][]string)}}, nil
	}
	data, err := os.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	var config CustomersConfig

	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}
	return &ConfigurationWatcher{customersConfig: config}, nil
}

func (watcher *ConfigurationWatcher) IsConfigurationUpdated(customer string, url string) bool {
	allowedUrls, ok := watcher.customersConfig.URLs[customer]
	if !ok {
		return false
	}
	return !slices.Contains(allowedUrls, url)
}
