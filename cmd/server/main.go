// Command server 是交易所 HTTP 服务的进程入口。
//
// 本文件只做装配工作：构造 Exchange、构造 Handler、注册路由、启动 echo。
// 任何业务逻辑都不应出现在这里 —— 它们分别属于 orderbook、exchange、api 三个包。
package main

import (
	"github.com/labstack/echo/v4"

	"github.com/212874798/crypto-exchange/internal/api"
	"github.com/212874798/crypto-exchange/internal/exchange"
)

func main() {
	e := echo.New()
	ex := exchange.New()
	api.Register(e, api.NewHandler(ex))

	e.Start(":5221")
}
