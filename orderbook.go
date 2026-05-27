package main

import (
	"fmt"
	"sort"
	"time"
)

type Match struct {
	Ask        *Order
	Bid        *Order
	Price      float64
	SizeFilled float64
}

type Order struct {
	Size      float64
	Bid       bool
	Limit     *Limit
	TimeStamp int64
}

// 实现go的sort接口，方便对订单进行时间排序
type Orders []*Order

func (o Orders) Len() int           { return len(o) }
func (o Orders) Swap(i, j int)      { o[i], o[j] = o[j], o[i] }
func (o Orders) Less(i, j int) bool { return o[i].TimeStamp < o[j].TimeStamp }

func NewOrder(size float64, bid bool) *Order {
	return &Order{
		Size:      size,
		Bid:       bid,
		TimeStamp: time.Now().Unix(),
	}
}

func (o *Order) String() string {
	return fmt.Sprintf("Order{Size: %.2f, Bid: %t, TimeStamp: %d}", o.Size, o.Bid, o.TimeStamp)
}

type Limit struct {
	Price       float64
	Orders      []*Order
	TotalVolume float64
}

type Limits []*Limit

// 实现go的sort接口，方便对限价单进行排序
type ByBestAsk struct{ Limits }

func (a ByBestAsk) Len() int           { return len(a.Limits) }
func (a ByBestAsk) Swap(i, j int)      { a.Limits[i], a.Limits[j] = a.Limits[j], a.Limits[i] }
func (a ByBestAsk) Less(i, j int) bool { return a.Limits[i].Price < a.Limits[j].Price }

type ByBestBid struct{ Limits }

func (b ByBestBid) Len() int           { return len(b.Limits) }
func (b ByBestBid) Swap(i, j int)      { b.Limits[i], b.Limits[j] = b.Limits[j], b.Limits[i] }
func (b ByBestBid) Less(i, j int) bool { return b.Limits[i].Price > b.Limits[j].Price }

func NewLimit(price float64) *Limit {
	return &Limit{
		Price:  price,
		Orders: []*Order{},
	}
}

func (l *Limit) AddOrder(o *Order) {
	o.Limit = l
	l.Orders = append(l.Orders, o)
	l.TotalVolume += o.Size
}

func (l *Limit) DeleteOrder(o *Order) {
	for i := 0; i < len(l.Orders); i++ {
		if l.Orders[i] == o {
			// 交换最后一个元素到当前位置，然后缩短切片长度
			//这样可以减少时间复杂度，将需要排序的操作交给撮合引擎
			l.Orders[i] = l.Orders[len(l.Orders)-1]
			l.Orders = l.Orders[:len(l.Orders)-1]
			break
		}
	}
	l.TotalVolume -= o.Size
	sort.Sort(Orders(l.Orders))
}

// 遍历一个限价单层级中的所有订单，尝试用这个订单去撮合市场价订单，直到市场价订单完全成交或者这个限价单层级的订单全部被吃掉
func (l *Limit) Fill(o *Order) []Match {
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

	return matches
}

// 成交一个订单，返回成交结果
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

type Orderbook struct {
	asks []*Limit //卖方要价
	bids []*Limit //买方出价

	//哈希表，价格到限价单的映射，方便快速查找和删除订单
	AskLimits map[float64]*Limit
	BidLimits map[float64]*Limit
}

func NewOrderbook() *Orderbook {
	return &Orderbook{
		asks:      []*Limit{},
		bids:      []*Limit{},
		AskLimits: make(map[float64]*Limit),
		BidLimits: make(map[float64]*Limit),
	}
}

// 市价单，不指定价格，立即以最优价格成交
// 市场价需要吃掉对手所有价位的订单，直到订单完全成交或者没有更多的订单可以吃掉
func (ob *Orderbook) PlaceMarketOrder(o *Order) ([]Match, error) {
	matches := []Match{}

	//如果是买单，吃掉卖单
	if o.Bid {
		if o.Size > ob.AskToVolume() {
			return nil, fmt.Errorf("not enough liquidity to fill the order: need %.2f, available %.2f", o.Size, ob.AskToVolume())
		}
		for _, limit := range ob.Asks() {
			limitMatches := limit.Fill(o)
			matches = append(matches, limitMatches...)

			if len(limit.Orders) == 0 {
				ob.clearLimit(false, limit)
			}
		}

		//如果是卖单，吃掉买单
	} else {
		if o.Size > ob.BidToVolume() {
			return nil, fmt.Errorf("not enough liquidity to fill the order: need %.2f, available %.2f", o.Size, ob.BidToVolume())
		}
		for _, limit := range ob.Bids() {
			limitMatches := limit.Fill(o)
			matches = append(matches, limitMatches...)

			if len(limit.Orders) == 0 {
				ob.clearLimit(true, limit)
			}
		}
	}

	return matches, nil
}

// 现价单，指定价格匹配时才成交
func (ob *Orderbook) PlaceLimitOrder(price float64, o *Order) {
	var limit *Limit
	if o.Bid {
		limit = ob.BidLimits[price]
	} else {
		limit = ob.AskLimits[price]
	}

	if limit == nil {
		limit = NewLimit(price)
		if o.Bid {
			ob.bids = append(ob.bids, limit)
			ob.BidLimits[price] = limit
		} else {
			ob.asks = append(ob.asks, limit)
			ob.AskLimits[price] = limit
		}
	}

	limit.AddOrder(o)
}

func (ob *Orderbook) clearLimit(bid bool, l *Limit) {
	if bid {
		delete(ob.BidLimits, l.Price)
		for i := 0; i < len(ob.bids); i++ {
			if ob.bids[i] == l {
				ob.bids[i] = ob.bids[len(ob.bids)-1]
				ob.bids = ob.bids[:len(ob.bids)-1]
			}
		}
	} else {
		delete(ob.AskLimits, l.Price)
		for i := 0; i < len(ob.asks); i++ {
			if ob.asks[i] == l {
				ob.asks[i] = ob.asks[len(ob.asks)-1]
				ob.asks = ob.asks[:len(ob.asks)-1]
			}
		}
	}
}

func (ob *Orderbook) CancelOrder(o *Order) {
	limit := o.Limit
	limit.DeleteOrder(o)
}

func (ob *Orderbook) BidToVolume() float64 {
	total := float64(0)
	for _, limit := range ob.bids {
		total += limit.TotalVolume
	}
	return total
}

func (ob *Orderbook) AskToVolume() float64 {
	total := float64(0)
	for _, limit := range ob.asks {
		total += limit.TotalVolume
	}
	return total
}

// 实现卖最优价排序，撮合引擎会从最优价开始撮合订单，所以需要对卖单进行升序排序，买单进行降序排序
func (ob *Orderbook) Asks() []*Limit {
	sort.Sort(ByBestAsk{ob.asks})
	return ob.asks
}

// 实现买最优价排序，撮合引擎会从最优价开始撮合订单，所以需要对卖单进行升序排序，买单进行降序排序
func (ob *Orderbook) Bids() []*Limit {
	sort.Sort(ByBestBid{ob.bids})
	return ob.bids
}
