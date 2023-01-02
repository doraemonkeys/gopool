package gopool

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestNewPool(t *testing.T) {
	type args struct {
		size  int
		queue int
		spawn int
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
		{"test1", args{size: 1, queue: 1, spawn: 1}},
		{"test2", args{size: 2, queue: 2, spawn: 2}},
		{"panic1", args{size: 0, queue: 2, spawn: 2}},
		{"panic2", args{size: 2, queue: -1, spawn: 3}},
		{"panic3", args{size: 2, queue: 2, spawn: 3}},
		{"panic4", args{size: 2, queue: 2, spawn: -1}},
		{"panic5", args{size: 1, queue: -1, spawn: 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if strings.Contains(tt.name, "panic") {
				defer func() {
					if r := recover(); r == nil {
						t.Errorf("The code did not panic")
					}
				}()
			}
			got := NewPool(tt.args.size, tt.args.queue, tt.args.spawn)
			if got == nil {
				t.Fatalf("get nil")
			}
			if cap(got.sem) != tt.args.size {
				t.Errorf("get sem size = %v, want %v", cap(got.sem), tt.args.size)
			}
			if cap(got.work) != tt.args.queue {
				t.Errorf("get work size = %v, want %v", cap(got.work), tt.args.queue)
			}
			if got.workerNum != len(got.sem) {
				t.Errorf("get workerNum = %v, want %v", got.workerNum, len(got.sem))
			}
		})
	}
}

func TestPool_Schedule(t *testing.T) {
	type args struct {
		task func()
	}
	tests := []struct {
		name string
		p    *Pool
		args args
	}{
		// TODO: Add test cases.
		{"test1", NewPool(1, 1, 1), args{func() {}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.p.Schedule(tt.args.task)
			if cap(tt.p.work) != 1 {
				t.Errorf("get work size = %v, want %v", cap(tt.p.work), 1)
			}
			if cap(tt.p.sem) != 1 {
				t.Errorf("get sem size = %v, want %v", cap(tt.p.sem), 1)
			}
			if tt.p.workerNum != 1 {
				t.Errorf("get workerNum = %v, want %v", tt.p.workerNum, 1)
			}
		})
	}
}

func TestPool_schedule(t *testing.T) {
	type args struct {
		task    func()
		timeout <-chan time.Time
	}
	tests := []struct {
		name    string
		p       *Pool
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{"test1", NewPool(1, 1, 1), args{func() {}, nil}, false},
		{"test2", NewPool(10, 1, 10), args{func() {}, nil}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.p.schedule(tt.args.task, tt.args.timeout); (err != nil) != tt.wantErr {
				t.Errorf("Pool.schedule() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	p := NewPool(10, 100, 10)
	for i := 0; i < 100; i++ {
		p.Schedule(func() {

		})
	}
	if cap(p.work) != 100 {
		t.Errorf("get work size = %v, want %v", cap(p.work), 100)
	}
	if len(p.sem) != 10 {
		t.Errorf("get sem size = %v, want %v", len(p.sem), 10)
	}
	if p.workerNum != 10 {
		t.Errorf("get workerNum = %v, want %v", p.workerNum, 10)
	}
	var err error
	for i := 0; i < 112; i++ {
		err = p.ScheduleTimeout(time.Millisecond*100, func() {
			time.Sleep(time.Second)
		})
	}
	if err == nil {
		t.Errorf("get err = %v, want %v", err, "timeout")
	}
	fmt.Println("ok")
	p = NewPool(10, 5, 6)
	if p.workerNum != 6 {
		t.Errorf("get workerNum = %v, want %v", p.workerNum, 6)
	}
	for i := 0; i < 15; i++ {
		p.Schedule(func() {
		})
	}
	if p.workerNum != 10 {
		t.Errorf("get workerNum = %v, want %v", p.workerNum, 10)
	}
	destoryed := p.DestoryWorker(5)
	time.Sleep(time.Second) //wait for destory
	if p.workerNum != 10-destoryed {
		t.Errorf("get workerNum = %v, want %v", p.workerNum, 10-destoryed)
	}
	if p.WorkerNum() != p.workerNum {
		t.Errorf("get workerNum = %v, want %v", p.WorkerNum(), p.workerNum)
	}
	for i := 0; i < 1009; i++ {
		p.Schedule(func() {
		})
	}
	p.DestoryWorker(5)
	go func(p *Pool) {
		for i := 0; i < 1009; i++ {
			p.Schedule(func() {
			})
		}
	}(p)
	p.Close()

	fmt.Println("ok")
	p = NewPool(10, 5, 10)
	if p.workerNum != 10 {
		t.Errorf("get workerNum = %v, want %v", p.workerNum, 10)
	}
	fmt.Println("ok1")
	err = p.ReSetMaxSize(20, 0)
	fmt.Println("err", err)
	if err != nil {
		t.Errorf("get err = %v, want %v", err, nil)
	}
	fmt.Println("ok2")
	if p.workerNum != 10 {
		t.Errorf("get workerNum = %v, want %v", p.workerNum, 10)
	}
	fmt.Println("ok3")
	for i := 0; i < 200; i++ {
		p.Schedule(func() {
		})
	}
	fmt.Println("ok4")
	if p.workerNum != 20 {
		t.Errorf("get workerNum = %v, want %v", p.workerNum, 20)
	}
	time.Sleep(time.Second)
	fmt.Println("ok5")
	err = p.ReSetMaxSize(18, 0)
	fmt.Println("ok6")
	if err != nil {
		t.Errorf("get err = %v, want %v", err, nil)
	}
	if p.workerNum != 18 {
		t.Errorf("get workerNum = %v, want %v", p.workerNum, 10)
	}
	fmt.Println("ok7")
	err = p.ReSetMaxSize(18, 0)
	fmt.Println("ok8")
	if err != nil {
		t.Errorf("get err = %v, want %v", err, nil)
	}
	p.Close()

	p = NewPool(20, 500, 20)
	if p.workerNum != 20 {
		t.Errorf("get workerNum = %v, want %v", p.workerNum, 20)
	}
	for i := 0; i < 530; i++ {
		p.work <- func() {
			time.Sleep(time.Second)
		}
	}
	err = p.ReSetMaxSize(1, time.Second+time.Millisecond*100)
	if err == nil {
		t.Errorf("get err = %v, want %v", nil, ErrReSetMaxSizeTimeout)
	}

	p = NewPool(10, 0, 10)
	for i := 0; i < 10; i++ {
		i := i
		p.Schedule(func() {
			sleep := time.Millisecond * time.Duration(i) * 200
			time.Sleep(sleep)
		})
	}
	err = p.ReSetMaxSize(1, 0)
	if err != ErrReSetMaxSizeTimeout {
		t.Errorf("get err = %v, want %v", err, ErrReSetMaxSizeTimeout)
	}
}
