# Sentinel Crawler 实现计划

## 项目概述

基于 Go 1.23 开发的欧空局哨兵卫星数据及元数据爬虫系统。采用高度模块化、面向接口的设计，支持多数据源、多存储后端、多触发方式的无缝切换。

## 目录结构

```
sentinel-crawler/
├── cmd/
│   ├── api/              # REST API 服务入口
│   │   └── main.go
│   ├── crawler/          # 爬虫 Worker 入口
│   │   └── main.go
│   └── cli/              # 命令行工具入口
│       └── main.go
├── internal/
│   ├── config/           # 配置管理
│   │   ├── config.go
│   │   └── config_test.go
│   ├── domain/           # 领域模型（实体、值对象）
│   │   ├── product.go
│   │   ├── task.go
│   │   └── search.go
│   ├── engine/           # 核心引擎
│   │   ├── crawler.go
│   │   ├── downloader.go
│   │   └── task_manager.go
│   ├── provider/         # 数据源适配器
│   │   ├── provider.go   # Provider 接口
│   │   ├── copernicus/   # Copernicus Data Space
│   │   ├── aws/          # AWS Open Data
│   │   └── registry.go   # Provider 注册表
│   ├── repository/       # 存储层
│   │   ├── metadata.go   # MetadataRepository 接口
│   │   ├── task.go       # TaskRepository 接口
│   │   ├── postgres/
│   │   ├── mongodb/
│   │   └── sqlite/
│   ├── api/              # HTTP API 层
│   │   ├── server.go
│   │   ├── handler/
│   │   └── middleware/
│   ├── scheduler/        # 定时调度器
│   │   └── scheduler.go
│   └── worker/           # 任务消费 Worker
│       └── worker.go
├── pkg/
│   ├── geojson/          # GeoJSON 几何类型
│   ├── httpclient/       # HTTP 客户端封装
│   └── checksum/         # 校验和工具
├── configs/
│   └── config.yaml       # 默认配置文件
├── migrations/           # 数据库迁移脚本
├── docker/
│   └── Dockerfile
├── docs/
│   └── architecture.md   # 架构设计文档
├── scripts/
│   └── migrate.sh
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

## 核心领域模型

### Product（卫星产品）

```go
package domain

import "time"

type Product struct {
    ID            string
    Name          string
    Platform      string            // S1A, S1B, S2A, S2B, S3A, S3B, S5P
    ProductType   string            // GRD, SLC, OCN, L1C, L2A, etc.
    SensingDate   time.Time
    IngestionDate time.Time
    Footprint     Geometry          // WKT or GeoJSON Geometry
    Size          int64
    DownloadURL   string
    Checksum      string
    ChecksumAlgo  string            // MD5, SHA256, etc.
    Metadata      map[string]any    // 扩展元数据
    RawXML        string            // 原始 OpenSearch/XML 响应
    Source        string            // 数据源标识，如 "copernicus", "aws"
    CreatedAt     time.Time
    UpdatedAt     time.Time
}

type Geometry struct {
    Type        string
    Coordinates []any
}
```

### Task（抓取任务）

```go
package domain

import "time"

type TaskType string

const (
    TaskTypeMetadataCrawl TaskType = "metadata_crawl"
    TaskTypeDownload      TaskType = "download"
    TaskTypeFullPipeline  TaskType = "full_pipeline"
)

type TaskStatus string

const (
    TaskStatusPending    TaskStatus = "pending"
    TaskStatusRunning    TaskStatus = "running"
    TaskStatusCompleted  TaskStatus = "completed"
    TaskStatusFailed     TaskStatus = "failed"
    TaskStatusCancelled  TaskStatus = "cancelled"
)

type Task struct {
    ID        string
    Type      TaskType
    Status    TaskStatus
    Spec      TaskSpec
    Progress  float64           // 0.0 ~ 1.0
    Error     string            // 失败时的错误信息
    Retries   int
    StartedAt *time.Time
    EndedAt   *time.Time
    CreatedAt time.Time
    UpdatedAt time.Time
}

type TaskSpec struct {
    Provider   string            // 数据源名称
    Query      SearchQuery       // 搜索条件
    DestDir    string            // 下载目标目录（下载任务用）
    MaxRetries int
}
```

### SearchQuery / SearchResult

```go
package domain

type SearchQuery struct {
    Platform     string    // 卫星平台筛选
    ProductType  string    // 产品类型筛选
    SensingFrom  time.Time // 观测时间起
    SensingTo    time.Time // 观测时间止
    IngestionFrom time.Time
    IngestionTo   time.Time
    Footprint    Geometry  // 空间范围（WKT）
    Page         int
    PageSize     int
    OrderBy      string
    OrderDir     string    // asc / desc
}

type SearchResult struct {
    Products   []*Product
    TotalCount int
    Page       int
    PageSize   int
    HasMore    bool
}
```

## 核心接口设计

### Provider 接口（数据源抽象）

文件：`internal/provider/provider.go`

```go
package provider

import (
    "context"
    "github.com/lucavallin/sentinel-crawler/internal/domain"
)

// Provider 定义统一的数据源接口
type Provider interface {
    // Name 返回 Provider 唯一标识名
    Name() string

    // Search 按条件搜索元数据，返回分页结果
    Search(ctx context.Context, query domain.SearchQuery) (*domain.SearchResult, error)

    // GetProduct 获取单个产品的完整元数据
    GetProduct(ctx context.Context, productID string) (*domain.Product, error)

    // Download 下载指定产品到本地目录
    Download(ctx context.Context, product *domain.Product, destDir string) error

    // Authenticate 使用凭证进行认证
    Authenticate(ctx context.Context, creds Credentials) error

    // HealthCheck 检查数据源可用性
    HealthCheck(ctx context.Context) error
}

// Credentials 认证凭证
type Credentials struct {
    Username string
    Password string
    APIKey   string
    Token    string
}

// Factory Provider 工厂函数类型
type Factory func(cfg map[string]any) (Provider, error)
```

### Repository 接口（存储抽象）

文件：`internal/repository/metadata.go`

```go
package repository

import (
    "context"
    "github.com/lucavallin/sentinel-crawler/internal/domain"
)

// MetadataRepository 元数据持久化接口
type MetadataRepository interface {
    Save(ctx context.Context, product *domain.Product) error
    SaveBatch(ctx context.Context, products []*domain.Product) error
    FindByID(ctx context.Context, id string) (*domain.Product, error)
    FindByQuery(ctx context.Context, query domain.ProductQuery) ([]*domain.Product, error)
    Exists(ctx context.Context, id string) (bool, error)
    Count(ctx context.Context, query domain.ProductQuery) (int, error)
}
```

文件：`internal/repository/task.go`

```go
package repository

import (
    "context"
    "github.com/lucavallin/sentinel-crawler/internal/domain"
)

// TaskRepository 任务状态持久化接口
type TaskRepository interface {
    Create(ctx context.Context, task *domain.Task) error
    Update(ctx context.Context, task *domain.Task) error
    FindByID(ctx context.Context, id string) (*domain.Task, error)
    FindByStatus(ctx context.Context, status domain.TaskStatus, limit int) ([]*domain.Task, error)
    List(ctx context.Context, filter domain.TaskFilter) ([]*domain.Task, error)
    Count(ctx context.Context, filter domain.TaskFilter) (int, error)
}
```

### Engine 接口（核心引擎）

文件：`internal/engine/crawler.go`

```go
package engine

import (
    "context"
    "github.com/lucavallin/sentinel-crawler/internal/domain"
    "github.com/lucavallin/sentinel-crawler/internal/provider"
    "github.com/lucavallin/sentinel-crawler/internal/repository"
)

// Crawler 元数据爬取引擎
type Crawler struct {
    provider   provider.Provider
    metadataRepo repository.MetadataRepository
    taskRepo   repository.TaskRepository
}

func NewCrawler(p provider.Provider, metaRepo repository.MetadataRepository, taskRepo repository.TaskRepository) *Crawler {
    return &Crawler{
        provider:     p,
        metadataRepo: metaRepo,
        taskRepo:     taskRepo,
    }
}

// Crawl 执行一次完整的元数据抓取任务
func (c *Crawler) Crawl(ctx context.Context, spec domain.TaskSpec) error

// crawlPage 抓取单页数据并持久化
func (c *Crawler) crawlPage(ctx context.Context, query domain.SearchQuery) (int, error)
```

文件：`internal/engine/downloader.go`

```go
package engine

import (
    "context"
    "github.com/lucavallin/sentinel-crawler/internal/domain"
    "github.com/lucavallin/sentinel-crawler/internal/provider"
)

// Downloader 影像下载引擎
type Downloader struct {
    provider provider.Provider
    workers  int
}

func NewDownloader(p provider.Provider, workers int) *Downloader {
    return &Downloader{provider: p, workers: workers}
}

// Download 下载指定产品，支持断点续传和校验
func (d *Downloader) Download(ctx context.Context, product *domain.Product, destDir string) error

// DownloadBatch 批量下载
func (d *Downloader) DownloadBatch(ctx context.Context, products []*domain.Product, destDir string) error
```

文件：`internal/engine/task_manager.go`

```go
package engine

import (
    "context"
    "github.com/lucavallin/sentinel-crawler/internal/domain"
    "github.com/lucavallin/sentinel-crawler/internal/repository"
)

// TaskManager 任务生命周期管理
type TaskManager struct {
    taskRepo repository.TaskRepository
}

func NewTaskManager(repo repository.TaskRepository) *TaskManager {
    return &TaskManager{taskRepo: repo}
}

func (tm *TaskManager) CreateTask(ctx context.Context, spec domain.TaskSpec) (*domain.Task, error)
func (tm *TaskManager) GetTask(ctx context.Context, id string) (*domain.Task, error)
func (tm *TaskManager) ListTasks(ctx context.Context, filter domain.TaskFilter) ([]*domain.Task, error)
func (tm *TaskManager) UpdateProgress(ctx context.Context, id string, progress float64) error
func (tm *TaskManager) MarkCompleted(ctx context.Context, id string) error
func (tm *TaskManager) MarkFailed(ctx context.Context, id string, errMsg string) error
func (tm *TaskManager) MarkCancelled(ctx context.Context, id string) error
```

## 配置管理设计

文件：`internal/config/config.go`

```go
package config

import (
    "github.com/spf13/viper"
)

type Config struct {
    Server   ServerConfig   `mapstructure:"server"`
    Database DatabaseConfig `mapstructure:"database"`
    Provider ProviderConfig `mapstructure:"provider"`
    Crawler  CrawlerConfig  `mapstructure:"crawler"`
    Download DownloadConfig `mapstructure:"download"`
    Scheduler SchedulerConfig `mapstructure:"scheduler"`
    Log      LogConfig      `mapstructure:"log"`
}

type ServerConfig struct {
    Host string `mapstructure:"host"`
    Port int    `mapstructure:"port"`
}

type DatabaseConfig struct {
    Driver   string `mapstructure:"driver"`   // postgres, mongodb, sqlite
    DSN      string `mapstructure:"dsn"`
    PoolSize int    `mapstructure:"pool_size"`
}

type ProviderConfig struct {
    Name        string         `mapstructure:"name"`        // copernicus, aws
    Credentials map[string]any `mapstructure:"credentials"`
    BaseURL     string         `mapstructure:"base_url"`
    Timeout     int            `mapstructure:"timeout"`
}

type CrawlerConfig struct {
    PageSize       int  `mapstructure:"page_size"`
    MaxConcurrency int  `mapstructure:"max_concurrency"`
    RetryAttempts  int  `mapstructure:"retry_attempts"`
    SkipExisting   bool `mapstructure:"skip_existing"`
}

type DownloadConfig struct {
    DestDir        string `mapstructure:"dest_dir"`
    Workers        int    `mapstructure:"workers"`
    ChunkSize      int64  `mapstructure:"chunk_size"`       // 分片大小
    VerifyChecksum bool   `mapstructure:"verify_checksum"`
    Resume         bool   `mapstructure:"resume"`           // 断点续传
}

type SchedulerConfig struct {
    Enabled bool     `mapstructure:"enabled"`
    Cron    string   `mapstructure:"cron"`    // cron 表达式
    Queries []string `mapstructure:"queries"` // 预定义查询名列表
}

type LogConfig struct {
    Level  string `mapstructure:"level"`
    Format string `mapstructure:"format"` // json, text
}

// Load 从文件和环境变量加载配置
func Load(path string) (*Config, error) {
    v := viper.New()
    v.SetConfigFile(path)
    v.SetEnvPrefix("SENTINEL_CRAWLER")
    v.AutomaticEnv()

    setDefaults(v)

    if err := v.ReadInConfig(); err != nil {
        return nil, err
    }

    var cfg Config
    if err := v.Unmarshal(&cfg); err != nil {
        return nil, err
    }
    return &cfg, nil
}

func setDefaults(v *viper.Viper) {
    v.SetDefault("server.host", "0.0.0.0")
    v.SetDefault("server.port", 8080)
    v.SetDefault("database.driver", "sqlite")
    v.SetDefault("database.dsn", "sentinel.db")
    v.SetDefault("crawler.page_size", 100)
    v.SetDefault("crawler.max_concurrency", 5)
    v.SetDefault("crawler.retry_attempts", 3)
    v.SetDefault("download.workers", 4)
    v.SetDefault("download.chunk_size", 10485760) // 10MB
    v.SetDefault("download.verify_checksum", true)
    v.SetDefault("download.resume", true)
    v.SetDefault("log.level", "info")
    v.SetDefault("log.format", "json")
}
```

## 可插拔组件注册机制

文件：`internal/provider/registry.go`

```go
package provider

import "sync"

var (
    registry = make(map[string]Factory)
    mu       sync.RWMutex
)

// Register 注册 Provider 工厂
func Register(name string, factory Factory) {
    mu.Lock()
    defer mu.Unlock()
    registry[name] = factory
}

// Get 获取已注册的 Provider 工厂
func Get(name string) (Factory, bool) {
    mu.RLock()
    defer mu.RUnlock()
    f, ok := registry[name]
    return f, ok
}

// List 列出所有已注册的 Provider 名称
func List() []string {
    mu.RLock()
    defer mu.RUnlock()
    names := make([]string, 0, len(registry))
    for n := range registry {
        names = append(names, n)
    }
    return names
}
```

## 默认配置文件

文件：`configs/config.yaml`

```yaml
server:
  host: 0.0.0.0
  port: 8080

database:
  driver: sqlite
  dsn: ./sentinel.db
  pool_size: 10

provider:
  name: copernicus
  base_url: https://catalogue.dataspace.copernicus.eu/odata/v1
  timeout: 30
  credentials:
    username: ""
    password: ""

crawler:
  page_size: 100
  max_concurrency: 5
  retry_attempts: 3
  skip_existing: true

download:
  dest_dir: ./downloads
  workers: 4
  chunk_size: 10485760  # 10MB
  verify_checksum: true
  resume: true

scheduler:
  enabled: false
  cron: "0 2 * * *"  # 每天凌晨 2 点
  queries: []

log:
  level: info
  format: json
```

## 实现优先级

1. **第一阶段：核心骨架**
   - `internal/config` 配置管理
   - `internal/domain` 领域模型
   - `internal/provider` Provider 接口 + Copernicus 实现
   - `internal/repository` Repository 接口 + SQLite 实现

2. **第二阶段：引擎实现**
   - `internal/engine/crawler.go` 元数据抓取引擎
   - `internal/engine/downloader.go` 下载引擎
   - `internal/engine/task_manager.go` 任务管理

3. **第三阶段：入口程序**
   - `cmd/crawler` CLI 爬虫工具
   - `cmd/api` REST API 服务
   - `cmd/cli` 管理命令行

4. **第四阶段：增强功能**
   - `internal/scheduler` 定时调度器
   - `internal/worker` 分布式 Worker
   - PostgreSQL / MongoDB Repository 实现
   - Docker 部署支持
