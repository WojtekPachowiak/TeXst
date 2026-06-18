package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

var (
	target     = flag.String("url", "http://localhost:8080/render", "target URL")
	clients    = flag.Int("clients", 20, "number of concurrent workers")
	requests   = flag.Int("requests", 100, "total number of request to send")
	bodyLength = flag.Int("length", 20, "random text length for the document body")
	engine     = flag.String("engine", "latex", "text rendering engine (latex or typst)")
	format     = flag.String("format", "pdf", "output format (pdf or png)")
)

func main() {
	flag.Parse()
	if *requests <= 0 || *clients <= 0 || *bodyLength <= 0 {
		fmt.Println("clients, requests, and bodyLength must be greater than zero")
		return
	}

	fmt.Printf("Load testing %s with %d requests using %d clients (options: %s,%s)\n", *target, *requests, *clients, *engine, *format)

	jobs := make(chan int, *requests)
	var wg sync.WaitGroup
	var mu sync.Mutex
	successes := 0
	failures := 0

	for i := 0; i < *clients; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range jobs {
				if err := sendRequest(); err != nil {
					mu.Lock()
					failures++
					mu.Unlock()
				} else {
					mu.Lock()
					successes++
					mu.Unlock()
				}
			}
		}()
	}

	for i := range *requests {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	fmt.Printf("Done: %d successes, %d failures\n", successes, failures)
}

func sendRequest() error {
	source := generateRandomSource(*bodyLength, *engine)
	payload := url.Values{
		"engine": {*engine},
		"format": {*format},
		"source": {source},
	}

	resp, err := http.PostForm(*target, payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

func generateRandomSource(length int, engine string) string {
	if length <= 0 {
		length = 20
	}

	allowed := []rune("abcdefghijklmnopqrstuvwxyz ")
	var builder strings.Builder
	builder.Grow(length)

	for i := 0; i < length; i++ {
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(allowed))))
		builder.WriteRune(allowed[idx.Int64()])
	}

	out := strings.TrimSpace(builder.String())
	if engine == "latex"{
		out = fmt.Sprintf(`\documentclass{article}\begin{document}%s\end{document}`, out)
	}

	return out
}
