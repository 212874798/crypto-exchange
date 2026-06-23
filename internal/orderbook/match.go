// 本文件定义撮合结果 Match。
package orderbook

// Match 表示一次成交：哪笔买单和哪笔卖单在什么价格成交了多少量。
// Ask / Bid 指向原订单，调用方可据此回写成交流水。
type Match struct {
	Ask        *Order
	Bid        *Order
	Price      float64
	SizeFilled float64
}
