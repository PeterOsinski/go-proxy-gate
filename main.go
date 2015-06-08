package main

import (
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const IP_LIST_FILENAME = "ip_list"
const GATE_PORT = "8080"
const GATE_TIMEOUT = 10 //seconds

const TIMEOUT_TEST_URL = "http://api.ipify.org"

var logger log.Logger
var gates []Gate

type Gate struct {
	address *url.URL
	timeout []int
}

func loadIpList() {
	content, err := ioutil.ReadFile(IP_LIST_FILENAME)
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
		Timeout:   time.Duration(GATE_TIMEOUT * time.Second),
	}

	start := time.Now()
	resp, err := client.Get(url)
	dur := int(time.Since(start) / 1000000)

	logger.Printf("Request to %s with gate %s took %d ms", url, g.address.String(), dur)
	responses <- RequestWithTime{resp, dur, err, &g}
}

func initLogger() {
	logger = *log.New(os.Stdout, "", log.Ldate|log.Lmicroseconds)
	logger.Print("Logger initialized")
}

func createServer() {
	http.HandleFunc("/", handle)

	logger.Printf("Server listening on port %s", GATE_PORT)
	http.ListenAndServe(":"+GATE_PORT, nil)
}

func handle(w http.ResponseWriter, r *http.Request) {

	gate := getRandomGate()

	response := make(chan RequestWithTime, 1)
	makeGetRequest(*gate, r.URL.String(), response)
	resp := <-response

	if resp.err == nil {
		robots, _ := ioutil.ReadAll(resp.response.Body)
		w.Write(robots)
	} else {
		w.Write([]byte{0, 0, 0, 0})
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
		if resp.err == nil && resp.response.StatusCode == 200 && resp.response.ContentLength >= 7 && resp.response.ContentLength <= 15{
			contents, _ := ioutil.ReadAll(resp.response.Body)
			if contents != nil {
				logger.Printf("Took time - %dms, response: %s (%d)", resp.time, contents, resp.response.ContentLength)
				resp.gate.timeout = append(resp.gate.timeout, resp.time)
				workingGates = append(workingGates, *resp.gate)
			}
		}
	}
	close(responses)

	gates = workingGates

	logger.Printf("Testing gates timeouts, completed. Working gates: %d", len(workingGates))
}

func main() {
	initLogger()
	loadIpList()
	testGates()
	createServer()
}

func getRandomGate() *Gate {
	rand.Seed(time.Now().UnixNano())
	return &gates[rand.Intn(len(gates))]
}
