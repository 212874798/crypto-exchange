// Package exchange 是应用层：把多个市场（Market）聚合到一个交易所对象，
// 每个市场对应一份独立的订单簿。HTTP 层通过本包按市场路由请求。
package exchange

import "github.com/212874798/crypto-exchange/internal/orderbook"

// Market 是市场（交易对）标识。当前只接入 ETH，后续可以追加。
type Market string

const (
	MarketETH Market = "ETH"
)

// Exchange 持有所有市场的订单簿。
//
// 当前实现是一个简单的 map，初始化时把支持的市场预置进去；
// 运行期不增删市场，所以 map 本身不需要加锁 —— 并发安全由各 Orderbook 自己负责。
type Exchange struct {
	orderbooks map[Market]*orderbook.Orderbook
}

// New 构造一个 Exchange，预置默认市场。
func New() *Exchange {
	books := make(map[Market]*orderbook.Orderbook)
	books[MarketETH] = orderbook.NewOrderbook()
	return &Exchange{orderbooks: books}
}

// Orderbook 按市场名取对应的订单簿，未注册的市场返回 nil, false。
func (ex *Exchange) Orderbook(m Market) (*orderbook.Orderbook, bool) {
	ob, ok := ex.orderbooks[m]
	return ob, ok
}
