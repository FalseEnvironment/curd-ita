package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const stackedProviderConfigValue = "stacked"

func isStackedProviderConfig(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "stacked", "stack", "auto", "all":
		return true
	default:
		return false
	}
}

func isFactoryDefaultProvider(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return true
	}
	if isStackedProviderConfig(raw) {
		return true
	}

	names, declined := parseProviderConfig(raw)
	if declined {
		return false
	}
	switch len(names) {
	case 0:
		return true
	case 1:
		switch names[0] {
		case "senshi", "allanime":
			return true
		}
	}
	return false
}

func providerListsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func migrateProviderConfig(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if isFactoryDefaultProvider(raw) {
		if raw == stackedProviderConfigValue {
			return raw, false
		}
		return stackedProviderConfigValue, true
	}

	if isStackedProviderConfig(raw) {
		if raw != stackedProviderConfigValue {
			return stackedProviderConfigValue, true
		}
		return raw, false
	}

	names, declined := parseProviderConfig(raw)
	if len(names) > 1 {
		return stackedProviderConfigValue, true
	}

	canonical := canonicalProviderConfigValue(raw)
	if canonical != raw {
		return canonical, true
	}
	_ = declined
	return raw, false
}

func readStoredCurdVersion(storagePath string) string {
	path := storageVersionFilePath(storagePath)
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

func writeStoredCurdVersion(storagePath, version string) error {
	storagePath = strings.TrimSpace(storagePath)
	version = strings.TrimSpace(version)
	if storagePath == "" || version == "" {
		return nil
	}
	if err := os.MkdirAll(storagePath, 0755); err != nil {
		return err
	}
	return os.WriteFile(storageVersionFilePath(storagePath), []byte(version+"\n"), 0644)
}

// injectMissingConfigDefaults adds any defaultConfigMap keys that are absent from
// configMap. Returns the sorted list of newly injected keys.
func injectMissingConfigDefaults(configMap map[string]string) []string {
	if configMap == nil {
		return nil
	}
	defaults := defaultConfigMap()
	keys := make([]string, 0, len(defaults))
	for key := range defaults {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	added := make([]string, 0)
	for _, key := range keys {
		if _, exists := configMap[key]; exists {
			continue
		}
		configMap[key] = defaults[key]
		added = append(added, key)
	}
	return added
}

// appendConfigKeys appends only the given keys to the config file so existing
// user options and ordering are left untouched.
func appendConfigKeys(configPath string, configMap map[string]string, keys []string) error {
	if strings.TrimSpace(configPath) == "" || len(keys) == 0 {
		return nil
	}
	file, err := os.OpenFile(configPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	for _, key := range keys {
		value, ok := configMap[key]
		if !ok {
			continue
		}
		if _, err := fmt.Fprintf(file, "%s=%s\n", key, value); err != nil {
			return err
		}
	}
	return nil
}

// MigrateOnVersionUpgrade updates stored state and config when curd is upgraded.
// On version change it injects any new default config options (append-only) and
// runs provider migrations. Same-version launches do not rewrite the config file
// just to fill defaults — that avoids churning user config every start.
// Returns whether the config file was updated.
func MigrateOnVersionUpgrade(configPath string, config *CurdConfig, appVersion string) (bool, error) {
	if config == nil {
		return false, nil
	}

	appVersion = strings.TrimSpace(appVersion)
	if appVersion == "" {
		appVersion = CurdVersion()
	}

	storagePath := os.ExpandEnv(config.StoragePath)
	if storagePath == "" {
		storagePath = filepath.Join(os.ExpandEnv("$HOME"), ".local", "share", "curd")
	}

	storedVersion := readStoredCurdVersion(storagePath)
	configUpdated := false

	if storedVersion != appVersion && strings.TrimSpace(configPath) != "" {
		configMap, err := LoadConfigFromFile(configPath)
		if err != nil {
			return false, err
		}

		// Respect AddMissingOptions=false as a hard opt-out of writing new keys.
		addMissing := true
		if config != nil {
			addMissing = config.AddMissingOptions
		}
		if val, exists := configMap["AddMissingOptions"]; exists {
			if parsed, parseErr := parseConfigBool(val); parseErr == nil {
				addMissing = parsed
			}
		}

		if addMissing {
			if added := injectMissingConfigDefaults(configMap); len(added) > 0 {
				if err := appendConfigKeys(configPath, configMap, added); err != nil {
					return false, fmt.Errorf("append new config options: %w", err)
				}
				configUpdated = true
				Log(fmt.Sprintf("Injected new config options on upgrade to %s: %s", appVersion, strings.Join(added, ", ")))
			}
		}

		if nextProvider, changed := migrateProviderConfig(configMap["Provider"]); changed {
			configMap["Provider"] = nextProvider
			// Provider value already exists in the file — rewrite the full map once.
			if err := SaveConfigToFile(configPath, configMap); err != nil {
				return configUpdated, err
			}
			configUpdated = true
		}

		// Refresh in-memory config so new keys (e.g. VimKeys) apply immediately.
		next := PopulateConfig(configMap)
		normalizeTrackingConfig(&next)
		*config = next
	} else if storedVersion != appVersion {
		// No config path, still run in-memory provider migration.
		if nextProvider, changed := migrateProviderConfig(config.Provider); changed {
			config.Provider = nextProvider
			configUpdated = true
		}
	}

	if err := writeStoredCurdVersion(storagePath, appVersion); err != nil {
		return configUpdated, fmt.Errorf("write curd version file: %w", err)
	}

	return configUpdated, nil
}

func parseConfigBool(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "yes", "y", "on":
		return true, nil
	case "false", "0", "no", "n", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid bool %q", value)
	}
}

func providerConfigDisplayLabel(raw string) string {
	if isStackedProviderConfig(raw) {
		names := defaultEnabledProviderStack()
		if len(names) == 0 {
			return "Default with fallback"
		}
		return fmt.Sprintf("Default with fallback (%s)", strings.Join(names, " → "))
	}
	names, _ := parseProviderConfig(raw)
	if len(names) == 1 {
		return names[0]
	}
	if len(names) > 1 {
		return strings.Join(names, " → ")
	}
	return raw
}
