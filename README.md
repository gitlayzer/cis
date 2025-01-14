# CIS(Container Image Sync) 容器镜像同步平台
CIS(Container Image Sync) 是一个开源的容器镜像同步平台，用于同步容器镜像到多个仓库，核心功能用于镜像仓库的迁移与同步，平台为前后端分离项目，技术栈如下

前端：React + Ant Design + TailwindCSS + Typescript + Vite + Axios + React-Router-Dom

后端：Go + Gin + Gorm + MySQL + JWT + Cobra + Cron

## 功能列表
- &#x2705; 支持任意标准 docker v2 格式的镜像仓库
- &#x2705; 支持将一个镜像从同步到多个不同的镜像仓库
- &#x2705; 支持定时同步镜像，支持手动同步镜像
- &#x2705; 支持多用户隔离工作流，支持不同用户管理不同工作流

## Feature
- [x] 支持工作流代理功能
- [x] 支持从多个仓库同步镜像到目标仓库
- [x] 支持镜像仓库管理镜像
- [x] ...

## 快速开始
### 1. 安装 CIS
> 注：项目目前需手动编译构建, Go 版本需 1.21+
```bash
# 克隆项目
git clone https://github.com/gitlayzer/cis.git

# 编译
make build

# 运行(支持两种配置数据库的方案)
# 1: 环境变量配置数据库
export DB_HOST=10.0.0.12
export DB_PORT=3306
export DB_USER=root
export DB_PASSWORD=123456
export DB_NAME=cis
# 运行二进制
./bin/cis start

# 2: 通过 flag 配置数据库并运行二进制文件
./bin/cis start --db-host=10.0.0.12 --db-port=3306 --db-user=root --db-pass=123456 --db-name=cis
```