package main

import (
	"fmt"
	"reflect"
	"testing"
)

func assert(t *testing.T, a, b any) {
	if !reflect.DeepEqual(a, b) {
		t.Errorf("Expected %v, got %v", a, b)
	}
}

func TestLimit(t *testing.T) {
	l := NewLimit(100.0)
	buyOrderA := NewOrder(5, true)
	buyOrderB := NewOrder(10, true)
	buyOrderC := NewOrder(15, true)
	buyOrderD := NewOrder(20, true)

	l.AddOrder(buyOrderA)
	l.AddOrder(buyOrderB)
	l.AddOrder(buyOrderC)
	l.AddOrder(buyOrderD)

	l.DeleteOrder(buyOrderB)
	fmt.Println(l)
}

func TestPlaceLimitOrder(t *testing.T) {
	ob := NewOrderbook()

	sellOrderA := NewOrder(5, false)
	sellOrderB := NewOrder(10, false)
	ob.PlaceLimitOrder(100.0, sellOrderA)
	ob.PlaceLimitOrder(100.0, sellOrderB)

	assert(t, len(ob.asks), 1)
	assert(t, ob.Asks()[0].Price, 100.0)
	assert(t, len(ob.Asks()[0].Orders), 2)
	assert(t, ob.Asks()[0].Orders[0], sellOrderA)
	assert(t, ob.Asks()[0].Orders[1], sellOrderB)
}
func TestPlaceMarketOrder(t *testing.T) {
	ob := NewOrderbook()
	sellOrderA := NewOrder(5, false)
	sellOrderB := NewOrder(10, false)
	ob.PlaceLimitOrder(100.0, sellOrderA)
	ob.PlaceLimitOrder(100.0, sellOrderB)

	buyOrder := NewOrder(12, true)
	matches, err := ob.PlaceMarketOrder(buyOrder)
	assert(t, err, nil)
	assert(t, len(matches), 2)
	assert(t, matches[0].SizeFilled, 5.0)
	assert(t, matches[0].Price, 100.0)
	assert(t, matches[1].SizeFilled, 7.0)
	assert(t, matches[1].Price, 100.0)
	assert(t, buyOrder.Size, 0.0)
	assert(t, sellOrderA.Size, 0.0)
	assert(t, sellOrderB.Size, 3.0)

	//测试流动性不足的情况
	buyOrder2 := NewOrder(20, true)
	_, err = ob.PlaceMarketOrder(buyOrder2)
	if err == nil {
		t.Errorf("Expected error due to insufficient liquidity, but got nil")
	}
}

func TestCancelOrder(t *testing.T) {
	ob := NewOrderbook()
	sellOrderA := NewOrder(5, false)
	sellOrderB := NewOrder(10, false)
	ob.PlaceLimitOrder(100.0, sellOrderA)
	ob.PlaceLimitOrder(100.0, sellOrderB)
	ob.CancelOrder(sellOrderA)
	assert(t, len(ob.asks), 1)
	assert(t, ob.Asks()[0].Price, 100.0)
	assert(t, len(ob.Asks()[0].Orders), 1)
	assert(t, ob.Asks()[0].Orders[0], sellOrderB)
}
