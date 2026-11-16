package source

import (
	"net/http"
	"time"
)

// Config keeps catalogue preferences in the units shown in settings. An empty
// source skips that priority slot; a zero cache lifetime or capacity disables it.
// The fixed-size source order makes the whole value comparable.
type Config struct {
	Sources               [3]string `json:"sources"`
	TimeoutSeconds        int       `json:"timeoutSeconds"`
	RequestTimeoutSeconds int       `json:"requestTimeoutSeconds"`
	CacheHitHours         int       `json:"cacheHitHours"`
	CacheMissMinutes      int       `json:"cacheMissMinutes"`
	CacheEntries          int       `json:"cacheEntries"`
}

// DefaultConfig supplies the catalogue policy; it does not enable lookups.
func DefaultConfig() Config {
	return Config{
		Sources:               [3]string{"netease", "qq", "kugou"},
		TimeoutSeconds:        12,
		RequestTimeoutSeconds: 8,
		CacheHitHours:         6,
		CacheMissMinutes:      30,
		CacheEntries:          512,
	}
}

// Normalized bounds values read from settings and removes unknown or duplicate
// sources. Explicit empty slots and disabled caches stay disabled.
func (c Config) Normalized() Config {
	for i, id := range c.Sources {
		switch id {
		case "":
		case "netease", "qq", "kugou":
			for j := range i {
				if c.Sources[j] == id {
					c.Sources[i] = ""
					break
				}
			}
		default:
			c.Sources[i] = ""
		}
	}
	c.TimeoutSeconds = max(1, min(c.TimeoutSeconds, 60))
	c.RequestTimeoutSeconds = max(1, min(c.RequestTimeoutSeconds, 30))
	c.CacheHitHours = max(0, min(c.CacheHitHours, 72))
	c.CacheMissMinutes = max(0, min(c.CacheMissMinutes, 240))
	c.CacheEntries = max(0, min(c.CacheEntries, 4096))
	return c
}

// NewConfiguredChain builds only the enabled catalogues, in priority order.
// Their HTTP client is shared and carries no account credentials.
func NewConfiguredChain(config Config) *Chain {
	config = config.Normalized()
	c := NewChain()
	c.Timeout = time.Duration(config.TimeoutSeconds) * time.Second
	c.hitTTL = time.Duration(config.CacheHitHours) * time.Hour
	c.missTTL = time.Duration(config.CacheMissMinutes) * time.Minute
	c.maxEntries = config.CacheEntries
	count := 0
	for _, id := range config.Sources {
		if id != "" {
			count++
		}
	}
	if count == 0 {
		return c
	}
	c.Providers = make([]Provider, 0, count)
	client := &http.Client{Timeout: time.Duration(config.RequestTimeoutSeconds) * time.Second}
	for _, id := range config.Sources {
		switch id {
		case "netease":
			c.Providers = append(c.Providers, &NetEase{Client: client})
		case "qq":
			c.Providers = append(c.Providers, &QQ{Client: client})
		case "kugou":
			c.Providers = append(c.Providers, &Kugou{Client: client})
		}
	}
	return c
}
