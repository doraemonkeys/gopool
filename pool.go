package gopool

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrScheduleTimeout returned by Pool to indicate that there no free
// goroutines during some period of time.
var ErrScheduleTimeout = errors.New("schedule error: timed out")
var ErrReSetMaxSizeTimeout = errors.New("setSize error: timed out,maybe some worker is busy")

// Pool contains logic of goroutine reuse.
type Pool struct {
	sem  chan struct{}
	work chan func()
	// 控制worker退出
	kill          chan struct{}
	workerNum     int
	workerNumLock sync.Mutex
	// 用于close所有worker
	ctx context.Context
	// 用于close所有worker
	ctxCancel context.CancelFunc
}

// NewPool creates new goroutine pool with given size. It also creates a work
// queue of given size. Finally, it spawns given amount of goroutines immediately.
func NewPool(size, queue, spawn int) *Pool {
	if spawn <= 0 && queue > 0 {
		panic("dead queue configuration detected")
	}
	if spawn > size {
		panic("spawn > workers")
	}
	if size <= 0 || queue < 0 {
		panic("invalid pool configuration")
	}
	p := &Pool{
		sem:  make(chan struct{}, size),
		work: make(chan func(), queue),
		kill: make(chan struct{}),
	}
	p.ctx, p.ctxCancel = context.WithCancel(context.Background())
	p.workerNumLock.Lock()
	for i := 0; i < spawn; i++ {
		p.sem <- struct{}{}
		go p.worker(func() {})
		p.workerNum++
	}
	p.workerNumLock.Unlock()
	return p
}

// Schedule schedules task to be executed over pool's workers.
func (p *Pool) Schedule(task func()) {
	p.schedule(task, nil)
}

// ScheduleTimeout schedules task to be executed over pool's workers.
// It returns ErrScheduleTimeout when no free workers met during given timeout.
func (p *Pool) ScheduleTimeout(timeout time.Duration, task func()) error {
	return p.schedule(task, time.After(timeout))
}

// 分四种情况讨论
// p.work没满，p.sem没满:  任务可能投放到p.work中，也可能起一个新的goroutine处理
// p.work没满，p.sem已满:  任务投放p.work中
// p.work已满，p.sem没满:  任务被新的goroutine处理
// p.work已满，p.sem已满:  任务投放到p.work被阻塞，如果有timeout，则可能超时返回
func (p *Pool) schedule(task func(), timeout <-chan time.Time) error {
	select {
	case <-timeout:
		return ErrScheduleTimeout
	case p.work <- task:
		return nil
	case p.sem <- struct{}{}:
		//p.sem的len一定大于等于p.workerNum
		p.workerNumLock.Lock()
		p.workerNum++
		p.workerNumLock.Unlock()
		go p.worker(task) //必须先p.workerNum++，否则ReSize或DestoryWorker可能会出错
		return nil
	case <-p.ctx.Done():
		return p.ctx.Err()
	}
}

func (p *Pool) worker(task func()) {

	task()

	for {
		select {
		case task := <-p.work:
			task()
		case <-p.ctx.Done():
			return
		case <-p.kill:
			<-p.sem //释放令牌
			return
		}
	}
}

// 关闭协程池，所有worker退出，如果worker正在处理任务，会等待任务完成后退出。
// 关闭后不应该再对pool进行任何操作。

// Close closes pool and all its workers, if the worker is busy, it will wait for the task to complete.
// After Close() is called, pool is not usable anymore.
func (p *Pool) Close() {
	p.ctxCancel()
	p.work = nil
}

func (p *Pool) WorkerNum() int {
	return p.workerNum
}

// DestoryWorker尝试让指定数量的worker退出，返回实际即将销毁的worker数量。
// 由于可能没有worker或者worker正在处理任务，所以实际销毁的worker数量可能小于num。
// 由于不能保证在销毁的过程中没有增加其他worker，且workerNum的变化可能会存在延迟，
// 所以调用DestoryWorker后不能立即调用ReSetMaxSize(需等待所有被kill的work退出完成，时间一般很短)。

// DestoryWorker tries to destory specified number of workers, and returns the actual number of workers destoryed.
// Because there may be no worker or the worker is busy, the actual number of workers destoryed may be less than num.
// Because it can't guarantee that there is no other worker created during the destorying process,and the change of workerNum may be delayed,
// so you should not call ReSetMaxSize immediately after calling DestoryWorker (you should wait for all the killed workers to exit, which usually takes a short time).
func (p *Pool) DestoryWorker(num int) (destoryed int) {
	destoryed = 0
	for i := 0; i < num; i++ {
		select {
		case p.kill <- struct{}{}:
			destoryed++
		default:
			// 没有worker或者worker正在处理任务
		}
	}
	// 更新workerNum
	p.workerNumLock.Lock()
	p.workerNum -= destoryed
	p.workerNumLock.Unlock()
	return
}

// 重新设置任务队列的长度需要杀死所有worker，所以暂不支持
// func (p *Pool) ReSetQueueLen(len int)

// ReSetMaxSize重新设置pool的size，如果size大于原来的size，则不会创建新的worker，只有当有任务投放时才会创建新的worker。
// 如果要将pool的size减小，ReSetMaxSize函数会调用DestoryWorker，所以此时最好有足够多的空闲worker，
// 否则可能会阻塞直到超时。如果超时会返回错误ErrReSetMaxSizeTimeout。
// 调用ReSetMaxSize的过程中不能再主动调用DestoryWorker。

// ReSetMaxSize sets the size of pool, if size is greater than the original size, no new worker will be created,
// only when there is a task to be placed will a new worker be created.
// If you want to reduce the size of the pool, the ReSetMaxSize function will call DestoryWorker,
// so it is better to have enough idle workers at this time, otherwise it may be blocked until timeout.
// If timeout, ErrReSetMaxSizeTimeout will be returned.
// You should not call DestoryWorker actively during the process of calling ReSetMaxSize.
func (p *Pool) ReSetMaxSize(size int, timeout time.Duration) error {
	fmt.Println("ReSetMaxSize", size, timeout)
	if size <= 0 {
		panic("invalid pool configuration")
	}
	if size == cap(p.sem) {
		return nil
	}
	old := p.sem
	//阻塞sem，防止产生新的worker
	p.sem = make(chan struct{})

	// 如果schedule()没有被并发调用，一般不会出现len(old) != p.workerNum的情况
	for i := 0; len(old) != p.workerNum; i++ {
		//等待p.workerNum与len(old)相等
		time.Sleep(time.Millisecond * 10)
		if i > 200 { //max wait 2s
			//workPool逻辑出错
			panic("unexpected error, sem size is not equal to workerNum")
		}
	}

	returnSem := make(chan struct{})
	killNum := 0
	go func() {
		//p.sem被替换为无缓冲的chan，所以退出的worker阻塞
		for {
			select {
			//使worker能正常退出
			case p.sem <- struct{}{}:
				killNum++
			case <-returnSem:
				return
			}
		}
	}()
	if p.workerNum > size {
		fmt.Println("ReSetMaxSize", size, timeout, "destory")
		destoryed := 0
		now := time.Now()
		oldWorkerNum := p.workerNum
		for {
			destoryed += p.DestoryWorker(oldWorkerNum - size - destoryed)
			fmt.Println("ReSetMaxSize", size, timeout, "destory", destoryed)
			if destoryed == oldWorkerNum-size {
				break //销毁完成
			}
			if time.Since(now) > timeout {
				//超时,把sem还原
				for i := 0; i < destoryed; i++ {
					<-old
				}
				for killNum != destoryed {
					time.Sleep(time.Millisecond * 10)
				}
				fmt.Println("destoryed worker num:", destoryed, "killNum:", killNum)
				returnSem <- struct{}{} //结束上面的goroutine
				p.sem = old
				return ErrReSetMaxSizeTimeout
			}
			time.Sleep(time.Millisecond * 10)
		}
		for killNum != destoryed {
			time.Sleep(time.Millisecond * 10)
		}
		fmt.Println("destoryed worker num2:", destoryed, "killNum:", killNum)
	}
	fmt.Println("ReSetMaxSize", size, timeout, "new")
	returnSem <- struct{}{} //结束上面的goroutine

	new := make(chan struct{}, size)
	p.workerNumLock.Lock()
	for i := 0; i < p.workerNum; i++ {
		new <- struct{}{}
	}
	p.sem = new
	p.workerNumLock.Unlock()
	return nil
}
