package internal

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Worker struct {
	config  Config
	counter *int64
	wg      *sync.WaitGroup
}

func NewWorker(config Config) *Worker {
	worker := &Worker{config: config}
	worker.counter = new(int64)
	return worker
}

// Sends requests by timer
func (w *Worker) do(ctx context.Context, cancel func(), payloadPipe <-chan string, timePipe <-chan time.Time) {
	defer w.wg.Done()
	client := http.DefaultClient
	for range timePipe {
		select {
		case <-ctx.Done():
			return
		default:
		}

		data, ok := <-payloadPipe
		if !ok {
			break
		}
		payload := strings.NewReader(data)
		req, err := http.NewRequest(w.config.Method, w.config.Url, payload)
		if err != nil {
			// Err means wrong request params, no need to continue
			fmt.Printf("new request error: %s\n", err.Error())
			cancel()
			return
		}
		for name, value := range w.config.Headers {
			req.Header.Add(name, value)
		}

		res, err := client.Do(req)
		if err != nil {
			// Err means wrong URL or network problems, no need to continue
			fmt.Printf("http clent error: %s\n", err.Error())
			cancel()
			return
		}
		atomic.AddInt64(w.counter, 1)
		_ = res.Body.Close()
	}
}

// Generates payloads for sequential requests with number of current request.
func (w *Worker) generate(ctx context.Context, payloadPipe chan<- string) {
	defer w.wg.Done()
	for i := 1; i <= w.config.Amount; i++ {
		payload := strings.ReplaceAll(w.config.Payload, "$CURRENT", strconv.Itoa(i))
		payload = strings.ReplaceAll(payload, "$TOTAL", strconv.Itoa(w.config.Amount))

		select {
		case <-ctx.Done():
			close(payloadPipe)
			return
		default:
			payloadPipe <- payload
		}
	}
	close(payloadPipe)
}

// Starts worker.
// Payloads for requests generates in separate goroutine through buffered channel for lower memory consumption
// and faster start. Requests are sending concurrently in pool of goroutines for higher rate.
func (w *Worker) Run() {
	factor := runtime.NumCPU()
	payloadPipe := make(chan string, factor*3)
	w.wg = new(sync.WaitGroup)

	ctx, cancel := context.WithCancel(context.Background())

	w.wg.Add(1)
	go w.generate(ctx, payloadPipe)

	timer := time.NewTicker(time.Duration(1000000000 / w.config.TargetRPS))
	w.wg.Add(factor)

	for i := 1; i <= factor; i++ {
		go w.do(ctx, cancel, payloadPipe, timer.C)
	}

	w.wg.Wait()
	cancel()
	timer.Stop()
	fmt.Printf("Success requests: %d\n", *(w.counter))
}
