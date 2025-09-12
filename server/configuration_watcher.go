package server

import (
	"errors"
	"fmt"
	"maps"
	"net/url"
	"os"

	"github.com/ethereum/go-ethereum/log"
	"gopkg.in/yaml.v3"
)

var ErrCustomerNotConfigured = errors.New("customer is not configured")

type CustomersConfig struct {
	URLs    map[string][]string `yaml:"urls"`
	Presets map[string]string   `yaml:"presets,omitempty"`
}

// ConfigurationWatcher
// all params are normilized
type ConfigurationWatcher struct {
	// CustomersConfig represents config for each custom with allowed list of configuration parameters
	ParsedCustomersConfig map[string][]URLParameters
	// ParsedPresets contains pre-parsed preset configurations for header-based override
	ParsedPresets map[string]URLParameters
}

// parseURLToParameters converts a raw URL string to URLParameters
func parseURLToParameters(rawURL string) (URLParameters, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return URLParameters{}, fmt.Errorf("failed to parse URL: %w", err)
	}

	params, err := ExtractParametersFromUrl(parsedURL, nil)
	if err != nil {
		return URLParameters{}, fmt.Errorf("failed to extract parameters: %w", err)
	}

	return params, nil
}

func NewConfigurationWatcher(customersConfig CustomersConfig) (*ConfigurationWatcher, error) {
	parsedCustomersConfig := make(map[string][]URLParameters)
	for customerID, urls := range customersConfig.URLs {
		allowedConfigs := make([]URLParameters, 0, len(urls))
		for _, rawURL := range urls {
			urlParam, err := parseURLToParameters(rawURL)
			if err != nil {
				return nil, fmt.Errorf("invalid URL for customer %s: %w", customerID, err)
			}
			allowedConfigs = append(allowedConfigs, urlParam)
		}
		parsedCustomersConfig[customerID] = allowedConfigs
	}

	// Parse presets for header-based override
	parsedPresets := make(map[string]URLParameters)
	for originID, presetURL := range customersConfig.Presets {
		params, err := parseURLToParameters(presetURL)
		if err != nil {
			// Log error but continue - graceful degradation
			log.Error("Failed to parse preset configuration", "originID", originID, "url", presetURL, "error", err)
			continue
		}
		parsedPresets[originID] = params
		log.Info("Loaded preset configuration", "originID", originID)
	}

	return &ConfigurationWatcher{
		ParsedCustomersConfig: parsedCustomersConfig,
		ParsedPresets:         parsedPresets,
	}, nil
}

func ReadCustomerConfigFromFile(fileName string) (*ConfigurationWatcher, error) {
	if fileName == "" {
		return &ConfigurationWatcher{
			ParsedCustomersConfig: make(map[string][]URLParameters),
			ParsedPresets:         make(map[string]URLParameters),
		}, nil
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
