// Package orderbook 提供撮合引擎的核心数据结构与算法。
// 它不依赖任何 HTTP 或框架代码，是纯领域层。
//
// 本文件定义订单 Order 及其按时间排序的适配器，并封装订单 ID 的生成。
package orderbook

import (
	"fmt"
	"sync/atomic"
	"time"
)

// orderIDCounter 是进程内单调递增的订单 ID 计数器，封装为包私有，
// 外部通过 NewOrder 拿到下一个 ID，避免对计数器本身的依赖。
var orderIDCounter int64

// nextOrderID 原子地返回下一个订单 ID。
func nextOrderID() int64 {
	return atomic.AddInt64(&orderIDCounter, 1)
}

// Order 表示一笔订单。Bid=true 表示买单，false 表示卖单。
// Limit 字段在订单被挂入 Orderbook 后由 Limit.AddOrder 回填，
// 用于撤单时 O(1) 定位到所属价位。
type Order struct {
	ID        int64
	Size      float64
	Bid       bool
	Limit     *Limit
	TimeStamp int64
}

// NewOrder 构造一笔新订单，使用当前进程的下一个 ID 和当前 Unix 时间戳。
func NewOrder(size float64, bid bool) *Order {
	return &Order{
		ID:        nextOrderID(),
		Size:      size,
		Bid:       bid,
		TimeStamp: time.Now().Unix(),
	}
}

func (o *Order) String() string {
	return fmt.Sprintf("Order{ID: %d, Size: %.2f, Bid: %t, TimeStamp: %d}", o.ID, o.Size, o.Bid, o.TimeStamp)
}

// Orders 实现 sort.Interface，按 TimeStamp 升序排列，
// 用于同一价位下保持「价格优先、时间优先」中的时间维度。
type Orders []*Order

func (o Orders) Len() int           { return len(o) }
func (o Orders) Swap(i, j int)      { o[i], o[j] = o[j], o[i] }
func (o Orders) Less(i, j int) bool { return o[i].TimeStamp < o[j].TimeStamp }
