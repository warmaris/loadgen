package internal

import (
	"fmt"
	"strconv"
	"time"
)

const logStringSegmentsCount = 6

type payload struct {
	id   int
	body string
}

type result struct {
	id         int
	startTime  time.Time
	endTime    time.Time
	isSuccess  bool
	statusCode int
	errMsg     string
}

func (r result) getLogString() string {
	success := 0
	if r.isSuccess {
		success = 1
	}
	return fmt.Sprintf("%d,%s,%s,%d,%d,%s",
		r.id,
		r.startTime.Format("2006-01-02 15:04:05.000000"),
		r.endTime.Format("2006-01-02 15:04:05.000000"),
		success,
		r.statusCode,
		strconv.Quote(r.errMsg))
}
