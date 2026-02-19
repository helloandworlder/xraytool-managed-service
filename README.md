# XrayTool Managed Service

Go + SQLite + Gin + Zap + Vue3 + Pinia + Tailwind + Ant Design Vue 的 Xray 托管面板，支持：

- 客户管理 / 宿主机公网 IP 扫描
- 静态 IP 订单（自动/手动分配）
- 默认 `0.0.0.0:默认端口` 入口，订单可选端口
- Xray gRPC 动态下发（失败自动走配置文件重建 + 重启）
- 订单到期自动下线 + Bark 到期提醒
- 批量导入已有 `ip:port:user:pass` 并识别本机 IP/端口占用
- 订单详情弹窗 / 批量续期-停用-测活-导出 / 任务日志筛选
- Web 一键导出数据库备份到浏览器下载，支持备份列表与恢复

## Quick Start

1. 准备 Xray 二进制（不全局安装）：

```bash
mkdir -p data/xray
cp /path/to/xray ./data/xray/xray
chmod +x ./data/xray/xray
```

2. 构建前端：

```bash
cd frontend
pnpm install
pnpm run build
cd ..
```

3. 构建后端：

```bash
go mod tidy
go build -o xraytool ./cmd/xraytool
go build -o xraytoolctl ./cmd/xtoolctl
chmod +x ./deploy/xtool
```

4. 运行：

```bash
./xraytool
```

默认地址 `http://127.0.0.1:18080`，默认管理员 `admin / admin123456`。

## Web 备份与恢复

- 设置页 -> 数据库备份恢复
- `一键导出到本机`：直接触发浏览器下载 `.db` 文件
- `创建备份`：保存到服务器 `XTOOL_BACKUP_DIR`
- `恢复`：从服务器备份恢复数据库（恢复后服务自动重启）

默认备份目录可通过环境变量配置：

```bash
XTOOL_BACKUP_DIR=./data/backups
```

## xtool 脚本

```bash
./deploy/xtool
```

可交互重置管理员账号密码。

## systemd

- 服务模板：`deploy/systemd/xraytool.service`
- 建议部署目录：`/opt/xraytool`

### 一键安装脚本

```bash
sudo ./deploy/install.sh --xray-bin /path/to/xray
```

可选参数：

- `--install-dir /opt/xraytool`
- `--xray-bin /path/to/xray`

## Linux 全套测试（OrbStack / Docker）

```bash
./scripts/linux_orbstack_test.sh
```

该脚本会在 Linux 容器中执行后端测试构建、前端 pnpm 构建以及 API smoke。

## 说明

- Xray 采用托管模式时，`xraytool` 会在 `data/xray/config.json` 生成并维护配置。
- 在线 gRPC 更新失败时，会自动执行配置重建并重启托管 Xray 进程。
