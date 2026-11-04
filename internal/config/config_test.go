package config

import (
	"io"
	"strings"
	"testing"
	"time"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestParse(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		env     map[string]string
		wantErr string
		check   func(*testing.T, Config)
	}{
		{name: "env only", env: map[string]string{EnvURL: "https://a.example/", EnvToken: "tok"},
			check: func(t *testing.T, c Config) {
				if c.URL != "https://a.example" || c.Token != "tok" || c.Timeout != 30*time.Second || c.ReadOnly {
					t.Fatalf("unexpected config %+v", c)
				}
			}},
		{name: "flags override env", args: []string{"--url", "https://b.example///", "--token", "flag", "--read-only", "--timeout", "5s", "--insecure-skip-verify"},
			env: map[string]string{EnvURL: "https://a.example", EnvToken: "env"},
			check: func(t *testing.T, c Config) {
				if c.URL != "https://b.example" || c.Token != "flag" || !c.ReadOnly || !c.InsecureSkipVerify || c.Timeout != 5*time.Second {
					t.Fatalf("unexpected config %+v", c)
				}
			}},
		{name: "missing url", env: map[string]string{EnvToken: "t"}, wantErr: "URL is missing"},
		{name: "missing token", env: map[string]string{EnvURL: "https://a"}, wantErr: "token is missing"},
		{name: "bad scheme", env: map[string]string{EnvURL: "ftp://a", EnvToken: "t"}, wantErr: "must start with"},
		{name: "bad timeout", args: []string{"--timeout", "0s"}, env: map[string]string{EnvURL: "https://a", EnvToken: "t"}, wantErr: "timeout"},
		{name: "extra arg", args: []string{"x"}, env: map[string]string{EnvURL: "https://a", EnvToken: "t"}, wantErr: "unexpected argument"},
		{name: "version skips validation", args: []string{"--version"},
			check: func(t *testing.T, c Config) {
				if !c.ShowVersion {
					t.Fatal("ShowVersion not set")
				}
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := Parse(tc.args, env(tc.env), io.Discard)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			tc.check(t, c)
		})
	}
}
