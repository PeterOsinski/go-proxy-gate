package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type CLI struct {
	IP_LIST_FILENAME string
	PORT				string
	GATE_TIMEOUT     int
	MAX_RETRIES      int
}

var CF CLI

const TIMEOUT_TEST_URL = "http://api.ipify.org"

var logger log.Logger
var gates []Gate
var activeRequests int

type Gate struct {
	address *url.URL
	timeout []int
}

func loadIpList() {
	content, err := ioutil.ReadFile(CF.IP_LIST_FILENAME)
	if err != nil {
		//Do something
	}
	gateList := strings.Split(string(content), "\n")
	logger.Printf("Loaded IP list with %d items", len(gateList))

	for _, address := range gateList {
		proxyUrl, _ := url.Parse("http://" + address)
		gates = append(gates, Gate{address: proxyUrl})
	}
}

type RequestWithTime struct {
	response *http.Response
	time     int
	err      error
	gate     *Gate
}

func makeGetRequest(g Gate, url string, responses chan RequestWithTime) {
	client := &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(g.address)},
		Timeout:   time.Duration(time.Duration(CF.GATE_TIMEOUT) * time.Second),
	}

	start := time.Now()
	resp, err := client.Get(url)
	dur := int(time.Since(start) / 1000000)

	//	logger.Printf("Request to %s with gate %s took %d ms", url, g.address.String(), dur)
	responses <- RequestWithTime{resp, dur, err, &g}
}

func initLogger() {
	logger = *log.New(os.Stdout, "", log.Ldate|log.Lmicroseconds)
	logger.Print("Logger initialized")
}

func createServer() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		activeRequests++
		handle(w, r, 1)
		activeRequests--
	})

	logger.Printf("Server listening on port %s", CF.PORT)
	err := http.ListenAndServe(":"+ CF.PORT, nil)
	
	if err != nil {
		logger.Fatal(err)
	}
}

func handle(w http.ResponseWriter, r *http.Request, retryCount int) {

	gate := getRandomGate()

	response := make(chan RequestWithTime, 1)
	makeGetRequest(*gate, r.URL.String(), response)
	resp := <-response

	if resp.err == nil {
		robots, _ := ioutil.ReadAll(resp.response.Body)
		w.Write(robots)
	} else if retryCount < CF.MAX_RETRIES {
		retryCount += 1
		//		logger.Printf("Retry request (%d) for: %s", retryCount, r.URL.String())
		handle(w, r, retryCount)
	}
}

func testGates() {

	logger.Printf("Testing gates timeouts, %d gates", len(gates))
	responses := make(chan RequestWithTime, len(gates))

	for _, gate := range gates {
		go makeGetRequest(gate, TIMEOUT_TEST_URL, responses)
	}

	workingGates := []Gate{}
	for a := 1; a <= len(gates); a++ {
		resp := <-responses
		if resp.err == nil && resp.response.StatusCode == 200 && resp.response.ContentLength >= 7 && resp.response.ContentLength <= 15 {
			contents, _ := ioutil.ReadAll(resp.response.Body)
			if contents != nil {
				//				logger.Printf("Took time - %dms, response: %s (%d)", resp.time, contents, resp.response.ContentLength)
				resp.gate.timeout = append(resp.gate.timeout, resp.time)
				workingGates = append(workingGates, *resp.gate)
				fmt.Printf(".")
			}
		}
	}
	fmt.Printf("\n")
	close(responses)

	gates = workingGates

	logger.Printf("Testing gates timeouts, completed. Working gates: %d", len(gates))
}

func parseCli() {
	flag.StringVar(&CF.IP_LIST_FILENAME, "ip_list_file", "ip_list", "Filename with ip gateways")
	flag.IntVar(&CF.GATE_TIMEOUT, "gate_timeout", 10, "Maximum time (in seconds) to wait for response from gate")
	flag.IntVar(&CF.MAX_RETRIES, "max_retries", 10, "Maximum number of retries for one request")
	flag.StringVar(&CF.PORT, "port", "8080", "Listen server port")

	flag.Parse()
	
	logger.Printf("%+v\n", &CF)
}

func main() {
	initLogger()
	parseCli()
	loadIpList()
	testGates()
	showStatus()
	createServer()
}

func showStatus() {
	ticker := time.NewTicker(time.Second)
	go func() {
		for _ = range ticker.C {
			logger.Printf("Active requests: %d, active gates: %d", activeRequests, len(gates))
		}
	}()
}

func getRandomGate() *Gate {
	rand.Seed(time.Now().UnixNano())
	return &gates[rand.Intn(len(gates))]
}
