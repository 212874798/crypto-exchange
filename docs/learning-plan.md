# 撮合引擎 7 晚学习计划

> 面向 Java 开发者的 Go 加密货币交易所项目
> 每晚 20:00-22:30（2.5h），周一至周日
> 参考文章：quant67.com 撮合引擎实现

---

## 总目标

完成一个 Go 语言实现的教学级加密货币交易所撮合引擎，包含：
- 完整订单簿（价格时间优先撮合）
- 多种订单类型（Limit / Market / Stop / FOK / GTD）
- WAL + 快照持久化
- 基础性能测试
- 架构文档

---

## Day 1（周一）：订单簿数据结构 + Go 基础

**文章：** §1.1-1.5（订单簿逻辑模型、数据结构选型、价格层内部结构、L1/L2/L3 行情）

**现有代码问题：**
- `[]*Limit` 未按价格排序 → 每次查最优价需要 O(n)
- `DeleteOrder` 用遍历查找 → 没有 HashMap 索引，撤单 O(n)
- `float64` 做价格 → 精度问题（文章 §10.2 坑点1）

**今晚要做：**
1. 把 `Orderbook.Asks/Bids` 改进为排序结构
   - 方案A：用 `[]*Limit` 但插入时保持排序（适合教学）
   - 方案B：换 `BTreeMap`（需要引入外部包，更标准但复杂）
   - 建议：方案A，手工维护排序切片，插入时二分查找位置
2. 添加 `Order.ID int64` 字段、全局自增计数器
3. 添加 `map[int64]*Order` 索引，实现 O(1) 撤单
4. 给 Orderbook 加方法：`InsertOrder`、`CancelOrder`、`BestBid`、`BestAsk`
5. 写测试：插入排序正确、撤单正确、最优价查询正确

**Java 对照重点：**
- Go slice append + copy ≈ Java ArrayList.add(index, element)
- 1.21+ 用 `slices.Insert()` 更简洁
- 指针接收者 `func (ob *Orderbook)` ≈ Java 非 static 方法

---

## Day 2（周二）：撮合算法 Price-Time Priority + bug 修复

**⏮ 昨日进度：** Day 1 超额完成！已实现排序订单簿 + HashMap 索引 + PlaceMarketOrder + Fill()。进度超前约 0.5 天。

**文章：** §2.1-2.2（价格时间优先、Pro-Rata）

**🔧 今晚第一件事（5分钟）：修复 DeleteOrder bug**

`orderbook.go` 第 76-87 行，`DeleteOrder` 方法有两个 bug：

Bug 1：swap-delete 后没有 `break`，继续遍历变短的切片有风险
Bug 2：`totalVolume -= o.Size` 在循环外，即使没找到订单也会减去

修复如下：
```go
func (l *Limit) DeleteOrder(o *Order) {
    for i := 0; i < len(l.Orders); i++ {
        if l.Orders[i] == o {
            l.Orders[i] = l.Orders[len(l.Orders)-1]
            l.Orders = l.Orders[:len(l.Orders)-1]
            l.totalVolume -= o.Size  // ← 移到 if 里面
            break                     // ← 加上 break
        }
    }
    sort.Sort(Orders(l.Orders))
}
```

**今晚要做：**
1. ✅（已完成）Match 结构体
2. ✅（已完成）PlaceMarketOrder 市价单撮合
3. ✅（已完成）价格层清空后 ClearLimit
4. 🔲 实现 PlaceLimitOrder 的撮合逻辑：限价单到达时**先尝试匹配对手方**，剩余再挂单
   - 买单价格 ≥ BestAsk → 触发撮合（用已有 Fill 逻辑）
   - 卖单价格 ≤ BestBid → 触发撮合
   - 不交叉 → 直接挂单（当前逻辑）
5. 🔲 LimitOrder 测试：价格交叉时成交 + 部分成交剩余挂单
6. 🔲 阅读 §2.2 Pro-Rata 按比例分配（了解即可，不做实现）
7. 价格/数量改为 `int64`（定点整数，price * 100 即精确到分）—— 暂缓，Day 5 重构时统一改

---

## Day 3（周三）：订单类型 + 确定性原则

**文章：** §3、§4

**今晚要做：**
1. 定义 `OrderType` 枚举（Limit / Market / Stop / FOK / GTD）
2. 实现 Market 单：一直扫到成交完或簿空
3. 实现 FOK 单：二阶段——先模拟全量成交，够了就真成交，不够全撤
4. 实现 Stop 单：价格触发后自动转 Market（简化版：放在内存队列，每次撮合前检查触发）
5. 实现 GTD 过期检查
6. 阅读 §4「确定性第一性原则」，理解为什么代码不能有随机性、不能依赖 map 遍历顺序

**Java 特有注意 §4：**
- Java 的 `HashMap.keySet()` 遍历顺序不稳定 → Go 的 `map` 同样不保证顺序！
- 撮合引擎的热路径上不能有 map 遍历
- 所有非确定性来源（系统时钟、随机数、goroutine 调度）都要隔离

---

## Day 4（周四）：持久化 WAL + 快照

**文章：** §5（状态机模型、WAL、快照、主备同步）

**今晚要做：**
1. 定义 `Event` 结构体（序列化撮合引擎的输入）
   ```go
   type Event struct {
       Seq   int64
       Type  EventType // NEW_ORDER, CANCEL_ORDER
       Order Order
   }
   ```
2. 实现简化版 WAL（预写日志）
   - 每条 Event 追加写入文件 + fsync
   - 启动时回放 WAL 重建订单簿状态
3. 实现快照
   - 将 Orderbook 序列化为 JSON/二进制
   - 写入 `.snapshot` 文件
   - 启动时从快照恢复 + 回放快照之后的新 WAL
4. 测试：重启后订单簿状态一致

**Java 对照：**
- `os.File` + `bufio.Writer` ≈ `FileChannel` + `ByteBuffer`
- `encoding/json` ≈ Jackson / Gson
- Go 没有 try-catch → 手动 `if err != nil`

---

## Day 5（周五）：引擎架构 + 性能模型

**文章：** §6、§7（单线程无锁、Disruptor、缓存友好、代码骨架）

**今晚要做：**
1. 构建 `Engine` 结构体，管理多交易对的订单簿
   ```go
   type Engine struct {
       orderbooks map[string]*Orderbook
       incoming   chan OrderCommand
       wal        *WAL
   }
   ```
2. 用 goroutine + channel 实现事件循环
   - 一个 goroutine 读 channel，串行处理
   - 这就是 Go 版的「单线程事件循环」
3. 添加命令类型：`SubmitOrder`、`CancelOrder`、`Snapshot`
4. 整合之前所有的 Orderbook 代码
5. 重构项目结构为 `internal/` 分层

**项目结构（今晚完成后）：**
```
crypto-exchange/
├── cmd/exchange/main.go          # 入口：启动引擎
├── internal/
│   ├── orderbook/
│   │   ├── order.go
│   │   ├── limit.go
│   │   ├── orderbook.go
│   │   └── orderbook_test.go
│   ├── engine/
│   │   ├── engine.go
│   │   └── engine_test.go
│   ├── matching/
│   │   └── matching.go
│   └── persistence/
│       ├── wal.go
│       └── snapshot.go
└── docs/
    └── learning-plan.md
```

---

## Day 6（周六）：状态机 + 集成测试 + 端到端

**文章：** §8（订单状态机、撮合事件流）

**今晚要做：**
1. 实现订单状态机（Received → Validated → Accepted → PartiallyFilled / Filled / Canceled / Expired / Rejected）
2. 端到端测试：命令行启动引擎 → 提交订单 → 查看成交 → 查看订单簿
3. 简单 CLI 交互（可选，加分项）：
   ```
   > BUY BTC/USDT QTY=1 PRICE=65000
   > SELL BTC/USDT QTY=0.5 PRICE=65100
   > BOOK BTC/USDT
   > TRADES BTC/USDT
   ```
4. 把所有测试跑通：`go test -v -cover ./...`

---

## Day 7（周日）：文档 + 基准测试 + GitHub 发布

**文章：** §9-13（开源参考、坑点清单、进阶话题）

**今晚要做：**
1. 写 `docs/architecture.md`（参照文章 §10.4 落地清单自查）
2. 写 `README.md`
3. 基准测试：`go test -bench=. -benchmem`
4. 对照坑点清单逐项检查
5. `git init && git push` 到 github.com/212874798/crypto-exchange
6. 复盘：哪些东西学会了，哪些还没吃透

**文档模板在下面。**

---

## Java ↔ Go 速查表（贴在旁边）

| 概念 | Java | Go |
|------|------|-----|
| 类/结构体 | `class Order {}` | `type Order struct {}` |
| 构造函数 | `new Order()` | `NewOrder()` 惯例 |
| 方法 | 类内定义 | `func (o *Order) Method()` |
| 可见性 | public/private | 首字母大小写 |
| 接口 | `interface` | `interface`（隐式实现） |
| 动态数组 | `ArrayList<T>` | `[]T` (slice) |
| Map | `HashMap<K,V>` | `map[K]V` |
| 线程 | `Thread` / `ExecutorService` | goroutine `go func()` |
| 并发队列 | `BlockingQueue` | `chan T` |
| 锁 | `synchronized` / `ReentrantLock` | `sync.Mutex` |
| 原子操作 | `AtomicInteger` | `sync/atomic` |
| 测试 | `@Test` + JUnit | `func TestXxx(t *testing.T)` |
| 构建 | Maven/Gradle `pom.xml` | `go.mod` + `go build` |
| 序列化 | Jackson | `encoding/json` |
| 异常 | try-catch-finally | `if err != nil` + return |
| 枚举 | `enum` | `const` + `iota` |
| 泛型 | `List<String>` | `[]string` (slice 天然泛型) |

---

## 每日节奏

```
20:00-20:10  回顾昨天的代码 + 看今天微信推送的任务
20:10-21:20  写代码（核心 70 分钟）
21:20-21:30  休息
21:30-22:15  测试 + 调试
22:15-22:30  提交代码 + 简单笔记
```

---

## 进度追踪

| 日期 | 天数 | 完成 | commit |
|------|------|------|--------|
| 5/25 周一 | Day 1 | □ | |
| 5/26 周二 | Day 2 | □ | |
| 5/27 周三 | Day 3 | □ | |
| 5/28 周四 | Day 4 | □ | |
| 5/29 周五 | Day 5 | □ | |
| 5/30 周六 | Day 6 | □ | |
| 5/31 周日 | Day 7 | □ | |
