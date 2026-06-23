# CLAUDE.md

这份文档面向在本仓库中工作的 Claude Code。它说明项目要做什么、当前到了哪一步、代码怎么组织，以及在改动时应当遵守哪些约定。

---

## 项目目标

用 Go 实现一个**教学级加密货币交易所撮合引擎**，最终目标包含：

- 完整的**订单簿**：价格-时间优先，支持挂单/撤单/快速查最优价
- 多种**订单类型**的**撮合引擎**：Limit / Market（已具备），后续 Stop / FOK / GTD
- **WAL（Write-Ahead Log）**：每一条改变订单簿状态的命令持久化落盘，保证宕机不丢单
- **快照（Snapshot）**：周期性把订单簿全量状态序列化到磁盘，配合 WAL 实现快速恢复
- **主备同步**：主节点把命令流复制给备节点，备节点回放达成状态一致，主挂掉可切换
- 基础性能测试与架构文档

当前阶段：订单簿 + 限价/市价撮合 + HTTP API 已就绪。WAL / 快照 / 主备同步**尚未开始**。开发节奏参见 [docs/learning-plan.md](docs/learning-plan.md)。

---

## 代码组织

项目采用 Go 社区标准布局，依赖单向：`cmd → api → exchange → orderbook`，无反向依赖。

```
crypto-exchange/
├─ cmd/server/main.go         # 进程入口，只做装配（构造对象 / 注册路由 / 启动）
├─ internal/
│  ├─ orderbook/              # 领域层（纯逻辑，仅依赖标准库）
│  │  ├─ order.go             #   Order + 时间排序 + 私有 ID 生成
│  │  ├─ limit.go             #   Limit 价位 + 最优价排序 + Fill 撮合
│  │  ├─ match.go             #   Match 成交结果
│  │  └─ orderbook.go         #   Orderbook 主体 + Snapshot 只读视图
│  ├─ exchange/exchange.go    # 应用层：多市场聚合（Market → Orderbook）
│  └─ api/                    # 适配层：HTTP 协议形状
│     ├─ dto.go               #   PlaceOrderRequest / OrderType 枚举
│     ├─ handler.go           #   三个 echo handler
│     └─ router.go            #   Register(e, h)，路由集中注册
├─ docs/learning-plan.md      # 7 晚学习计划，记录每一步的目标和取舍
├─ Makefile                   # build / run / test
└─ go.mod
```

### 各层职责（不要越界）

- **orderbook**：纯领域逻辑，零外部依赖。任何 HTTP、序列化、日志框架的代码都不应进入此包。
- **exchange**：把多个市场组合起来。运行期不增删市场，map 本身不加锁，并发安全由各 Orderbook 自己负责。
- **api**：HTTP 协议形状。`OrderType` / `PlaceOrderRequest` 这些枚举与 DTO 属于协议层概念，**不应**下沉到领域层。
- **cmd/server**：只做装配。任何业务规则、撮合逻辑都不应出现在 `main.go`。

### 关键约定

- **并发**：`Orderbook` 全部读写用 `sync.Mutex` 互斥，**不是** `RWMutex`。原因：`Asks/Bids` 的读路径会调用 `sort.Sort` 修改底层切片顺序，并非纯读。
- **方法命名**：`Orderbook` 上大写方法（如 `PlaceLimitOrder`、`Asks`）对外暴露并自带加锁；小写方法（如 `sortedAsks`、`bidToVolume`、`clearLimit`）是已持锁的内部版本，仅供同包内已加锁的方法复用，**调用时不要重复上锁**。
- **快照读**：HTTP 层读订单簿请用 `Orderbook.Snapshot()`，它返回与内部存储解耦的只读视图（`Snapshot{Bids, Asks}`），可安全长期持有或异步序列化。不要为了"性能"绕过它直接读内部 slice。
- **订单 ID**：`NewOrder` 内部用包私有的 `nextOrderID()`（atomic 计数器）生成。外部不要再造一份 ID 生成器。
- **价格类型**：当前用 `float64`，已知有精度坑（详见 learning-plan §10.2）。后续切到定点数（如 `decimal.Decimal` 或 `int64` cents）时是跨包改动，需要同步改 DTO 和测试。

---

## 路线图（待办的大模块）

按 learning-plan 的节奏，下一步要逐步加入：

1. **WAL（Write-Ahead Log）**
   - 在每次 `PlaceLimitOrder` / `PlaceMarketOrder` / `CancelOrder` **生效前**把命令写入只追加日志文件，`fsync` 后再修改内存状态。
   - 命令格式建议自描述（带类型标签 + 长度前缀），便于回放时一条条读出。
   - 建议新增 `internal/wal/` 包；orderbook 层通过接口接收 WAL 写入器，保持领域层不感知文件系统。

2. **快照（Snapshot）**
   - 周期性（或按 WAL 增长量触发）把整本订单簿序列化到磁盘，记录截止到哪条 WAL 序号。
   - 启动恢复：加载最近一份快照 → 重放快照之后的 WAL 记录。
   - 复用现有的 `Snapshot()` 只读视图作为序列化起点，但要扩展成"含 ID/Limit 反向指针"的完整状态，能完全还原。

3. **主备同步**
   - 主节点把 WAL 命令流通过网络推给备节点，备节点按相同顺序回放达成状态一致。
   - 先做单主单备的同步复制（主等备 ack 才回客户端），再考虑异步与切主。
   - 协议层与 WAL 格式应当尽量复用，避免两套序列化。

引入新模块时**保持依赖方向单向**：领域层不直接 import WAL/网络包，而是通过接口注入。

---

## 构建与验证

```bash
make build          # = go build -o bin/exchange ./cmd/server
make run            # 启动 HTTP 服务，监听 :5221
make test           # = go test -v ./...
go vet ./...        # 静态检查
```

提交前请确保 `go test ./...` 与 `go vet ./...` 均通过。

---

## HTTP API（当前版本）

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET`    | `/orderbook?market=ETH` | 返回当前订单簿快照（bids/asks） |
| `POST`   | `/order` | 下单，body 见 `api.PlaceOrderRequest` |
| `DELETE` | `/order/:market/:id` | 按 ID 撤单 |

`PlaceOrderRequest` 形状：
```json
{ "Type": "LIMIT" | "MARKET", "Bid": true, "Size": 5.0, "Price": 100.0, "Market": "ETH" }
```

---

## 工作建议

- **小步走**：每个新模块（WAL / 快照 / 主备）单独一个包，先把接口和最小实现拉通，再补健壮性。
- **不要在 `main.go` 写逻辑**：任何超出"构造对象 + 注册路由 + 启动"的代码都说明分层错了。
- **改动领域层时同步加/改测试**：`orderbook` 包的测试是回归网，撮合相关改动必须有用例覆盖。
- **改动 HTTP 形状要同时改 `api/handler_test.go`**：它是真实启动路径的集成测试，能在第一时间发现接口回归。
- **遇到学习节点的取舍**：先翻 `docs/learning-plan.md` 里的当晚条目，里面记录了为什么选 A 不选 B。
