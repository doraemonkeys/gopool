# gopool
A simple and practical worker pool,You can use gopool to control the number of goroutines.



## QuickStart

```go
package main

import (
	"fmt"
	"time"

	"github.com/Doraemonkeys/gopool"
)

func main() {
	//create a pool with max size 100, task queue size 20, initial size 50
	p := gopool.NewPool(10, 5, 6)
	fmt.Println("current worker num:", p.WorkerNum())
	//add task to pool
	for i := 0; i < 20; i++ {
		i := i
		p.Schedule(func() {
			fmt.Println("task", i)
		})
	}
	fmt.Println("current worker num:", p.WorkerNum())
	time.Sleep(time.Second)
	p.Close()
}
```

