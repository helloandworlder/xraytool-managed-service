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

### Public 一键安装（推荐）

在服务器上直接粘贴执行：

```bash
curl -fsSL https://raw.githubusercontent.com/helloandworlder/xraytool-managed-service/refs/heads/main/deploy/public-install.sh | sudo bash
```

安装脚本会交互询问：

- 面板监听端口（回车自动随机，也可手动输入）
- 管理员账号（回车自动随机，也可手动输入）
- 管理员密码（回车自动随机，也可手动输入）
- （可选）实例 ID / 服务名（用于同机多实例）

并自动完成：

- 下载当前架构对应的 `xraytool` release 包（amd64/arm64）
- 下载 Xray-core 到私有目录（不全局安装）
- 写入 `/etc/default/xraytool` 与 systemd 服务
- 自动检测 Xray API 端口占用（默认尝试 `10085`，冲突时自动随机可用端口）
- 启动服务并做健康检查

非交互示例：

```bash
curl -fsSL https://raw.githubusercontent.com/helloandworlder/xraytool-managed-service/refs/heads/main/deploy/public-install.sh | sudo bash -s -- \
  --non-interactive \
  --instance-id hk01 \
  --xray-api-port random \
  --port 18080 \
  --admin-user admin \
  --admin-pass 'YourStrongPass123'
```

同机多实例建议：

- 用 `--instance-id <id>` 自动生成独立 systemd 服务名（`xraytool-<id>`）与默认安装目录（`/opt/xraytool-<id>`）
- 或用 `--service-name <name>` 指定自定义服务名（不带 `.service`）
- 每个实例请使用不同的 `--port`、`--xray-api-port`（建议 `random`）
- 可用 `--xray-bin-path /path/to/xray` 为实例指定独立 Xray Core 二进制来源

### 源码构建安装（开发环境）

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

### 3x-ui Terminal UI（BubbleTea）

新增命令：`cmd/threeuitui`

```bash
go build -o threeuitui ./cmd/threeuitui
./threeuitui --target "http://host:port/basepath username password [2fa]" --quick-days 30
# 非交互全量续费（所有有到期时间的账号统一续费 N 天）
./threeuitui --target "http://host:port/basepath username password [2fa]" --renew-all-days 30
# 非交互导出 VMESS 专线（中文表头 xlsx）
./threeuitui --target "http://host:port/basepath username password [2fa]" --export-vmess-xlsx --export-path ./vmess.xlsx
```

功能包含：

- 基于 `Inbound + Email` 前缀分组
- 连接后展示“最近到期分组”视图（支持按出口 IP 范围查看）
- 先选择出口 IP，再执行批量续费
- 已到期账号批量续费（自动全续）
- 即将到期账号批量续费（交互勾选，支持分页）
- 近 3 天到期账号快速批量续费（交互分页）
- 支持 `Y` 一键确认续费“今日已到期”账号（续费天数由 `--quick-days` 控制）
- 导出 VMESS XLSX（中文列名：专线备注 / 账号 / VMESS专线 / 出口IP / 开通时间 / 到期时间）

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

支持生产运维常用操作（交互菜单）：

- 一键重置管理员账号密码（可自定义或随机）
- 一键更新到最新版本（保留当前端口与安装目录）
- 指定版本更新（如 `v0.1.3`）
- 一键卸载（可选择保留或删除数据目录）
- 查看服务状态

## systemd

- 服务模板：`deploy/systemd/xraytool.service`
- 建议部署目录：`/opt/xraytool`

### 本地源码安装脚本

```bash
sudo ./deploy/install.sh --xray-bin /path/to/xray
```

可选参数：

- `--install-dir /opt/xraytool`
- `--xray-bin /path/to/xray`
- `--instance-id hk01`
- `--service-name xraytool-hk01`

### Public 安装脚本（可下载到本地执行）

```bash
curl -fsSL https://raw.githubusercontent.com/helloandworlder/xraytool-managed-service/refs/heads/main/deploy/public-install.sh -o public-install.sh
sudo bash public-install.sh --help
```

## Linux 全套测试（OrbStack / Docker）

```bash
./scripts/linux_orbstack_test.sh
```

该脚本会在 Linux 容器中执行后端测试构建、前端 pnpm 构建以及 API smoke。

## 生产 E2E（OrbStack）

```bash
./scripts/production_e2e_orbstack.sh
```

该脚本会在 OrbStack Linux 机器中执行完整安装验证：

- 构建当前代码并打包本地 release
- 调用 `deploy/public-install.sh` 完成 systemd 安装
- 验证 `systemctl` 服务状态、`/healthz`、登录 API

## Release CI/CD

GitHub Actions 已支持自动构建 Linux release 资产。

### 通过 tag 自动发版

```bash
git tag v0.2.0
git push origin v0.2.0
```

触发后会自动：

- 运行后端测试 `go test ./...`
- 构建前端 `frontend -> web/dist`
- 编译 Linux `amd64` 的 XrayTool、xtoolctl 和受管 Xray Fork
- 打包 release tar.gz
- 生成包含 Fork 的 `checksums.txt` 和 release manifest
- 发布到 GitHub Releases

### 手动发版

打开 GitHub Actions 里的 `Release CI` 工作流，填写：

- `version`：例如 `v0.2.0`
- `draft`：是否先发草稿
- `prerelease`：是否标记预发布

手动触发时，工作流会自动创建对应 tag 并发布 release。

### Release 产物

- `xraytool-linux-amd64`
- `xraytoolctl-linux-amd64`
- `xray-linux-amd64`（受管 Xray Fork）
- `xraytool-linux-amd64.tar.gz`
- `checksums.txt`

详细变更记录见：

- `CHANGELOG.md`

## 线上升级与回归

### 固定版本线上升级（保留端口/账号/Xray API 端口）

```bash
sudo bash deploy/online-upgrade.sh --version v0.2.0
```

可选参数：

- `--version v0.1.8` 指定不可变版本
- `--package-path /path/xraytool-linux-amd64.tar.gz --package-sha256 <sha256>` 使用已审查的本地制品
- `--backup-dir /opt/xraytool/upgrade-backups/canary-v0.2.0` 指定回滚快照目录
- `--skip-backup` 跳过升级前数据库备份

升级脚本默认会：

- 基于当前 `/etc/default/xraytool` 保留关键配置
- 执行升级前数据库备份
- 备份旧的 XrayTool、Fork、Xray 配置和前端静态文件
- 执行升级后健康检查
- 校验实例上行和下行限速配置均为有效的非零 bit/s 值

升级失败时使用同一快照回滚：

```bash
sudo bash deploy/rollback.sh \
  --service-name xraytool \
  --install-dir /opt/xraytool \
  --backup-dir /opt/xraytool/upgrade-backups/canary-v0.2.0
```

### 21 台服务器滚动升级

`deploy/rolling-upgrade.sh` 只接受不含密码的 `host,port,user` 清单；密码放在
外部目录中，每个文件名为主机 IP，权限必须是 `0600` 或 `0400`。它按清单顺序
一次只连接一台服务器，单台服务器内再逐个升级所有 `xraytool-*.service`。脚本
要求清单必须正好包含 21 台唯一主机，20 台或缺失主机会在 SSH 前直接拒绝。任一
服务失败会先回滚该服务，任一主机失败则停止后续主机；本地原子锁目录防止两个滚动
发布同时运行，并在 `--evidence-dir` 保存版本、制品 SHA、开始/完成时间和完整日志。

示例：

```bash
bash deploy/rolling-upgrade.sh \
  --inventory /secure/production-21.csv \
  --package /secure/xraytool-linux-amd64.tar.gz \
  --package-sha256 <sha256> \
  --version v0.3.0 \
  --ssh-key /secure/production-rollout_ed25519 \
  --evidence-dir /secure/deploy-evidence/v0.3.0
```

用户和实例上下行速度单位均为十进制 bit/s；没有配置或填写 0 时，默认都是
`30,000,000 bit/s`，不是“不限制”。

管理员登录不再作为升级脚本的自动门禁。管理员凭据属于每台旧实例自己的运行状态，需在业务侧单独核对；限速发布的最终验收以实际代理上传/下载流量测试为准。

## 说明

- Xray 采用托管模式时，`xraytool` 会在 `data/xray/config.json` 生成并维护配置。
- 在线 gRPC 更新失败时，会自动执行配置重建并重启托管 Xray 进程。
