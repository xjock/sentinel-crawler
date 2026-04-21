package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config 全局配置
type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Database  DatabaseConfig  `mapstructure:"database"`
	Providers ProvidersConfig `mapstructure:"providers"`
	Crawler   CrawlerConfig   `mapstructure:"crawler"`
	Download  DownloadConfig  `mapstructure:"download"`
	Queue     QueueConfig     `mapstructure:"queue"`
	Scheduler SchedulerConfig `mapstructure:"scheduler"`
	Log       LogConfig       `mapstructure:"log"`
	Metrics   MetricsConfig   `mapstructure:"metrics"`
}

// ServerConfig HTTP 服务配置
type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Driver   string `mapstructure:"driver"` // postgres, sqlite
	DSN      string `mapstructure:"dsn"`
	PoolSize int    `mapstructure:"pool_size"`
}

// ProvidersConfig Provider 配置集合
type ProvidersConfig struct {
	Active     string           `mapstructure:"active"`
	Copernicus CopernicusConfig `mapstructure:"copernicus"`
	AWS        AWSConfig        `mapstructure:"aws"`
}

// CopernicusConfig Copernicus Data Space 配置
type CopernicusConfig struct {
	Enabled            bool   `mapstructure:"enabled"`
	BaseURL            string `mapstructure:"base_url"`
	DownloadBaseURL    string `mapstructure:"download_base_url"`
	TokenURL           string `mapstructure:"token_url"`
	ClientID           string `mapstructure:"client_id"`
	ClientSecret       string `mapstructure:"client_secret"`
	Username           string `mapstructure:"username"`
	Password           string `mapstructure:"password"`
	Timeout            int    `mapstructure:"timeout"`
	RateLimit          int    `mapstructure:"rate_limit"`
	MaxResultsPerQuery int    `mapstructure:"max_results_per_query"`
}

// AWSConfig AWS Open Data 预留配置
type AWSConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	Region    string `mapstructure:"region"`
	Bucket    string `mapstructure:"bucket"`
	AccessKey string `mapstructure:"access_key"`
	SecretKey string `mapstructure:"secret_key"`
}

// CrawlerConfig 爬虫配置
type CrawlerConfig struct {
	PageSize       int  `mapstructure:"page_size"`
	MaxConcurrency int  `mapstructure:"max_concurrency"`
	RetryAttempts  int  `mapstructure:"retry_attempts"`
	SkipExisting   bool `mapstructure:"skip_existing"`
}

// DownloadConfig 下载配置
type DownloadConfig struct {
	DestDir        string `mapstructure:"dest_dir"`
	Workers        int    `mapstructure:"workers"`
	ChunkSize      int64  `mapstructure:"chunk_size"`
	VerifyChecksum bool   `mapstructure:"verify_checksum"`
	Resume         bool   `mapstructure:"resume"`
}

// QueueConfig 队列配置
type QueueConfig struct {
	Driver  string `mapstructure:"driver"` // memory, redis
	Address string `mapstructure:"address"`
}

// SchedulerConfig 定时调度配置
type SchedulerConfig struct {
	Enabled bool     `mapstructure:"enabled"`
	Cron    string   `mapstructure:"cron"`
	Queries []string `mapstructure:"queries"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// MetricsConfig 指标配置
type MetricsConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"`
	Port    int    `mapstructure:"port"`
}

// Load 从文件和环境变量加载配置
func Load(path string) (*Config, error) {
	v := viper.New()
	if path != "" {
		v.SetConfigFile(path)
	}
	v.SetEnvPrefix("SENTINEL_CRAWLER")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	setDefaults(v)

	if path != "" {
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)

	v.SetDefault("database.driver", "sqlite")
	v.SetDefault("database.dsn", "./sentinel.db")
	v.SetDefault("database.pool_size", 10)

	v.SetDefault("providers.active", "copernicus")
	v.SetDefault("providers.copernicus.enabled", true)
	v.SetDefault("providers.copernicus.base_url", "https://catalogue.dataspace.copernicus.eu/odata/v1")
	v.SetDefault("providers.copernicus.download_base_url", "https://download.dataspace.copernicus.eu")
	v.SetDefault("providers.copernicus.token_url", "https://identity.dataspace.copernicus.eu/auth/realms/CDSE/protocol/openid-connect/token")
	v.SetDefault("providers.copernicus.timeout", 30)
	v.SetDefault("providers.copernicus.rate_limit", 10)
	v.SetDefault("providers.copernicus.max_results_per_query", 1000)

	v.SetDefault("crawler.page_size", 100)
	v.SetDefault("crawler.max_concurrency", 5)
	v.SetDefault("crawler.retry_attempts", 3)
	v.SetDefault("crawler.skip_existing", true)

	v.SetDefault("download.dest_dir", "./downloads")
	v.SetDefault("download.workers", 4)
	v.SetDefault("download.chunk_size", 10485760) // 10MB
	v.SetDefault("download.verify_checksum", true)
	v.SetDefault("download.resume", true)

	v.SetDefault("queue.driver", "memory")
	v.SetDefault("queue.address", "")

	v.SetDefault("scheduler.enabled", false)
	v.SetDefault("scheduler.cron", "0 2 * * *")
	v.SetDefault("scheduler.queries", []string{})

	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")

	v.SetDefault("metrics.enabled", true)
	v.SetDefault("metrics.path", "/metrics")
	v.SetDefault("metrics.port", 0)
}
