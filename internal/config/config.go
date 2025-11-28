package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config manages application configuration
type Config struct {
	path  string
	viper *viper.Viper
}

// New creates a new Config with the default path (~/.ghcrctl/config.yaml)
// The path can be overridden by setting the GHCRCTL_CONFIG environment variable.
func New() *Config {
	// Check for environment variable override (useful for testing)
	if configPath := os.Getenv("GHCRCTL_CONFIG"); configPath != "" {
		return NewWithPath(configPath)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory if home directory cannot be determined
		homeDir = "."
	}

	configDir := filepath.Join(homeDir, ".ghcrctl")
	configPath := filepath.Join(configDir, "config.yaml")

	return NewWithPath(configPath)
}

// NewWithPath creates a new Config with a custom path
func NewWithPath(path string) *Config {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	return &Config{
		path:  path,
		viper: v,
	}
}

// Load reads the configuration file
func (c *Config) Load() error {
	// Check if config file exists
	if _, err := os.Stat(c.path); os.IsNotExist(err) {
		// Config doesn't exist, which is fine - we'll use defaults
		return nil
	}

	// Read the config file
	if err := c.viper.ReadInConfig(); err != nil {
		return err
	}

	return nil
}

// Save writes the configuration to disk
func (c *Config) Save() error {
	// Ensure the config directory exists
	configDir := filepath.Dir(c.path)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return err
	}

	// Write the config file
	return c.viper.WriteConfig()
}

// GetOwner returns the configured GHCR owner name and type (org or user)
// Priority order:
// 1. Environment variables (GHCRCTL_OWNER, GHCRCTL_OWNER_TYPE)
// 2. Config file (~/.ghcrctl/config.yaml)
func (c *Config) GetOwner() (string, string, error) {
	// Check environment variables first
	if envOwner := os.Getenv("GHCRCTL_OWNER"); envOwner != "" {
		envOwnerType := os.Getenv("GHCRCTL_OWNER_TYPE")
		if envOwnerType == "" {
			envOwnerType = "user" // default to user if not specified
		}
		return envOwner, envOwnerType, nil
	}

	// Fall back to config file
	if err := c.Load(); err != nil {
		return "", "", err
	}

	name := c.viper.GetString("owner-name")
	ownerType := c.viper.GetString("owner-type")

	return name, ownerType, nil
}

// SetOwner sets the GHCR owner name and type (org or user)
func (c *Config) SetOwner(name string, ownerType string) error {
	// Validate owner type
	if ownerType != "org" && ownerType != "user" {
		return fmt.Errorf("invalid owner type '%s': must be 'org' or 'user'", ownerType)
	}

	// Load current config first
	if err := c.Load(); err != nil {
		return err
	}

	// Set the values
	c.viper.Set("owner-name", name)
	c.viper.Set("owner-type", ownerType)

	// Save to disk
	return c.Save()
}
