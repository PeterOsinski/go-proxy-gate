package main

import (
	"errors"
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

const TIMEOUT_TEST_URL = "http://api.ipify.org"

var logger log.Logger
var gates []*Gate
var uaList []string
var activeRequests int

type Gate struct {
	address *url.URL
	timeout []int
	success int
	fail    int
	client  *http.Client
}

func (g Gate) getRatio() float32 {

	if g.success == 0 && g.fail == 0 {
		return 1
	}

	if g.success > 0 && g.fail == 0 {
		return 1 + (float32(g.success) / 1)
	}

	return 1 + (float32(g.success) / float32(g.fail))
}

type RequestWithTime struct {
	response *http.Response
	time     int
	err      error
	gate     *Gate
}

func loadLinesFromFile(f string) []string {
	content, err := ioutil.ReadFile(f)
	if err != nil {
		//Do something
	}

	var list []string
	list = strings.Split(string(content), "\n")
	logger.Printf("Loaded '%s' list with %d items", f, len(list))

	return list
}

func loadIpList() {
	for _, address := range loadLinesFromFile(CF.IP_LIST_FILENAME) {
		proxyUrl, _ := url.Parse("http://" + address)
		gates = append(gates, &Gate{address: proxyUrl})
	}
}

func makeGetRequest(g *Gate, url string, responses chan RequestWithTime) {

	//	logger.Printf("Fire request to %s with gate %s", url, g.address.String())
	//	logger.Printf("Using gate with ratio %g", g.getRatio())

	req, _ := http.NewRequest("GET", url, nil)

	if len(uaList) > 0 {
		req.Header.Set("User-Agent", getRandomUA())
	}

	start := time.Now()
	resp, err := g.client.Do(req)
	dur := int(time.Since(start) / 1000000)

	//	logger.Printf("Request to %s with gate %s took %d ms", url, g.address.String(), dur)

	responses <- RequestWithTime{resp, dur, err, g}
}

func initLogger() {
	logger = *log.New(os.Stdout, "", log.Ldate|log.Lmicroseconds)
	logger.Print("Logger initialized")
}

func validateResponse(resp RequestWithTime) ([]byte, error) {
	if resp.err == nil && resp.response.StatusCode == 200 {
		content, errRead := ioutil.ReadAll(resp.response.Body)
		timeout := (resp.time - 100) > (CF.GATE_REQ_TIMEOUT * 1000)

		if errRead != nil {
			//			logger.Println("Error while reading buffer")
		}

		if timeout {
			//			logger.Println("Timeout reading body")
		}

		contentStr := string(content)

		a1 := strings.Contains(contentStr, "</html>")
		a2 := strings.Contains(contentStr, "</body>")

		if a1 && a2 && errRead == nil && !timeout && len(strings.Trim(contentStr, " ")) > 0 {
			//			logger.Println("Good", len(contentStr), resp.time)

			resp.gate.success++
			return content, nil
		}
	}

	resp.gate.fail++
	return []byte{}, errors.New("invalid response")
}

func handle(w http.ResponseWriter, r *http.Request, retryCount int) {

	responseSend := false

	response := make(chan RequestWithTime, CF.SIM_REQ)

	for i := 1; i <= CF.SIM_REQ; i++ {
		go makeGetRequest(getRandomGate(), r.URL.String(), response)
	}

	for a := 1; a <= CF.SIM_REQ; a++ {
		resp := <-response
		if content, err := validateResponse(resp); !responseSend && err == nil {
			responseSend = true
			w.Write(content)
			return
		}
	}

	if retryCount < CF.MAX_RETRIES {
		retryCount += 1
		logger.Printf("Retry request (%d) for: %s", retryCount, r.URL.String())
		handle(w, r, retryCount)
	} else {
		logger.Println("Error")
		w.Write([]byte{0})
	}
}

func testGates() {

	logger.Printf("Testing gates timeouts, %d gates", len(gates))
	responses := make(chan RequestWithTime, len(gates))

	for _, gate := range gates {

		client := new(http.Client)
		client.Transport = &http.Transport{Proxy: http.ProxyURL(gate.address)}
		client.Timeout = time.Duration(time.Duration(CF.GATE_TEST_TIMEOUT) * time.Second)
		gate.client = client

		go makeGetRequest(gate, TIMEOUT_TEST_URL, responses)
	}

	workingGates := []*Gate{}
	for a := 1; a <= len(gates); a++ {
		resp := <-responses
		if resp.err == nil && resp.response.StatusCode == 200 && resp.response.ContentLength >= 7 && resp.response.ContentLength <= 15 {
			contents, _ := ioutil.ReadAll(resp.response.Body)
			if contents != nil {
				//				logger.Printf("Took time - %dms, response: %s (%d)", resp.time, contents, resp.response.ContentLength)
				resp.gate.timeout = append(resp.gate.timeout, resp.time)

				resp.gate.client.Timeout = time.Duration(time.Duration(CF.GATE_REQ_TIMEOUT) * time.Second)
				workingGates = append(workingGates, resp.gate)
				fmt.Printf(".")
			}
		}
	}
	fmt.Printf("\n")
	close(responses)

	gates = workingGates

	logger.Printf("Testing gates timeouts, completed. Working gates: %d", len(gates))
}

func main() {
	initLogger()
	parseCli()
	loadIpList()
	testGates()
	uaList = loadLinesFromFile(CF.UA_LIST)
	showStatus()

	if CF.HTTPS != "" {
		go createHttpsServer()
	}
	createHttpServer()
}

func showStatus() {
	ticker := time.NewTicker(15 * time.Second)
	go func() {
		for _ = range ticker.C {
			logger.Printf("Active requests: %d, active gates: %d", activeRequests, len(gates))

			for _, gate := range gates {
				logger.Printf("Gate status %s, ratio %g, success: %d, fail: %d", gate.address.String(), gate.getRatio(), gate.success, gate.fail)
			}
		}
	}()
}

func getRandomUA() string {
	rand.Seed(time.Now().UnixNano())
	return uaList[rand.Intn(len(uaList))]
}

func getRandomGate() *Gate {

	rand.Seed(time.Now().UnixNano())
	var g *Gate

	getRandGateSimple := func() *Gate {
		return gates[rand.Intn(len(gates))]
	}

	if rand.Float64() >= CF.EXPLORE_BIAS {
		g = getRandomGate()
		//		logger.Printf("Exploring gates! Choosed gate with ratio %g", g.getRatio())
		return g
	}

	var sum float32
	for _, gate := range gates {
		sum += gate.getRatio()
	}

	idx := float32(rand.Intn(int(sum))) + rand.Float32()

	var last float32
	for _, gate := range gates {

		if last < idx && idx < (last+gate.getRatio()) {
			//			logger.Printf("Gate chosed! last %g, idx %g, ratio %g. ", last, idx, gate.getRatio())
			g = gate
			break
		}

		last += gate.getRatio()

	}

	if g != nil {
		return g
	} else {
		return getRandGateSimple()
	}
}
