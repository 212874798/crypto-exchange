package main

import (
	"encoding/json"

	"github.com/labstack/echo/v4"
)

func main() {
	e := echo.New()
	ex := NewExchange()

	e.GET("/orderbook", ex.handleGetOrderbook)
	e.POST("/order", ex.handlePlaceOrder)

	e.Start(":5221")

}

type OrderType string

const (
	OrderTypeLimit  OrderType = "LIMIT"
	OrderTypeMarket OrderType = "MARKET"
)

type Market string

const (
	MarketETH Market = "ETH"
)

type Exchange struct {
	orderbooks map[Market]*Orderbook
}

func NewExchange() *Exchange {
	orderbooks := make(map[Market]*Orderbook)
	orderbooks[MarketETH] = NewOrderbook()
	return &Exchange{
		orderbooks: orderbooks,
	}
}

type PlaceOrderRequest struct {
	Type   OrderType
	Bid    bool
	Size   float64
	Price  float64
	Market Market
}

func (ex *Exchange) handleGetOrderbook(c echo.Context) error {
	market := c.QueryParam("market")
	ob, exists := ex.orderbooks[Market(market)]
	if !exists {
		return c.JSON(404, "Market not found")
	}

	// 构建买单视图
	bids := make([]map[string]any, 0)
	for _, limit := range ob.Bids() {
		orders := make([]map[string]any, 0)
		for _, o := range limit.Orders {
			orders = append(orders, map[string]any{
				"size":      o.Size,
				"bid":       o.Bid,
				"timestamp": o.TimeStamp,
			})
		}
		bids = append(bids, map[string]any{
			"price":       limit.Price,
			"totalVolume": limit.TotalVolume,
			"orders":      orders,
		})
	}

	// 构建卖单视图
	asks := make([]map[string]any, 0)
	for _, limit := range ob.Asks() {
		orders := make([]map[string]any, 0)
		for _, o := range limit.Orders {
			orders = append(orders, map[string]any{
				"size":      o.Size,
				"bid":       o.Bid,
				"timestamp": o.TimeStamp,
			})
		}
		asks = append(asks, map[string]any{
			"price":       limit.Price,
			"totalVolume": limit.TotalVolume,
			"orders":      orders,
		})
	}

	return c.JSON(200, map[string]any{
		"market": market,
		"asks":   asks,
		"bids":   bids,
	})
}

func (ex *Exchange) handlePlaceOrder(c echo.Context) error {
	var placeOrderReq PlaceOrderRequest
	if err := json.NewDecoder(c.Request().Body).Decode(&placeOrderReq); err != nil {
		return c.JSON(400, "Invalid request")
	}

	market := Market(placeOrderReq.Market)
	ob, exists := ex.orderbooks[market]
	if !exists {
		return c.JSON(404, "Market not found")
	}

	order := NewOrder(placeOrderReq.Size, placeOrderReq.Bid)

	switch placeOrderReq.Type {
	case OrderTypeLimit:
		ob.PlaceLimitOrder(placeOrderReq.Price, order)
		return c.JSON(200, "Limit order placed successfully")

	case OrderTypeMarket:
		matches, err := ob.PlaceMarketOrder(order)
		if err != nil {
			return c.JSON(400, map[string]string{"error": err.Error()})
		}
		return c.JSON(200, map[string]any{
			"message": "Market order placed successfully",
			"matches": len(matches),
		})

	default:
		return c.JSON(400, "Invalid order type: must be LIMIT or MARKET")
	}

}
