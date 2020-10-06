package internal

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"text/tabwriter"
	"time"
)

type Worker struct {
	config  Config
	counter *int64
	wg      *sync.WaitGroup
	pPipe   chan payload
	tPipe   <-chan time.Time
	rPipe   chan result
}

func NewWorker(config Config) *Worker {
	worker := &Worker{config: config}
	worker.counter = new(int64)
	return worker
}

// Sends requests by timer
func (w *Worker) do(cancel func()) {
	defer w.wg.Done()
	defer cancel()

	client := http.DefaultClient
	for range w.tPipe {
		data, ok := <-w.pPipe
		if !ok {
			break
		}
		payload := strings.NewReader(data.body)
		req, err := http.NewRequest(w.config.Method, w.config.Url, payload)
		if err != nil {
			// Err means wrong request params, no need to continue
			log.Printf("new request error: %s", err.Error())
			return
		}
		for name, value := range w.config.Headers {
			req.Header.Add(name, value)
		}

		log.Printf("starting request %d", data.id)
		startTime := time.Now()
		res, err := client.Do(req)
		endTime := time.Now()

		reqResult := result{
			id:        data.id,
			startTime: startTime,
			endTime:   endTime,
		}

		if err != nil {
			log.Printf("http clent error: %s", err.Error())
			reqResult.errMsg = err.Error()
		} else {
			atomic.AddInt64(w.counter, 1)
			reqResult.isSuccess = true
			reqResult.statusCode = res.StatusCode
			_ = res.Body.Close()
		}
		w.rPipe <- reqResult
	}
}

// Generates payloads for sequential requests with number of current request.
func (w *Worker) generate(ctx context.Context) {
	defer w.wg.Done()
	defer close(w.pPipe)
	for i := 1; i <= w.config.Amount; i++ {
		body := strings.ReplaceAll(w.config.Payload, "$CURRENT", strconv.Itoa(i))
		body = strings.ReplaceAll(body, "$TOTAL", strconv.Itoa(w.config.Amount))

		select {
		case <-ctx.Done():
			return
		default:
			w.pPipe <- payload{
				id:   i,
				body: body,
			}
		}
	}
}

func (w *Worker) log(wg *sync.WaitGroup) {
	defer wg.Done()

	var file *os.File
	var err error

	if w.config.Logfile == "" {
		w.config.Logfile = os.TempDir() + string(os.PathSeparator) + "loadgen.log"
	}

	file, err = os.Create(w.config.Logfile)
	if err != nil {
		log.Printf("cannot open log file to write")
	} else {
		defer file.Close()
	}

	for r := range w.rPipe {
		if file != nil {
			_, err = fmt.Fprintln(file, r.getLogString())
			if err != nil {
				log.Printf("cannot write request result: %v", err)
			}
		}
	}
}

func (w *Worker) stats() {
	var count,
		successCount,
		userErrorCount,
		serverErrorCount,
		netErrorCount int

	var overallDuration,
		successDuration,
		userErrorDuration,
		serverErrorDuration,
		netErrorDuration,
		maxDuration,
		minDuration time.Duration

	file, err := os.Open(w.config.Logfile)
	if err != nil {
		log.Printf("cannot open log file for stats: %v", err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lineParams := strings.Split(scanner.Text(), ",")
		if len(lineParams) != logStringSegmentsCount {
			log.Printf("logstring format broken, expect %d segments, got %d", logStringSegmentsCount, len(lineParams))
			return
		}

		count++
		start, err := time.Parse("2006-01-02 15:04:05.000000", lineParams[1])
		if err != nil {
			log.Printf("cannot convert start date: %v", err)
			return
		}

		end, err := time.Parse("2006-01-02 15:04:05.000000", lineParams[2])
		if err != nil {
			log.Printf("cannot convert end date: %v", err)
			return
		}

		duration := end.Sub(start)

		if minDuration == 0 || minDuration > duration {
			minDuration = duration
		}
		if maxDuration < duration {
			maxDuration = duration
		}
		overallDuration = time.Duration((int(overallDuration)*(count-1) + int(duration)) / count)

		status, err := strconv.ParseInt(lineParams[4], 10, 64)
		if err != nil {
			log.Printf("cannot parse status code: %v", err)
			return
		}

		if status == 0 {
			netErrorCount++
			netErrorDuration = time.Duration((int(netErrorDuration)*(netErrorCount-1) + int(duration)) / netErrorCount)
		} else if status < 400 {
			successCount++
			successDuration = time.Duration((int(successDuration)*(successCount-1) + int(duration)) / successCount)
		} else if status < 500 {
			userErrorCount++
			userErrorDuration = time.Duration((int(userErrorDuration)*(userErrorCount-1) + int(duration)) / userErrorCount)
		} else {
			serverErrorCount++
			serverErrorDuration = time.Duration((int(serverErrorDuration)*(serverErrorCount-1) + int(duration)) / serverErrorCount)
		}
	}

	tw := tabwriter.NewWriter(os.Stdout, 2, 2, 2, ' ', tabwriter.Debug)

	_, _ = fmt.Fprintf(tw, "Total:\tOK:\t400:\t500:\tNetErr:\tFastest:\tSlowest:\n")
	_, _ = fmt.Fprintf(tw, "%d (%dms)\t%d (%dms)\t%d (%dms)\t%d (%dms)\t%d (%dms)\t%dms\t%dms\n",
		count, overallDuration.Milliseconds(),
		successCount, successDuration.Milliseconds(),
		userErrorCount, userErrorDuration.Milliseconds(),
		serverErrorCount, serverErrorDuration.Milliseconds(),
		netErrorCount, netErrorDuration.Milliseconds(),
		minDuration.Milliseconds(), maxDuration.Milliseconds())

	_ = tw.Flush()
}

// Starts worker.
// Payloads for requests generates in separate goroutine through buffered channel for lower memory consumption
// and faster start. Requests are sending concurrently in pool of goroutines for higher rate.
func (w *Worker) Run() {
	factor := runtime.NumCPU()
	w.pPipe = make(chan payload, factor*3)
	w.rPipe = make(chan result, factor*3)
	w.wg = new(sync.WaitGroup)

	ctx, cancel := context.WithCancel(context.Background())

	w.wg.Add(1)
	go w.generate(ctx)

	timer := time.NewTicker(time.Duration(1000000000 / w.config.TargetRPS))
	w.tPipe = timer.C
	w.wg.Add(factor)

	for i := 1; i <= factor; i++ {
		go w.do(cancel)
	}

	logWg := new(sync.WaitGroup)
	logWg.Add(1)
	go w.log(logWg)

	w.wg.Wait()
	cancel()
	timer.Stop()
	close(w.rPipe)
	log.Printf("Success requests: %d", *(w.counter))

	log.Println("Writing log file")
	logWg.Wait()

	w.stats()
}
