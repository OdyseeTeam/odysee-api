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
	dbName         string
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

func (c *Config) ReadS3Config(name string) (S3Config, error) {
	var s3cfg S3Config
	return s3cfg, c.V.UnmarshalKey(name, &s3cfg)
}

func (c *Config) ReadPostgresConfig(name string) PostgresConfig {
	var pcfg PostgresConfig
	c.V.UnmarshalKey(name, &pcfg)
	return pcfg
}

func (c PostgresConfig) GetFullDSN() string {
	return c.DSN
}

func (c PostgresConfig) GetDBName() string {
	return c.dbName
}

func (c PostgresConfig) MigrateOnConnect() bool {
	return c.AutoMigrations
}
