Simple API load generator
===

We needed to test our API server how many rps it can handle. Known tools for load testing are overkill, 
so I made this simple program. It can generate sequential requests with high rate to single URL with custom payload,
using current request index and total number of requests. No statistics, no time metrics, just requests. 
That's all we needed.

Usage
---

Go v1.13+ required. No dependencies, just `go install ./cmd/...` and run `loadgen` with any options below. URL is required.

Options:

    -f    <filepath>        Path to config file
    -t    <number>          Target RPS (request per second)
    -a    <number>          Amount of requests
    -u    <URL>             URL for testing
    -m    <string>          HTTP Method name

You can use it without config file. In that case, payload is only simple string with numbers of current and total requests.
No additional headers will be added.

For custom requests you should use config file. A json file like [this](cmd/loadgen/conf.json).
