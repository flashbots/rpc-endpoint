package server

import (
	"errors"
	"maps"
	"net/url"
	"os"

	"gopkg.in/yaml.v3"
)

var ErrCustomerNotConfigured = errors.New("customer is not configured")

type CustomersConfig struct {
	URLs map[string][]string `yaml:"urls"`
}

// ConfigurationWatcher
// all params are normilized
type ConfigurationWatcher struct {
	// CustomersConfig represents config for each custom with allowed list of configuration parameters
	ParsedCustomersConfig map[string][]URLParameters
}

func NewConfigurationWatcher(customersConfig CustomersConfig) (*ConfigurationWatcher, error) {
	parsedCustomersConfig := make(map[string][]URLParameters)
	for k, v := range customersConfig.URLs {
		var allowedConfigs []URLParameters
		for _, rawUrl := range v {
			parsedUrl, err := url.Parse(rawUrl)
			if err != nil {
				return nil, err
			}
			URLParam, err := ExtractParametersFromUrl(parsedUrl, nil)
			if err != nil {
				return nil, err
			}
			allowedConfigs = append(allowedConfigs, URLParam)
		}
		parsedCustomersConfig[k] = allowedConfigs
	}
	return &ConfigurationWatcher{ParsedCustomersConfig: parsedCustomersConfig}, nil
}

func ReadCustomerConfigFromFile(fileName string) (*ConfigurationWatcher, error) {
	if fileName == "" {
		return &ConfigurationWatcher{ParsedCustomersConfig: make(map[string][]URLParameters)}, nil
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
	return NewConfigurationWatcher(config)
}

func (watcher *ConfigurationWatcher) IsConfigurationUpdated(customer string, urlParams URLParameters) bool {
	allowedUrls, ok := watcher.ParsedCustomersConfig[customer]
	if !ok {
		return false
	}
	for _, au := range allowedUrls {
		if EquivalentURLParams(au, urlParams) {
			return false
		}
	}
	return true
}

func (watcher *ConfigurationWatcher) Customers() []string {
	customers := make([]string, 0, len(watcher.ParsedCustomersConfig))
	for k := range watcher.ParsedCustomersConfig {
		customers = append(customers, k)
	}

	return customers
}

func EquivalentURLParams(left URLParameters, right URLParameters) bool {
	leftParams := maps.Clone(left.rawNormalizedQueryParams)
	delete(leftParams, "refund")
	rightParams := maps.Clone(right.rawNormalizedQueryParams)
	delete(rightParams, "refund")

	if left.fast != right.fast {
		return false
	}
	if len(leftParams) != len(rightParams) {
		return false
	}

	for k, v := range leftParams {
		if k == "refund" {
			continue
		}
		rightV := rightParams[k]
		if len(rightV) != len(v) {
			return false
		}
		for i := range v {
			if v[i] != rightV[i] {
				return false
			}
		}
	}
	return true
}
