// 本文件定义价位 Limit：同一价格上所有订单的容器，以及在该价位上的撮合逻辑。
package orderbook

import "sort"

// Limit 表示订单簿中的一个价位，聚合了挂在该价格上的所有同向订单。
type Limit struct {
	Price       float64
	Orders      []*Order
	TotalVolume float64
}

// Limits 是 Limit 的切片别名，用于绑定不同的排序策略。
type Limits []*Limit

// ByBestAsk 按价格升序排序：卖盘最优价（最低价）排在最前。
type ByBestAsk struct{ Limits }

func (a ByBestAsk) Len() int           { return len(a.Limits) }
func (a ByBestAsk) Swap(i, j int)      { a.Limits[i], a.Limits[j] = a.Limits[j], a.Limits[i] }
func (a ByBestAsk) Less(i, j int) bool { return a.Limits[i].Price < a.Limits[j].Price }

// ByBestBid 按价格降序排序：买盘最优价（最高价）排在最前。
type ByBestBid struct{ Limits }

func (b ByBestBid) Len() int           { return len(b.Limits) }
func (b ByBestBid) Swap(i, j int)      { b.Limits[i], b.Limits[j] = b.Limits[j], b.Limits[i] }
func (b ByBestBid) Less(i, j int) bool { return b.Limits[i].Price > b.Limits[j].Price }

// NewLimit 创建一个空的价位。
func NewLimit(price float64) *Limit {
	return &Limit{
		Price:  price,
		Orders: []*Order{},
	}
}

// AddOrder 把订单挂到当前价位末尾，并维护反向指针与总量。
func (l *Limit) AddOrder(o *Order) {
	o.Limit = l
	l.Orders = append(l.Orders, o)
	l.TotalVolume += o.Size
}

// DeleteOrder 从当前价位移除指定订单。
// 采用「与末尾交换 + 截短」的方式删除以避免 O(n) 搬移，
// 之后再对剩余订单按时间戳重新排序，保证撮合顺序不被破坏。
func (l *Limit) DeleteOrder(o *Order) {
	for i := 0; i < len(l.Orders); i++ {
		if l.Orders[i] == o {
			l.Orders[i] = l.Orders[len(l.Orders)-1]
			l.Orders = l.Orders[:len(l.Orders)-1]
			break
		}
	}
	l.TotalVolume -= o.Size
	sort.Sort(Orders(l.Orders))
}

// Fill 用对手方订单 o 尝试吃掉本价位上的订单，直到 o 完全成交或本价位被吃光。
// 第二个返回值是本次完全成交（Size 归零）的订单列表，交由上层 Orderbook
// 从全局索引里清理 —— 这样 Limit 自身不需要感知索引结构。
func (l *Limit) Fill(o *Order) ([]Match, []*Order) {
	var (
		matches       []Match
		orderToDelete []*Order
	)

	for _, order := range l.Orders {
		match := l.fillOrder(order, o)
		matches = append(matches, match)
		l.TotalVolume -= match.SizeFilled

		if order.Size == 0.0 {
			orderToDelete = append(orderToDelete, order)
		}

		if o.Size == 0.0 {
			break
		}
	}

	for _, order := range orderToDelete {
		l.DeleteOrder(order)
	}

	return matches, orderToDelete
}

// fillOrder 计算两笔订单的撮合结果：取较小一方的 Size 作为成交量，
// 分别扣减双方剩余量，并按 Bid 字段区分买卖方组装 Match。
func (l *Limit) fillOrder(a, b *Order) Match {
	var (
		bid, ask   *Order
		sizeFilled float64
	)
	if a.Bid {
		bid = a
		ask = b
	} else {
		bid = b
		ask = a
	}

	if a.Size >= b.Size {
		a.Size -= b.Size
		sizeFilled = b.Size
		b.Size = 0.0
	} else {
		b.Size -= a.Size
		sizeFilled = a.Size
		a.Size = 0.0
	}

	return Match{
		Ask:        ask,
		Bid:        bid,
		Price:      l.Price,
		SizeFilled: sizeFilled,
	}
}
