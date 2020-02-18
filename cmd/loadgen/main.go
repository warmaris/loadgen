package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/warmaris/loadgen/internal"
	"io/ioutil"
	"log"
	"os"
)

var usageStr = `
Usage: app [options]

Options:
    -f    <filepath>        Path to config file
    -t    <number>          Target RPS (request per second)
    -a    <number>          Amount of requests
    -u    <URL>             URL for testing
    -m    <string>          HTTP Method name
`

func usage() {
	fmt.Print(usageStr)
	os.Exit(0)
}

func main() {
	var filePath string
	var targetRps int
	var amount int
	var url string
	var method string

	flag.StringVar(&filePath, "f", "", "Path to config file")
	flag.StringVar(&method, "m", "POST", "HTTP Method name")
	flag.StringVar(&url, "u", "", "URL for testing")
	flag.IntVar(&targetRps, "t", 100, "Target RPS (request per second)")
	flag.IntVar(&amount, "a", 1000, "Amount of requests")

	flag.Usage = usage
	flag.Parse()

	var config internal.Config

	if filePath != "" {
		data, err := ioutil.ReadFile(filePath)
		if err != nil {
			log.Fatalf("cannot read config file: %s", err.Error())
		}

		err = json.Unmarshal(data, &config)
		if err != nil {
			log.Fatalf("config error: %s", err.Error())
		}
	} else {
		config = internal.Config{
			Url:    url,
			Method: method,
			Headers: map[string]string{
				"Content-Type": "text/plain",
			},
			Payload: "Sending req #$CURRENT of $TOTAL",
		}
	}

	if config.Url == "" {
		log.Print("URL must be provided")
		flag.Usage()
	}
	if config.TargetRPS == 0 {
		config.TargetRPS = targetRps
	}
	if config.Amount == 0 {
		config.Amount = amount
	}
	if config.Method == "" {
		config.Method = "POST"
	}

	worker := internal.NewWorker(config)
	worker.Run()
}
