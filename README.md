# Go Proxy Parse PoC

这是一个本地代理解析 PoC，用于验证以下链路：

1. 本地代理抓取 HTTP/HTTPS 流量
2. 按规则提取字段
3. 写入 SQLite
4. 基于历史价格给出简单捡漏推荐

## 免责声明

- 该 PoC 仅用于你可合法访问和采集的数据源。
- 不包含针对任何游戏私有协议的逆向、绕过或对抗方案。

## 快速开始

### 1) 安装依赖

```powershell
go mod tidy
```

### 2) 启动代理

```powershell
go run ./cmd/proxy-poc -listen 127.0.0.1:18080 -admin-listen 127.0.0.1:18081 -rules config/rules.json -db data/proxy_poc.db
```

如果你要尝试 HTTPS 内容解析，增加：

```powershell
-mitm -export-ca certs/goproxy-ca.crt
```

然后把导出的 CA 证书导入到系统受信任根证书（仅本机测试环境）。

### 3) 启动模拟数据源（可选）

```powershell
go run ./cmd/mock-market
```

### 4) 用代理请求数据

```powershell
curl.exe -x http://127.0.0.1:18080 "http://127.0.0.1:19090/api/market/item?id=sword_1"
```

多请求几次后，再打一条低价请求：

```powershell
curl.exe -x http://127.0.0.1:18080 "http://127.0.0.1:19090/api/market/item?id=sword_1&price=60"
```

### 5) 查看结果

```powershell
curl.exe http://127.0.0.1:18081/stats
curl.exe http://127.0.0.1:18081/recommendations
```

## 规则配置

规则文件：`config/rules.json`

- `match`: 按 `method`/`host_contains`/`path_contains`/`url_regex` 过滤请求
- `extractors`: 按 `source + kind + pattern` 提取字段
  - `source`: `request_body` / `response_body`
  - `kind`: `gjson` / `regex`

`record_type = market_item` 时，会尝试把字段映射为价格快照：

- 必需字段：`item_name` + `unit_price`
- 可选字段：`item_key`（默认回退到 `item_name`），`quantity`

## 参数

- `-max-body-kb`: 单次请求/响应最大抓取体积（默认 512KB）
- `-window-size`: 基线窗口（默认 30）
- `-min-samples`: 最小样本数（默认 8）
- `-discount-threshold`: 捡漏阈值（默认 0.2，即低于基线 20%）

