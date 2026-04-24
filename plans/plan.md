# Sentinel Crawler 实现计划 v2

## 项目概述

基于 Go 1.23 开发的欧空局哨兵卫星数据及元数据爬虫系统。采用分层架构设计，严格遵循依赖向内原则，支持多数据源、多存储后端、多触发方式的无缝切换。

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
│   ├── domain/           # 领域层：实体、值对象、领域接口
│   │   ├── product.go
│   │   ├── task.go
│   │   ├── download_state.go
│   │   ├── search.go
│   │   ├── geometry.go
│   │   ├── provider.go   # Provider 接口定义
│   │   ├── repository.go # Repository 接口定义
│   │   └── queue.go      # TaskQueue 接口定义
│   ├── usecase/          # 应用服务层：业务编排
│   │   ├── crawl.go      # CrawlUseCase 接口 + 实现
│   │   ├── download.go   # DownloadUseCase 接口 + 实现
│   │   └── task.go       # TaskUseCase 接口 + 实现
│   ├── provider/         # 数据源适配器（基础设施实现）
│   │   ├── copernicus/   # Copernicus Data Space 实现
│   │   ├── aws/          # AWS Open Data 实现
│   │   ├── registry.go   # Provider 注册表
│   │   └── factory.go    # Provider 工厂
│   ├── repository/       # 存储层实现（基础设施实现）
│   │   ├── sqlite/
│   │   │   ├── product.go
│   │   │   ├── task.go
│   │   │   └── download_state.go
│   │   ├── postgres/
│   │   │   ├── product.go
│   │   │   ├── task.go
│   │   │   └── download_state.go
│   │   └── mongodb/      # 预留
│   ├── queue/            # 任务队列实现（基础设施实现）
│   │   ├── memory/       # 内存队列实现
│   │   └── redis/        # Redis 队列实现
│   ├── api/              # HTTP API 接口适配层
│   │   ├── server.go     # HTTP Server 构造与生命周期
│   │   ├── handler/      # HTTP Handler
│   │   │   ├── task.go
│   │   │   ├── product.go
│   │   │   └── health.go
│   │   └── middleware/   # 中间件
│   │       ├── logging.go
│   │       ├── recovery.go
│   │       └── metrics.go
│   ├── scheduler/        # 定时调度器
│   │   └── scheduler.go
│   └── worker/           # 任务消费 Worker
│       └── worker.go
├── pkg/
│   ├── geojson/          # GeoJSON 几何类型
│   ├── httpclient/       # HTTP 客户端封装（含重试、限流）
│   ├── checksum/         # 校验和工具
│   └── logger/           # slog 封装
├── configs/
│   └── config.yaml       # 默认配置文件
├── migrations/           # 数据库迁移脚本
│   ├── sqlite/
│   └── postgres/
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
    Footprint     Geometry
    Size          int64
    DownloadURL   string
    Checksum      string
    ChecksumAlgo  string            // MD5, SHA256, etc.
    Metadata      map[string]string // 扩展元数据
    RawXML        string            // 原始 OpenSearch/XML 响应
    Source        string            // 数据源标识
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
    TaskStatusQueued     TaskStatus = "queued"
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
    Progress  float64
    Error     string
    Retries   int
    WorkerID  string
    StartedAt *time.Time
    EndedAt   *time.Time
    CreatedAt time.Time
    UpdatedAt time.Time
}

type TaskSpec struct {
    Provider   string
    Query      SearchQuery
    DestDir    string
    MaxRetries int
}
```

### DownloadState（断点续传状态）

```go
package domain

type DownloadState struct {
    ID               string
    TaskID           string
    ProductID        string
    DestPath         string
    TotalBytes       int64
    ReceivedBytes    int64
    ChecksumExpected string
    Status           string
    CreatedAt        time.Time
    UpdatedAt        time.Time
}
```

### SearchQuery / SearchResult

```go
package domain

type SearchQuery struct {
    Platform      string
    ProductType   string
    SensingFrom   time.Time
    SensingTo     time.Time
    IngestionFrom time.Time
    IngestionTo   time.Time
    Footprint     Geometry
    Page          int
    PageSize      int
    OrderBy       string
    OrderDir      string
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

### Provider 接口（领域层定义）

文件：`internal/domain/provider.go`

```go
package domain

import (
    "context"
    "io"
)

type Provider interface {
    Name() string
    Search(ctx context.Context, query SearchQuery) (*SearchResult, error)
    GetProduct(ctx context.Context, id string) (*Product, error)
    Fetch(ctx context.Context, product *Product) (io.ReadCloser, error)
    HealthCheck(ctx context.Context) error
}
```

### Repository 接口（领域层定义）

文件：`internal/domain/repository.go`

```go
package domain

import "context"

type ProductRepository interface {
    Save(ctx context.Context, product *Product) error
    SaveBatch(ctx context.Context, products []*Product) error
    FindByID(ctx context.Context, id string) (*Product, error)
    FindByQuery(ctx context.Context, query ProductQuery) ([]*Product, error)
    Exists(ctx context.Context, id string) (bool, error)
    Count(ctx context.Context, query ProductQuery) (int, error)
}

type TaskRepository interface {
    Create(ctx context.Context, task *Task) error
    Update(ctx context.Context, task *Task) error
    FindByID(ctx context.Context, id string) (*Task, error)
    FindByStatus(ctx context.Context, status TaskStatus, limit int) ([]*Task, error)
    List(ctx context.Context, filter TaskFilter) ([]*Task, error)
    Count(ctx context.Context, filter TaskFilter) (int, error)
}

type DownloadStateRepository interface {
    Save(ctx context.Context, state *DownloadState) error
    FindByTaskID(ctx context.Context, taskID string) (*DownloadState, error)
    Delete(ctx context.Context, id string) error
}
```

### TaskQueue 接口（领域层定义）

文件：`internal/domain/queue.go`

```go
package domain

import "context"

type TaskQueue interface {
    Enqueue(ctx context.Context, task *Task) error
    Dequeue(ctx context.Context) (*Task, error)
    Ack(ctx context.Context, taskID string) error
    Nack(ctx context.Context, taskID string, requeue bool) error
    Close() error
}
```

### UseCase 接口（应用服务层）

文件：`internal/usecase/crawl.go`

```go
package usecase

import (
    "context"
    "github.com/xjock/sentinel-crawler/internal/domain"
)

type Crawler interface {
    Crawl(ctx context.Context, spec domain.TaskSpec) error
}

type crawler struct {
    provider      domain.Provider
    productRepo   domain.ProductRepository
    taskRepo      domain.TaskRepository
    taskQueue     domain.TaskQueue
    maxConcurrency int
    pageSize       int
}

func NewCrawler(
    provider domain.Provider,
    productRepo domain.ProductRepository,
    taskRepo domain.TaskRepository,
    taskQueue domain.TaskQueue,
    maxConcurrency int,
    pageSize int,
) Crawler {
    return &crawler{
        provider:       provider,
        productRepo:    productRepo,
        taskRepo:       taskRepo,
        taskQueue:      taskQueue,
        maxConcurrency: maxConcurrency,
        pageSize:       pageSize,
    }
}

func (c *crawler) Crawl(ctx context.Context, spec domain.TaskSpec) error
```

文件：`internal/usecase/download.go`

```go
package usecase

import (
    "context"
    "github.com/xjock/sentinel-crawler/internal/domain"
)

type Downloader interface {
    Download(ctx context.Context, taskID string, product *domain.Product, destDir string) error
    DownloadBatch(ctx context.Context, taskID string, products []*domain.Product, destDir string) error
}

type downloader struct {
    provider     domain.Provider
    stateRepo    domain.DownloadStateRepository
    taskRepo     domain.TaskRepository
    workers      int
    chunkSize    int64
    verifyChecksum bool
    resume       bool
}

func NewDownloader(
    provider domain.Provider,
    stateRepo domain.DownloadStateRepository,
    taskRepo domain.TaskRepository,
    workers int,
    chunkSize int64,
    verifyChecksum bool,
    resume bool,
) Downloader {
    return &downloader{
        provider:       provider,
        stateRepo:      stateRepo,
        taskRepo:       taskRepo,
        workers:        workers,
        chunkSize:      chunkSize,
        verifyChecksum: verifyChecksum,
        resume:         resume,
    }
}

func (d *downloader) Download(ctx context.Context, taskID string, product *domain.Product, destDir string) error
func (d *downloader) DownloadBatch(ctx context.Context, taskID string, products []*domain.Product, destDir string) error
```

文件：`internal/usecase/task.go`

```go
package usecase

import (
    "context"
    "github.com/xjock/sentinel-crawler/internal/domain"
)

type TaskManager interface {
    CreateTask(ctx context.Context, spec domain.TaskSpec) (*domain.Task, error)
    GetTask(ctx context.Context, id string) (*domain.Task, error)
    ListTasks(ctx context.Context, filter domain.TaskFilter) ([]*domain.Task, error)
    CancelTask(ctx context.Context, id string) error
    RetryTask(ctx context.Context, id string) error
}

type taskManager struct {
    taskRepo  domain.TaskRepository
    taskQueue domain.TaskQueue
}

func NewTaskManager(taskRepo domain.TaskRepository, taskQueue domain.TaskQueue) TaskManager {
    return &taskManager{
        taskRepo:  taskRepo,
        taskQueue: taskQueue,
    }
}

func (tm *taskManager) CreateTask(ctx context.Context, spec domain.TaskSpec) (*domain.Task, error)
func (tm *taskManager) GetTask(ctx context.Context, id string) (*domain.Task, error)
func (tm *taskManager) ListTasks(ctx context.Context, filter domain.TaskFilter) ([]*domain.Task, error)
func (tm *taskManager) CancelTask(ctx context.Context, id string) error
func (tm *taskManager) RetryTask(ctx context.Context, id string) error
```

## 配置管理设计

文件：`internal/config/config.go`

```go
package config

import "github.com/spf13/viper"

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

type ServerConfig struct {
    Host string `mapstructure:"host"`
    Port int    `mapstructure:"port"`
}

type DatabaseConfig struct {
    Driver   string `mapstructure:"driver"`   // postgres, sqlite
    DSN      string `mapstructure:"dsn"`
    PoolSize int    `mapstructure:"pool_size"`
}

type ProvidersConfig struct {
    Active     string             `mapstructure:"active"`     // 当前激活的 Provider 名称
    Copernicus CopernicusConfig   `mapstructure:"copernicus"`
    AWS        AWSConfig          `mapstructure:"aws"`        // 预留
}

// CopernicusConfig Copernicus Data Space 专用配置
type CopernicusConfig struct {
    Enabled          bool   `mapstructure:"enabled"`
    BaseURL          string `mapstructure:"base_url"`            // OData Catalogue API
    DownloadBaseURL  string `mapstructure:"download_base_url"`   // 下载服务地址（与搜索可能不同）
    TokenURL         string `mapstructure:"token_url"`           // OAuth2 Token 端点
    ClientID         string `mapstructure:"client_id"`           // OAuth2 Client ID
    ClientSecret     string `mapstructure:"client_secret"`       // OAuth2 Client Secret
    Username         string `mapstructure:"username"`            // 用户名认证（备选）
    Password         string `mapstructure:"password"`            // 密码认证（备选）
    Timeout          int    `mapstructure:"timeout"`             // HTTP 请求超时（秒）
    RateLimit        int    `mapstructure:"rate_limit"`          // 每秒请求数限制
    MaxResultsPerQuery int  `mapstructure:"max_results_per_query"` // 单次搜索最大返回数
}

// AWSConfig AWS Open Data 预留配置
type AWSConfig struct {
    Enabled   bool   `mapstructure:"enabled"`
    Region    string `mapstructure:"region"`
    Bucket    string `mapstructure:"bucket"`
    AccessKey string `mapstructure:"access_key"`
    SecretKey string `mapstructure:"secret_key"`
}

type CrawlerConfig struct {
    PageSize       int  `mapstructure:"page_size"`
    MaxConcurrency int  `mapstructure:"max_concurrency"`
    RetryAttempts  int  `mapstructure:"retry_attempts"`
    SkipExisting   bool `mapstructure:"skip_existing"`
}

type DownloadConfig struct {
    DestDir        string `mapstructure:"dest_dir"`
    Workers        int    `mapstructure:"workers"`      // 产品级并发下载数
    ChunkSize      int64  `mapstructure:"chunk_size"`   // 分片大小（0 表示不分片）
    VerifyChecksum bool   `mapstructure:"verify_checksum"`
    Resume         bool   `mapstructure:"resume"`
}

type QueueConfig struct {
    Driver  string `mapstructure:"driver"`  // memory, redis
    Address string `mapstructure:"address"` // Redis 地址
}

type SchedulerConfig struct {
    Enabled bool     `mapstructure:"enabled"`
    Cron    string   `mapstructure:"cron"`
    Queries []string `mapstructure:"queries"`
}

type LogConfig struct {
    Level  string `mapstructure:"level"`
    Format string `mapstructure:"format"`
}

type MetricsConfig struct {
    Enabled bool   `mapstructure:"enabled"`
    Path    string `mapstructure:"path"`
    Port    int    `mapstructure:"port"` // 独立 metrics 端口，可选
}

func Load(path string) (*Config, error)
func setDefaults(v *viper.Viper)
```

## 可插拔组件注册机制

### Provider 注册表

文件：`internal/provider/registry.go`

```go
package provider

import (
    "sync"
    "github.com/xjock/sentinel-crawler/internal/domain"
)

type Factory func(cfg CopernicusConfig) (domain.Provider, error)

var (
    registry = make(map[string]Factory)
    mu       sync.RWMutex
)

func Register(name string, factory Factory) {
    mu.Lock()
    defer mu.Unlock()
    registry[name] = factory
}

func Get(name string) (Factory, bool) {
    mu.RLock()
    defer mu.RUnlock()
    f, ok := registry[name]
    return f, ok
}

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

### Repository 工厂

文件：`internal/repository/factory.go`

```go
package repository

import (
    "fmt"
    "github.com/xjock/sentinel-crawler/internal/domain"
    "github.com/xjock/sentinel-crawler/internal/repository/postgres"
    "github.com/xjock/sentinel-crawler/internal/repository/sqlite"
)

func NewProductRepository(driver, dsn string) (domain.ProductRepository, error) {
    switch driver {
    case "sqlite":
        return sqlite.NewProductRepository(dsn)
    case "postgres":
        return postgres.NewProductRepository(dsn)
    default:
        return nil, fmt.Errorf("unsupported database driver: %s", driver)
    }
}

func NewTaskRepository(driver, dsn string) (domain.TaskRepository, error) { /* ... */ }

func NewDownloadStateRepository(driver, dsn string) (domain.DownloadStateRepository, error) { /* ... */ }
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

providers:
  active: copernicus
  copernicus:
    enabled: true
    base_url: https://catalogue.dataspace.copernicus.eu/odata/v1
    download_base_url: https://download.dataspace.copernicus.eu
    token_url: https://identity.dataspace.copernicus.eu/auth/realms/CDSE/protocol/openid-connect/token
    client_id: ""
    client_secret: ""
    username: ""
    password: ""
    timeout: 30
    rate_limit: 10
    max_results_per_query: 1000
  aws:
    enabled: false
    region: eu-central-1
    bucket: ""
    access_key: ""
    secret_key: ""

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

queue:
  driver: memory
  address: ""

scheduler:
  enabled: false
  cron: "0 2 * * *"
  queries: []

log:
  level: info
  format: json

metrics:
  enabled: true
  path: /metrics
  port: 0  # 0 表示复用 server.port
```

## HTTP API 设计

### 任务管理

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/v1/tasks | 创建任务 |
| GET | /api/v1/tasks/:id | 获取任务详情 |
| GET | /api/v1/tasks | 列出任务 |
| POST | /api/v1/tasks/:id/cancel | 取消任务 |
| POST | /api/v1/tasks/:id/retry | 重试任务 |

### 产品查询

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/v1/products | 查询产品 |
| GET | /api/v1/products/:id | 获取产品详情 |

### 健康检查

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /health | 依赖健康检查 |
| GET | /ready | 就绪检查 |
| GET | /metrics | Prometheus 指标 |

## 实现优先级

### 第一阶段：核心骨架与领域层
- `internal/domain` 所有实体、接口
- `internal/config` 配置管理
- `internal/provider/provider.go` Provider 注册表 + Copernicus 实现
- `internal/repository/sqlite` SQLite 实现（Product、Task、DownloadState）
- `internal/queue/memory` 内存队列实现
- `internal/usecase` CrawlUseCase、DownloadUseCase、TaskUseCase
- `cmd/cli` 初始化命令行工具

### 第二阶段：API 与 Worker
- `cmd/api` REST API 服务 + HTTP Handler
- `internal/api/middleware` 日志、恢复、指标中间件
- `cmd/crawler` Worker 入口
- `internal/scheduler` 定时调度器

### 第三阶段：可观测性与增强
- `pkg/logger` slog 封装，全链路结构化日志
- Prometheus metrics 中间件
- 断点续传完整实现
- 集成测试

### 第四阶段：分布式支持
- `internal/repository/postgres` PostgreSQL 实现
- `internal/queue/redis` Redis 队列实现
- Docker 部署支持
- 数据库迁移工具集成
