# GitHub 仓库初始化与推送指南

## 前提条件

- 已安装 [Git](https://git-scm.com/)
- 已安装 [GitHub CLI](https://cli.github.com/)（可选，用于命令行创建仓库）
- 或已登录 GitHub 网页版

## 步骤一：初始化本地仓库

在 `/home/luca/dev/web/sentinel-crawler` 目录下执行：

```bash
cd /home/luca/dev/web/sentinel-crawler
git init
git add -A
git commit -m "feat: initial architecture design for sentinel crawler

- add system architecture documentation
- add implementation plan with domain models and interfaces
- add project README"
```

## 步骤二：在 GitHub 创建仓库

### 方式 A：使用 GitHub CLI（推荐）

```bash
# 创建公开仓库
gh repo create sentinel-crawler --public --source=. --remote=origin --push

# 或创建私有仓库
gh repo create sentinel-crawler --private --source=. --remote=origin --push
```

### 方式 B：使用 GitHub 网页

1. 打开 https://github.com/new
2. 输入 Repository name: `sentinel-crawler`
3. 选择 Public 或 Private
4. **不要**勾选 "Initialize this repository with a README"（本地已有）
5. 点击 **Create repository**
6. 复制页面上的 push 命令，通常是：

```bash
git remote add origin https://github.com/YOUR_USERNAME/sentinel-crawler.git
git branch -M main
git push -u origin main
```

## 步骤三：验证推送

```bash
git log --oneline
git remote -v
```

确认远程地址正确且 commit 已推送。

## 常见问题

### 身份认证失败

如果推送时提示认证失败，配置 GitHub Personal Access Token：

```bash
git remote set-url origin https://YOUR_TOKEN@github.com/YOUR_USERNAME/sentinel-crawler.git
```

或使用 SSH：

```bash
git remote set-url origin git@github.com:YOUR_USERNAME/sentinel-crawler.git
```

### 分支名称

如果 Git 默认分支为 `master` 而非 `main`：

```bash
git branch -M main
```
