package main

import (
	"container/heap"
	"fmt"
	"sort"
	"time"
)

// task represents asynchronous work.
type task struct {
	fn       func()
	priority time.Duration
	index    int
}

// evloop schedules tasks using a priority queue.
type evloop struct {
	now  time.Duration
	done chan struct{}
	q    []*task
}

func (ev *evloop) Len() int {
	return len(ev.q)
}

func (ev *evloop) Less(i, j int) bool {
	return ev.q[i].priority < ev.q[j].priority
}

func (ev *evloop) Swap(i, j int) {
	ev.q[i], ev.q[j] = ev.q[j], ev.q[i]
	ev.q[i].index = i
	ev.q[j].index = j
}

func (ev *evloop) Push(x interface{}) {
	n := len(ev.q)
	tsk := x.(*task)
	tsk.index = n
	ev.q = append(ev.q, tsk)
}

func (ev *evloop) Pop() interface{} {
	old := ev.q
	n := len(old)
	tsk := old[n-1]
	old[n-1] = nil
	tsk.index = -1
	ev.q = old[0 : n-1]
	return tsk
}

func (ev *evloop) update(tsk *task, fn func(), priority time.Duration) {
	tsk.fn = fn
	tsk.priority = priority
	heap.Fix(ev, tsk.index)
}

// setTimeout schedules a function to run after a given duration.
// It returns a cancellation function rather than an opaque token
// to simplify the implementation.
func (ev *evloop) setTimeout(fn func(), timeout time.Duration) func() {
	tsk := &task{
		fn:       fn,
		priority: timeout + ev.now,
		index:    -1,
	}
	heap.Push(ev, tsk)

	return func() {
		idx := tsk.index
		i := sort.Search(len(ev.q), func(i int) bool {
			return ev.q[i].index >= idx
		})
		if ev.q[i] != tsk {
			return
		}
		ev.update(tsk, tsk.fn, 0)
		heap.Pop(ev)
	}
}

func (ev *evloop) setInterval(fn func(), timeout time.Duration) func() {
	var loop, cancel func()
	loop = func() {
		fn()
		cancel = ev.setTimeout(loop, timeout)
	}
	cancel = ev.setTimeout(loop, timeout)
	return func() {
		cancel()
	}
}

// run executes each task in the queue and exits when done.
func (ev *evloop) run() chan struct{} {
	go func() {
		for ev.Len() > 0 {
			tsk := heap.Pop(ev).(*task)
			timeout := tsk.priority - ev.now
			select {
			case <-time.After(timeout):
				ev.now = tsk.priority
				tsk.fn()
			}
		}
		ev.done <- struct{}{}
		close(ev.done)
	}()
	return ev.done
}

func newEvloop() *evloop {
	ev := &evloop{
		q:    make([]*task, 0),
		done: make(chan struct{}),
	}
	heap.Init(ev)
	return ev
}

func main() {
	ev := newEvloop()

	// This is how NOT to do concurrency in Go...
	cancelInterval := ev.setInterval(func() {
		fmt.Print(".")
	}, 100*time.Millisecond)
	ev.setTimeout(func() {
		fmt.Println("goodbye world")
		ev.setTimeout(func() {
			fmt.Println("i'm")
			cancelTimeout := ev.setTimeout(func() {
				fmt.Println("never gonna happen")
			}, 1*time.Second)
			ev.setTimeout(func() {
				fmt.Println("done")
				cancelInterval()
			}, 1*time.Second+250*time.Millisecond)
			cancelTimeout()
		}, 50*time.Millisecond)
	}, 500*time.Millisecond)
	ev.setTimeout(func() {
		fmt.Println("hello world")
	}, 50*time.Millisecond)

	<-ev.run()
}
