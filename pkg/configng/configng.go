package configng

import (
	"fmt"

	"github.com/spf13/viper"
)

type S3Config struct {
	Endpoint    string
	Region      string
	Bucket      string
	Key, Secret string
	Minio       bool
}

type PostgresConfig struct {
	DSN            string
	AutoMigrations bool
}

type Config struct {
	V *viper.Viper
}

func Read(path, name, format string) (*Config, error) {
	v := viper.New()
	v.SetConfigName(name)
	v.SetConfigType(format)
	v.AddConfigPath(path)
	err := v.ReadInConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}
	return &Config{V: v}, nil
}

func (c *Config) ReadS3Config(name string) S3Config {
	var s3c S3Config
	c.V.UnmarshalKey(name, &s3c)
	return s3c
}

func (c *Config) ReadPostgresConfig(name string) PostgresConfig {
	var pcfg PostgresConfig
	c.V.UnmarshalKey(name, &pcfg)
	return pcfg
}

func (c PostgresConfig) GetFullDSN() string {
	return c.DSN
}

func (c PostgresConfig) MigrateOnConnect() bool {
	return c.AutoMigrations
}
