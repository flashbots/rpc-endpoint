package server

import (
	"errors"
)

var (
	ErrCustomerNotConfigured = errors.New("customer is not configured")
)

// ConfigurationWatcher
// all params are normilized
type ConfigurationWatcher struct {
	// customersConfig represents config for each custom with allowed list of configuration parameters
	customersConfig map[string][]string
	// customersByOrigin mappings for customerOrigin:customerName
	customersByOrigin map[string]string
}

func NewConfigurationWatcher(customersConfig map[string][]string) *ConfigurationWatcher {
	return &ConfigurationWatcher{customersConfig: customersConfig}
}

func (watcher *ConfigurationWatcher) GetCustomerByOrigin(origin string) (string, error) {
	name, ok := watcher.customersByOrigin[origin]
	if !ok {
		return "", ErrCustomerNotConfigured
	}

	return name, nil
}

func (watcher *ConfigurationWatcher) IsConfigurationUpdated(customer string, queryParams map[string][]string) (bool, error) {
	customerParams, ok := watcher.customersConfig[customer]
	if !ok {
		return false, ErrCustomerNotConfigured
	}

	var isUpdated bool
	for k := range queryParams {
		var contains bool
		for _, p := range customerParams {
			if p == k {
				contains = true
				break
			}
		}
		if contains {
			continue
		}
		isUpdated = true
		break
	}

	return isUpdated, nil
}
