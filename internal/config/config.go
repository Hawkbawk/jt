// Package config loads jt's own settings. They live in a separate file from
// jira-cli's config and are read through their own viper instance so the two
// never share keys.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
)

// Config holds jt's settings.
type Config struct {
	Theme Theme `mapstructure:"theme"`

	viper *viper.Viper
	mu    *sync.Mutex
}

// Load reads the settings file, creating it with the defaults when missing.
func Load() (Config, error) {
	v, err := initializeViper()
	if err != nil {
		return Config{}, err
	}

	c, err := decode(v)
	if err != nil {
		return Config{}, err
	}
	c.mu = &sync.Mutex{}
	return c, nil
}

// decode reads the settings out of v, rejecting values that don't parse.
func decode(v *viper.Viper) (Config, error) {
	c := Config{viper: v}
	if err := v.Unmarshal(&c, viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.StringToSliceHookFunc(","),
		mapstructure.TextUnmarshallerHookFunc(),
	))); err != nil {
		return Config{}, fmt.Errorf("invalid config: %w", err)
	}
	return c, nil
}

// SaveTheme writes t to the settings file, keeping its other keys.
func (c Config) SaveTheme(t Theme) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.Set("theme", t.String())
	return c.viper.WriteConfig()
}

func initializeViper() (*viper.Viper, error) {
	v := viper.New()
	v.SetEnvPrefix("jt")
	v.AutomaticEnv()
	applyDefaults(v)

	path, err := loadConfiguration(v)
	if err != nil {
		return nil, err
	}

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) && !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("can't read the settings file %s: %w", path, withoutPath(err))
		}
		if err := writeDefaults(v, path); err != nil {
			return nil, err
		}
	}

	return v, nil
}

// writeDefaults creates the settings file from v's defaults and reads it back
// so v knows its path when saving.
func writeDefaults(v *viper.Viper, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return createError(path, withoutPath(err))
	}
	if err := v.SafeWriteConfigAs(path); err != nil {
		return createError(path, withoutPath(err))
	}
	return v.ReadInConfig()
}

// createError explains that the settings file couldn't be created and how to
// point jt somewhere writable, without repeating the path the OS error
// already carries.
func createError(path string, err error) error {
	return fmt.Errorf("can't create the settings file %s: %w\nmake that directory writable, or set JT_CONFIG_FILE to a file jt can write", path, err)
}

// withoutPath drops the path from an OS error, leaving the reason.
func withoutPath(err error) error {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err
	}
	return err
}

func applyDefaults(v *viper.Viper) {
	v.SetDefault("theme", ThemeBase.String())
}

// loadConfiguration points v at the settings file and returns where the file
// goes when it doesn't exist yet.
func loadConfiguration(v *viper.Viper) (string, error) {
	if f := v.GetString("config_file"); f != "" {
		v.SetConfigFile(f)
		return f, nil
	}

	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	v.SetConfigName("jt")
	v.SetConfigType("yaml")
	v.AddConfigPath(dir)

	return filepath.Join(dir, "jt.yaml"), nil
}
