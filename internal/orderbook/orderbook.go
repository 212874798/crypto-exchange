// 本文件定义订单簿主体 Orderbook：限价单/市价单的入口、撤单、以及对外快照。
package orderbook

import (
	"fmt"
	"sort"
	"sync"
)

// Orderbook 是单个市场的订单簿。
//
// 并发模型：所有读写都通过 mu 互斥，没有读写锁 —— 因为 Asks/Bids 读取
// 路径上会调用 sort.Sort 修改底层切片顺序，并不是纯读操作。
//
// 命名约定：大写方法（PlaceLimitOrder、Asks 等）对外暴露并自带加锁；
// 小写方法（sortedAsks、bidToVolume、clearLimit 等）是已持锁的内部版本，
// 仅供同包内已加锁的方法复用，调用时不要再次上锁。
type Orderbook struct {
	mu sync.Mutex

	asks []*Limit // 卖盘价位集合
	bids []*Limit // 买盘价位集合

	// 价格 -> 价位 的哈希索引，挂单时 O(1) 找到既有价位。
	askLimits map[float64]*Limit
	bidLimits map[float64]*Limit

	// 订单 ID -> 订单 的全局索引，撤单时 O(1) 定位订单。
	orders map[int64]*Order
}

// NewOrderbook 构造一个空的订单簿。
func NewOrderbook() *Orderbook {
	return &Orderbook{
		asks:      []*Limit{},
		bids:      []*Limit{},
		askLimits: make(map[float64]*Limit),
		bidLimits: make(map[float64]*Limit),
		orders:    make(map[int64]*Order),
	}
}

// PlaceMarketOrder 立即按对手方最优价成交，不指定价格。
// 流程：买单依次吃掉 ask 队列（升序，最优价在前），卖单依次吃掉 bid 队列（降序）。
// 若对手方总量不足以填满订单，返回错误且不做部分成交（保留原有语义）。
func (ob *Orderbook) PlaceMarketOrder(o *Order) ([]Match, error) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	matches := []Match{}

	if o.Bid {
		// 买单：吃卖盘
		if o.Size > ob.askToVolume() {
			return nil, fmt.Errorf("not enough liquidity to fill the order: need %.2f, available %.2f", o.Size, ob.askToVolume())
		}
		for _, limit := range ob.sortedAsks() {
			limitMatches, filled := limit.Fill(o)
			matches = append(matches, limitMatches...)

			for _, filledOrder := range filled {
				delete(ob.orders, filledOrder.ID)
			}

			if len(limit.Orders) == 0 {
				ob.clearLimit(false, limit)
			}
		}
	} else {
		// 卖单：吃买盘
		if o.Size > ob.bidToVolume() {
			return nil, fmt.Errorf("not enough liquidity to fill the order: need %.2f, available %.2f", o.Size, ob.bidToVolume())
		}
		for _, limit := range ob.sortedBids() {
			limitMatches, filled := limit.Fill(o)
			matches = append(matches, limitMatches...)

			for _, filledOrder := range filled {
				delete(ob.orders, filledOrder.ID)
			}

			if len(limit.Orders) == 0 {
				ob.clearLimit(true, limit)
			}
		}
	}

	return matches, nil
}

// PlaceLimitOrder 把订单挂到指定价格，不立即撮合。
// 如果该价位已存在则复用，否则新建并接入价位列表与价格索引。
func (ob *Orderbook) PlaceLimitOrder(price float64, o *Order) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	var limit *Limit
	if o.Bid {
		limit = ob.bidLimits[price]
	} else {
		limit = ob.askLimits[price]
	}

	if limit == nil {
		limit = NewLimit(price)
		if o.Bid {
			ob.bids = append(ob.bids, limit)
			ob.bidLimits[price] = limit
		} else {
			ob.asks = append(ob.asks, limit)
			ob.askLimits[price] = limit
		}
	}

	limit.AddOrder(o)
	ob.orders[o.ID] = o
}

// CancelOrder 按 ID 撤单：从所在价位移除，并清理全局索引。
// 若价位被清空则同时从订单簿中移除该价位。
func (ob *Orderbook) CancelOrder(id int64) error {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	o, ok := ob.orders[id]
	if !ok {
		return fmt.Errorf("order %d not found", id)
	}

	limit := o.Limit
	limit.DeleteOrder(o)
	delete(ob.orders, id)

	if len(limit.Orders) == 0 {
		ob.clearLimit(o.Bid, limit)
	}
	return nil
}

// clearLimit 从订单簿移除一个空价位。已持锁版本。
// 使用「与末尾交换 + 截短」避免 O(n) 搬移；价位顺序不重要，会被排序方法恢复。
func (ob *Orderbook) clearLimit(bid bool, l *Limit) {
	if bid {
		delete(ob.bidLimits, l.Price)
		for i := 0; i < len(ob.bids); i++ {
			if ob.bids[i] == l {
				ob.bids[i] = ob.bids[len(ob.bids)-1]
				ob.bids = ob.bids[:len(ob.bids)-1]
			}
		}
	} else {
		delete(ob.askLimits, l.Price)
		for i := 0; i < len(ob.asks); i++ {
			if ob.asks[i] == l {
				ob.asks[i] = ob.asks[len(ob.asks)-1]
				ob.asks = ob.asks[:len(ob.asks)-1]
			}
		}
	}
}

// BidToVolume / AskToVolume 返回买/卖盘当前总挂单量（对外加锁版本）。
func (ob *Orderbook) BidToVolume() float64 {
	ob.mu.Lock()
	defer ob.mu.Unlock()
	return ob.bidToVolume()
}

func (ob *Orderbook) AskToVolume() float64 {
	ob.mu.Lock()
	defer ob.mu.Unlock()
	return ob.askToVolume()
}

// bidToVolume / askToVolume 是已持锁的内部版本，供同包方法复用。
func (ob *Orderbook) bidToVolume() float64 {
	total := float64(0)
	for _, limit := range ob.bids {
		total += limit.TotalVolume
	}
	return total
}

func (ob *Orderbook) askToVolume() float64 {
	total := float64(0)
	for _, limit := range ob.asks {
		total += limit.TotalVolume
	}
	return total
}

// Asks 返回按最优价排序的卖盘价位列表。
// 注意：返回的切片指向内部存储，调用方使用期间不应再触发写操作。
func (ob *Orderbook) Asks() []*Limit {
	ob.mu.Lock()
	defer ob.mu.Unlock()
	return ob.sortedAsks()
}

// Bids 返回按最优价排序的买盘价位列表。
func (ob *Orderbook) Bids() []*Limit {
	ob.mu.Lock()
	defer ob.mu.Unlock()
	return ob.sortedBids()
}

// sortedAsks / sortedBids 是已持锁的内部排序版本。
// 排序原则：卖盘升序、买盘降序，使撮合时从队首开始即为最优价。
func (ob *Orderbook) sortedAsks() []*Limit {
	sort.Sort(ByBestAsk{ob.asks})
	return ob.asks
}

func (ob *Orderbook) sortedBids() []*Limit {
	sort.Sort(ByBestBid{ob.bids})
	return ob.bids
}

// OrderCount 返回当前订单簿中尚未成交/撤销的订单数，
// 仅供测试与监控；持锁读取。
func (ob *Orderbook) OrderCount() int {
	ob.mu.Lock()
	defer ob.mu.Unlock()
	return len(ob.orders)
}

// --- 对外只读快照 -----------------------------------------------------------

// OrderSnapshot 是订单的对外只读视图，用于序列化给客户端。
type OrderSnapshot struct {
	ID        int64   `json:"id"`
	Size      float64 `json:"size"`
	Bid       bool    `json:"bid"`
	TimeStamp int64   `json:"timestamp"`
}

// LevelSnapshot 是一个价位的对外只读视图。
type LevelSnapshot struct {
	Price       float64         `json:"price"`
	TotalVolume float64         `json:"totalVolume"`
	Orders      []OrderSnapshot `json:"orders"`
}

// Snapshot 是整个订单簿的对外只读视图。
type Snapshot struct {
	Bids []LevelSnapshot `json:"bids"`
	Asks []LevelSnapshot `json:"asks"`
}

// Snapshot 持锁拍取一份订单簿快照，保证读到的不是撮合中途的状态。
// 返回值与内部存储完全解耦，调用方可安全地长期持有或异步序列化。
func (ob *Orderbook) Snapshot() Snapshot {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	return Snapshot{
		Bids: levelsToSnapshot(ob.sortedBids()),
		Asks: levelsToSnapshot(ob.sortedAsks()),
	}
}

func levelsToSnapshot(levels []*Limit) []LevelSnapshot {
	out := make([]LevelSnapshot, 0, len(levels))
	for _, l := range levels {
		orders := make([]OrderSnapshot, 0, len(l.Orders))
		for _, o := range l.Orders {
			orders = append(orders, OrderSnapshot{
				ID:        o.ID,
				Size:      o.Size,
				Bid:       o.Bid,
				TimeStamp: o.TimeStamp,
			})
		}
		out = append(out, LevelSnapshot{
			Price:       l.Price,
			TotalVolume: l.TotalVolume,
			Orders:      orders,
		})
	}
	return out
}
