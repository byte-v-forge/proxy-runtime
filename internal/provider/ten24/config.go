package ten24

import (
	"errors"
	"fmt"
)

type Config struct {
	APIURL        string
	APIRegion     string
	APIFormat     string
	APITime       string
	APINum        string
	APIType       string
	ProxyAddr     string
	Username      string
	Password      string
	Protocol      string
	Region        string
	State         string
	City          string
	ASN           string
	SessionID     string
	StickyMinutes int
}

func (c Config) Validate() error {
	if c.APIURL != "" {
		return nil
	}
	if c.ProxyAddr == "" {
		return errors.New("PROXY_RUNTIME_1024_PROXY_ADDR or PROXY_RUNTIME_1024_API_URL is required")
	}
	if c.Username == "" {
		return errors.New("PROXY_RUNTIME_1024_USERNAME is required")
	}
	if c.Password == "" {
		return errors.New("PROXY_RUNTIME_1024_PASSWORD is required")
	}
	if c.ASN != "" && (c.State != "" || c.City != "") {
		return errors.New("1024proxy ASN cannot be combined with state or city")
	}
	if c.StickyMinutes != 0 {
		if c.StickyMinutes < 1 || c.StickyMinutes > 120 {
			return errors.New("PROXY_RUNTIME_1024_STICKY_MINUTES must be between 1 and 120")
		}
	}
	switch c.Protocol {
	case "", "http", "socks5":
		return nil
	default:
		return fmt.Errorf("unsupported 1024proxy protocol %q", c.Protocol)
	}
}
