package internal

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestFire(t *testing.T) {
	count := new(int64)
	*count = 0

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(count, 1)
		body, err := ioutil.ReadAll(r.Body)
		defer r.Body.Close()

		if err != nil {
			http.Error(w, err.Error(), 500)
			t.Error("Cannot read body from test request")
			return
		}
		fmt.Println(string(body))
	}))

	config := Config{
		Url:       mock.URL,
		Amount:    10,
		TargetRPS: 10000,
		Headers: map[string]string{
			"Content-Type": "text/plain",
		},
		Payload: "Sending req #$CURRENT of $TOTAL",
	}
	worker := NewWorker(config)

	start := time.Now()
	worker.Run()

	fmt.Printf("%d hits for %v\n", *count, time.Since(start))
	if *count != 10 {
		t.Error("Count mismatch")
	}
}

func TestWrongURL(t *testing.T) {
	config := Config{
		Url:       "not_an_URL",
		Amount:    1000,
		TargetRPS: 10000,
		Headers: map[string]string{
			"Content-Type": "text/plain",
		},
		Payload: "Sending req #$CURRENT of $TOTAL",
	}
	worker := NewWorker(config)

	worker.Run()
	if *(worker.counter) > 0 {
		t.Error("Must not make any requests")
	}
}

func TestWrongMethod(t *testing.T) {
	count := new(int64)
	*count = 0

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(count, 1)
		defer r.Body.Close()
		fmt.Println(r.Method)
	}))

	config := Config{
		Url:       mock.URL,
		Method:    "\t\n\r",
		Amount:    10,
		TargetRPS: 10000,
		Headers: map[string]string{
			"Content-Type": "text/plain",
		},
		Payload: "Sending req #$CURRENT of $TOTAL",
	}
	worker := NewWorker(config)

	worker.Run()
	if *(worker.counter) > 0 {
		t.Error("Must not make any requests")
	}
}

func TestServerError(t *testing.T) {
	count := new(int64)
	*count = 0

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(count, 1)
		defer r.Body.Close()

		http.Error(w, "Internal Error", 500)
	}))

	config := Config{
		Url:       mock.URL,
		Amount:    10,
		TargetRPS: 10000,
		Headers: map[string]string{
			"Content-Type": "text/plain",
		},
		Payload: "Sending req #$CURRENT of $TOTAL",
	}
	worker := NewWorker(config)

	start := time.Now()
	worker.Run()

	fmt.Printf("%d hits for %v\n", *count, time.Since(start))
	if *count != 10 {
		t.Error("Count mismatch")
	}
}
