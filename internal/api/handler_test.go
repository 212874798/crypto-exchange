package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/212874798/crypto-exchange/internal/exchange"
)

// newTestServer 起一个 echo 实例 + Exchange + Handler，并通过 Register 装好路由，
// 与 cmd/server/main.go 的真实启动路径完全一致。
func newTestServer() (*echo.Echo, *exchange.Exchange) {
	e := echo.New()
	ex := exchange.New()
	Register(e, NewHandler(ex))
	return e, ex
}

func placeLimit(t *testing.T, e *echo.Echo, market exchange.Market, bid bool, price, size float64) int64 {
	t.Helper()
	body, _ := json.Marshal(PlaceOrderRequest{
		Type:   OrderTypeLimit,
		Bid:    bid,
		Size:   size,
		Price:  price,
		Market: market,
	})
	req := httptest.NewRequest(http.MethodPost, "/order", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("place limit: expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		OrderID int64 `json:"orderId"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode place limit response: %v", err)
	}
	if resp.OrderID == 0 {
		t.Fatalf("expected non-zero order id, got %s", rec.Body.String())
	}
	return resp.OrderID
}

func TestHandleCancelOrder_Success(t *testing.T) {
	e, ex := newTestServer()

	id := placeLimit(t, e, exchange.MarketETH, false, 100.0, 5)

	// 撤单
	req := httptest.NewRequest(http.MethodDelete, "/order/"+string(exchange.MarketETH)+"/"+strconv.FormatInt(id, 10), nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// JSON number 默认解成 float64
	if got, _ := resp["cancelled"].(float64); int64(got) != id {
		t.Fatalf("expected cancelled=%d, got %v", id, resp["cancelled"])
	}

	// orderbook 应已为空：Limit 被清理、Orders 索引为空
	ob, _ := ex.Orderbook(exchange.MarketETH)
	if n := len(ob.Asks()); n != 0 {
		t.Fatalf("expected 0 ask limits after cancel, got %d", n)
	}
	if n := ob.OrderCount(); n != 0 {
		t.Fatalf("expected empty Orders index, got %d", n)
	}
}

func TestHandleCancelOrder_NotFound(t *testing.T) {
	e, _ := newTestServer()

	req := httptest.NewRequest(http.MethodDelete, "/order/"+string(exchange.MarketETH)+"/99999", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleCancelOrder_UnknownMarket(t *testing.T) {
	e, _ := newTestServer()

	req := httptest.NewRequest(http.MethodDelete, "/order/BTC/1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleCancelOrder_BadID(t *testing.T) {
	e, _ := newTestServer()

	req := httptest.NewRequest(http.MethodDelete, "/order/"+string(exchange.MarketETH)+"/notanumber", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

// 撤掉一个价位上 N 个订单中的一个，确认 Limit 仍在、剩余订单还在
func TestHandleCancelOrder_PartialLevel(t *testing.T) {
	e, ex := newTestServer()

	idA := placeLimit(t, e, exchange.MarketETH, false, 100.0, 5)
	_ = placeLimit(t, e, exchange.MarketETH, false, 100.0, 10)

	req := httptest.NewRequest(http.MethodDelete, "/order/"+string(exchange.MarketETH)+"/"+strconv.FormatInt(idA, 10), nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	ob, _ := ex.Orderbook(exchange.MarketETH)
	asks := ob.Asks()
	if len(asks) != 1 {
		t.Fatalf("expected 1 ask limit, got %d", len(asks))
	}
	if len(asks[0].Orders) != 1 {
		t.Fatalf("expected 1 remaining order at the level, got %d", len(asks[0].Orders))
	}
	if asks[0].TotalVolume != 10 {
		t.Fatalf("expected TotalVolume=10, got %f", asks[0].TotalVolume)
	}
	if n := ob.OrderCount(); n != 1 {
		t.Fatalf("expected 1 remaining order in index, got %d", n)
	}
}
