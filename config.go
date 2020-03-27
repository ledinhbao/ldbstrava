package ldbstrava

import (
	"fmt"
	"strings"
)

// Config struct contains configuration for Strava Package
//   - SubscriptionDBKey	: strava subcsription key, stored in database, indicates exist subscription if present.
type Config struct {
	ClientID          string
	ClientSecret      string
	Scopes            []string
	PathPrefix        string
	PathRedirect      string
	PathSubcription   string
	GlobalDatabase    string
	SubscriptionDBKey string
}

// Active Config object for package
var config *Config

func (c *Config) getRedirectPath() string {
	return c.PathPrefix + c.PathRedirect
}

// GetRevokeURLFor return revoke link for username
func (c *Config) GetRevokeURLFor(username string) string {
	return c.PathPrefix + "/strava/revoke/" + username
}

// ActiveConfig return current config object.
func ActiveConfig() *Config { return config }

// GetAuthURL return authorize link to strava: ?client_id=<>&
func (c Config) GetAuthURL() string {
	res := fmt.Sprintf("https://www.strava.com/oauth/authorize?client_id=%s", c.ClientID)
	res += fmt.Sprintf("&redirect_uri=%s", c.getRedirectPath())
	res += fmt.Sprintf("&response_type=code&approval_prompt=auto&scope=%s", strings.Join(c.Scopes, ","))
	return res
}