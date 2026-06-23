// Package api 是 HTTP 适配层：把 echo 请求解码成领域调用，再把结果编码回 JSON。
// 本层只负责协议形状，所有业务规则都在 orderbook / exchange 包里。
package api

import "github.com/212874798/crypto-exchange/internal/exchange"

// OrderType 是 HTTP 请求中订单类型的枚举字符串。
// 它属于协议层概念，领域层不感知 —— handler 解析后会调用不同的领域方法。
type OrderType string

const (
	OrderTypeLimit  OrderType = "LIMIT"
	OrderTypeMarket OrderType = "MARKET"
)

// PlaceOrderRequest 是 POST /order 的请求体形状。
type PlaceOrderRequest struct {
	Type   OrderType
	Bid    bool
	Size   float64
	Price  float64
	Market exchange.Market
}
