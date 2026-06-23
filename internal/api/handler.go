// 本文件实现三个 HTTP handler：查看订单簿、下单、撤单。
package api

import (
	"encoding/json"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/212874798/crypto-exchange/internal/exchange"
	"github.com/212874798/crypto-exchange/internal/orderbook"
)

// Handler 持有 Exchange 引用，所有路由方法都挂在它上面。
type Handler struct {
	ex *exchange.Exchange
}

// NewHandler 构造一个绑定到指定 Exchange 的 Handler。
func NewHandler(ex *exchange.Exchange) *Handler {
	return &Handler{ex: ex}
}

// GetOrderbook 处理 GET /orderbook?market=XXX
// 直接复用 Orderbook.Snapshot() 拿一份带锁的一致性快照，再包上市场名返回。
func (h *Handler) GetOrderbook(c echo.Context) error {
	market := c.QueryParam("market")
	ob, ok := h.ex.Orderbook(exchange.Market(market))
	if !ok {
		return c.JSON(404, "Market not found")
	}

	snap := ob.Snapshot()
	return c.JSON(200, map[string]any{
		"market": market,
		"asks":   snap.Asks,
		"bids":   snap.Bids,
	})
}

// PlaceOrder 处理 POST /order，按订单类型分发到限价/市价两种领域方法。
func (h *Handler) PlaceOrder(c echo.Context) error {
	var req PlaceOrderRequest
	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
		return c.JSON(400, "Invalid request")
	}

	ob, ok := h.ex.Orderbook(req.Market)
	if !ok {
		return c.JSON(404, "Market not found")
	}

	order := orderbook.NewOrder(req.Size, req.Bid)

	switch req.Type {
	case OrderTypeLimit:
		ob.PlaceLimitOrder(req.Price, order)
		return c.JSON(200, map[string]any{
			"message": "Limit order placed successfully",
			"orderId": order.ID,
		})

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

// CancelOrder 处理 DELETE /order/:market/:id
func (h *Handler) CancelOrder(c echo.Context) error {
	ob, ok := h.ex.Orderbook(exchange.Market(c.Param("market")))
	if !ok {
		return c.JSON(404, "Market not found")
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(400, "Invalid order id")
	}

	if err := ob.CancelOrder(id); err != nil {
		return c.JSON(404, map[string]string{"error": err.Error()})
	}
	return c.JSON(200, map[string]any{"cancelled": id})
}
