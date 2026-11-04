// Package config reads the server settings from flags and environment variables.
package config

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"
)

// Environment variable names.
const (
	EnvURL   = "ONCALLALERT_URL"
	EnvToken = "ONCALLALERT_TOKEN"
)

// Config is the validated runtime configuration.
type Config struct {
	URL                string
	Token              string
	ReadOnly           bool
	InsecureSkipVerify bool
	Timeout            time.Duration
	ShowVersion        bool
}

// Parse reads args (without the program name) and the environment.
// Flags override the environment. --version skips validation of URL and token.
func Parse(args []string, getenv func(string) string, usageOut io.Writer) (Config, error) {
	var c Config
	fs := flag.NewFlagSet("oncallalert-mcp", flag.ContinueOnError)
	fs.SetOutput(usageOut)
	url := fs.String("url", "", "OnCallAlert server URL (env "+EnvURL+")")
	token := fs.String("token", "", "API token (env "+EnvToken+" is safer: flags show in the process list)")
	fs.BoolVar(&c.ReadOnly, "read-only", false, "hide all tools that change data")
	fs.BoolVar(&c.InsecureSkipVerify, "insecure-skip-verify", false, "skip TLS certificate verification")
	fs.DurationVar(&c.Timeout, "timeout", 30*time.Second, "timeout for each request to the server")
	fs.BoolVar(&c.ShowVersion, "version", false, "print the version and exit")
	if err := fs.Parse(args); err != nil {
		return c, err
	}
	if fs.NArg() > 0 {
		return c, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	if c.ShowVersion {
		return c, nil
	}

	c.URL = firstNonEmpty(*url, getenv(EnvURL))
	c.Token = firstNonEmpty(*token, getenv(EnvToken))
	return c, c.validate()
}

func (c *Config) validate() error {
	c.URL = strings.TrimRight(strings.TrimSpace(c.URL), "/")
	c.Token = strings.TrimSpace(c.Token)
	if c.URL == "" {
		return errors.New("server URL is missing: set " + EnvURL + " or pass --url")
	}
	if !strings.HasPrefix(c.URL, "https://") && !strings.HasPrefix(c.URL, "http://") {
		return fmt.Errorf("server URL %q must start with https:// or http://", c.URL)
	}
	if c.Token == "" {
		return errors.New("API token is missing: set " + EnvToken + " or pass --token")
	}
	if c.Timeout <= 0 {
		return errors.New("--timeout must be greater than zero")
	}
	return nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
