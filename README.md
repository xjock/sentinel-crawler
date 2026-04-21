# Sentinel Crawler

基于 Go 语言开发的欧空局哨兵（Sentinel）卫星数据及元数据爬虫系统。支持多数据源、多存储后端、多触发方式的高度模块化架构。

## 特性

- **多数据源适配**：内置 Copernicus Data Space 支持，可扩展 AWS Open Data、Google Earth Engine 等
- **元数据与下载解耦**：可独立执行元数据抓取、影像下载或完整流水线
- **可插拔存储**：支持 SQLite、PostgreSQL、MongoDB 等多种存储后端
- **多种触发方式**：REST API、Cron 定时调度、事件驱动（Kafka/RabbitMQ）
- **断点续传与校验**：影像下载支持断点续传、MD5/SHA 校验
- **水平扩展**：支持单节点与分布式 Worker 部署

## 快速开始

### 依赖

- Go 1.23+
- Git

### 安装

```bash
git clone https://github.com/xjock/sentinel-crawler.git
cd sentinel-crawler
go mod tidy
```

### 配置

编辑 `configs/config.yaml`：

```yaml
provider:
  name: copernicus
  credentials:
    username: "your-username"
    password: "your-password"
```

### 运行 CLI 爬虫

```bash
go run ./cmd/cli crawl --platform S2 --product-type L1C --sensing-from 2024-01-01
```

### 启动 API 服务

```bash
go run ./cmd/api
```

## 架构

详见 [docs/architecture.md](docs/architecture.md)。

## 实现计划

详见 [plans/plan.md](plans/plan.md)。

## 许可证

MIT License
