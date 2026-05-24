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
	totalVolume float64
}

type Limits []*Limit

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
	l.totalVolume += o.Size
}

func (l *Limit) DeleteOrder(o *Order) {
	for i := 0; i < len(l.Orders); i++ {
		if l.Orders[i] == o {
			// 交换最后一个元素到当前位置，然后缩短切片长度
			//这样可以减少时间复杂度，将需要排序的操作交给撮合引擎
			l.Orders[i] = l.Orders[len(l.Orders)-1]
			l.Orders = l.Orders[:len(l.Orders)-1]
		}
	}
	l.totalVolume -= o.Size
	sort.Sort(Orders(l.Orders))
}

type Orderbook struct {
	Asks []*Limit
	Bids []*Limit

	AskLimits map[float64]*Limit
	BidLimits map[float64]*Limit
}

func NewOrderbook() *Orderbook {
	return &Orderbook{
		Asks:      []*Limit{},
		Bids:      []*Limit{},
		AskLimits: make(map[float64]*Limit),
		BidLimits: make(map[float64]*Limit),
	}
}

func (ob *Orderbook) PlaceOrder(price float64, o *Order) []Match {

	if o.Size > 0 {
		ob.add(price, o)
	}

	return []Match{}
}

func (ob *Orderbook) add(price float64, o *Order) {
	var limit *Limit
	if o.Bid {
		limit = ob.BidLimits[price]
	} else {
		limit = ob.AskLimits[price]
	}

	if limit == nil {
		limit = NewLimit(price)
		if o.Bid {
			ob.Bids = append(ob.Bids, limit)
			ob.BidLimits[price] = limit
		} else {
			ob.Asks = append(ob.Asks, limit)
			ob.AskLimits[price] = limit
		}
	}

	limit.AddOrder(o)
}
