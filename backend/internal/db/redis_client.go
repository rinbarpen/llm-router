package db

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

const (
	defaultRedisPort    = "6379"
	defaultRedisDBIndex = 0
	defaultRedisTimeout = time.Second
)

// RedisConfig captures connection options equivalent to python redis.from_url usage.
type RedisConfig struct {
	URL          string
	Address      string
	Username     string
	Password     string
	DB           int
	UseTLS       bool
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

func ParseRedisURL(raw string) (RedisConfig, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return RedisConfig{}, fmt.Errorf("parse redis url: %w", err)
	}
	if u.Scheme != "redis" && u.Scheme != "rediss" {
		return RedisConfig{}, fmt.Errorf("unsupported redis scheme: %s", u.Scheme)
	}

	host := strings.TrimSpace(u.Hostname())
	if host == "" {
		host = "127.0.0.1"
	}
	port := strings.TrimSpace(u.Port())
	if port == "" {
		port = defaultRedisPort
	}

	db := defaultRedisDBIndex
	if path := strings.Trim(strings.TrimSpace(u.Path), "/"); path != "" {
		if _, err := fmt.Sscanf(path, "%d", &db); err != nil {
			return RedisConfig{}, fmt.Errorf("invalid redis db in url path: %q", path)
		}
	}

	password, _ := u.User.Password()
	username := ""
	if u.User != nil {
		username = u.User.Username()
	}

	return RedisConfig{
		URL:          raw,
		Address:      net.JoinHostPort(host, port),
		Username:     username,
		Password:     password,
		DB:           db,
		UseTLS:       u.Scheme == "rediss",
		DialTimeout:  defaultRedisTimeout,
		ReadTimeout:  defaultRedisTimeout,
		WriteTimeout: defaultRedisTimeout,
	}, nil
}
