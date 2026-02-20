# Online Upgrade Regression Checklist

适用于线上升级 `xraytool` 后的回归核对。

## 0. 基础信息确认

- 服务状态：`systemctl status xraytool --no-pager`
- 环境文件：`/etc/default/xraytool`
- 面板地址：`http://<host>:<port>`

## 1. 升级与健康检查

- 执行升级脚本：

```bash
sudo bash deploy/online-upgrade.sh --version latest
```

- 预期结果：
  - 升级结束输出 `Upgrade completed successfully`
  - `healthz ok`
  - systemd 服务保持 `active`

## 2. 核心 API 回归（自动）

- 执行回归脚本：

```bash
python3 scripts/online_regression.py
```

- 预期结果：
  - 输出若干 `[PASS]`
  - 最终 `fail=0`

## 3. 手工业务回归（面板）

- 登录后台：管理员账号密码可正常使用
- IP 视图：
  - `IP 超卖热度` 可展示矩阵 + 表格
  - 可切换 `本机全局 / 某客户` 视图
- 订单创建：
  - 可选“具体到期时间”
  - 快捷天数按钮可用（7/15/30/90）
  - 显示“可分配 IP 数量”
- 订单编辑：
  - 可修改名称、数量、端口、到期时间
  - 支持缩短到期时间（减少）
- 导出：
  - 支持提取数量
  - 文件名符合：`客户代号-IP掩码-日期-订单名-条数.txt`
  - 导出结果顺序为乱序
- 测活：
  - 支持抽样 5%/10%/100%
  - `流式测活` 实时展示成功/失败滚动结果
- 实时监控：
  - `Socks5 客户实时状态` 可看到在线数、实时速度、周期流量

## 4. 设置与通知

- 设置页无可编辑 `Xray API 地址` 项（避免误导）
- Bark：
  - 启用方式为开关（switch）
  - 测试通知按钮可发送成功（已配置情况下）

## 5. 备份下载链路

- 创建备份 -> 下载备份文件
- 预期：下载成功，不再出现鉴权导致的下载失败

## 6. 回滚准备

- 升级脚本会保留：
  - 升级前数据库备份（默认）
  - 升级前 env/systemd/binary 快照目录
- 出现异常时，先停止服务后按快照恢复，再 `systemctl daemon-reload && systemctl restart xraytool`
