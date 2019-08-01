package db

import (
	"fmt"
	"time"
)

type Config struct {
	Type                   string `json:"type" validate:"nonzero"`
	User                   string `json:"user" validate:"nonzero"`
	Password               string `json:"password"`
	Host                   string `json:"host" validate:"nonzero"`
	Port                   uint16 `json:"port" validate:"nonzero"`
	Timeout                int    `json:"timeout" validate:"min=1"`
	DatabaseName           string `json:"database_name" validate:""`
	RequireSSL             bool   `json:"require_ssl" validate:""`
	CACert                 string `json:"ca_cert" validate:""`
	SkipHostnameValidation bool   `json:"skip_hostname_validation" validate:""`
}

func (c Config) ConnectionString() (string, error) {
	if c.Timeout < 1 {
		return "", fmt.Errorf("timeout must be at least 1 second: %d", c.Timeout)
	}
	switch c.Type {
	case "postgres":
		ms := (time.Duration(c.Timeout) * time.Second).Nanoseconds() / 1000 / 1000
		return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable&connect_timeout=%d", c.User, c.Password, c.Host, c.Port, c.DatabaseName, ms), nil
	case "mysql":
		mysqlConnectionStringBuilder := &MySQLConnectionStringBuilder{
			MySQLAdapter: &MySQLAdapter{},
		}
		return mysqlConnectionStringBuilder.Build(c)
	default:
		return "", fmt.Errorf("database type '%s' is not supported", c.Type)
	}
}
