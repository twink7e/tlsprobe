package common

import (
	"fmt"
	"reflect"
	"sync"
)

func wrapper(w *WaitPool, f interface{}, args ...interface{}) {
	fn := reflect.ValueOf(f)
	if fn.Type().NumIn() != len(args) {
		panic(fmt.Sprintf("invaild input parameters of function %v", fn.Type()))
	}
	inputs := make([]reflect.Value, len(args))
	for k, in := range args {
		inputs[k] = reflect.ValueOf(in)
	}
	fn.Call(inputs)
	<-w.ch
	w.wg.Done()
}

type WaitPool struct {
	limit uint
	ch    chan struct{}
	wg    *sync.WaitGroup
}

func NewWaitPool(limit uint) *WaitPool {
	return &WaitPool{
		limit: limit,
		ch:    make(chan struct{}, limit),
		wg:    new(sync.WaitGroup),
	}
}

func (w *WaitPool) Run(f interface{}, args ...interface{}) {
	w.wg.Add(1)
	w.ch <- struct{}{}
	go wrapper(w, f, args...)
}

func (w *WaitPool) Wait() {
	w.wg.Wait()
}
