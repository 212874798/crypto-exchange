// 本文件集中路由注册，便于一眼看清整套 HTTP 接口。
package api

import "github.com/labstack/echo/v4"

// Register 把所有 API 路由挂到给定的 echo 实例上。
// main.go 与集成测试都通过它来装配路由，避免两处写两遍。
func Register(e *echo.Echo, h *Handler) {
	e.GET("/orderbook", h.GetOrderbook)
	e.POST("/order", h.PlaceOrder)
	e.DELETE("/order/:market/:id", h.CancelOrder)
}
