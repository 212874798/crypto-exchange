package main

import (
	"fmt"
	"testing"
)

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

func TestOrderbook(t *testing.T) {
	ob := NewOrderbook()

	buyOrderA := NewOrder(5, true)
	ob.PlaceOrder(18, buyOrderA)

	fmt.Println(ob)

}
