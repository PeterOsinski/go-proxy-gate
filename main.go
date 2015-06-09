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
	IP_LIST_FILENAME  string
	PORT              string
	GATE_TEST_TIMEOUT int
	GATE_REQ_TIMEOUT  int
	MAX_RETRIES       int
	UA_LIST           string
}

var CF CLI

const TIMEOUT_TEST_URL = "http://api.ipify.org"

var logger log.Logger
var gates []Gate
var uaList []string
var activeRequests int

type Gate struct {
	address *url.URL
	timeout []int
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
		gates = append(gates, Gate{address: proxyUrl})
	}
}

type RequestWithTime struct {
	response *http.Response
	time     int
	err      error
	gate     *Gate
}

func makeGetRequest(g Gate, url string, timeout int, responses chan RequestWithTime) {
	client := &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(g.address)},
		Timeout:   time.Duration(time.Duration(timeout) * time.Second),
	}

	req, _ := http.NewRequest("GET", url, nil)

	if len(uaList) > 0 {
		req.Header.Set("User-Agent", getRandomUA())
	}

	start := time.Now()
	resp, err := client.Do(req)
	dur := int(time.Since(start) / 1000000)

	//		logger.Printf("Request to %s with gate %s took %d ms", url, g.address.String(), dur)
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
	err := http.ListenAndServe(":"+CF.PORT, nil)

	if err != nil {
		logger.Fatal(err)
	}
}

func handle(w http.ResponseWriter, r *http.Request, retryCount int) {

	gate := getRandomGate()

	response := make(chan RequestWithTime, 1)
	makeGetRequest(*gate, r.URL.String(), CF.GATE_REQ_TIMEOUT, response)
	resp := <-response

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
		go makeGetRequest(gate, TIMEOUT_TEST_URL, CF.GATE_TEST_TIMEOUT, responses)
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
	flag.IntVar(&CF.GATE_TEST_TIMEOUT, "gate_test_timeout", 10, "Maximum time (in seconds) to wait for response from gate when testing")
	flag.IntVar(&CF.GATE_REQ_TIMEOUT, "gate_req_timeout", 10, "Maximum time (in seconds) to wait for response from gate")
	flag.IntVar(&CF.MAX_RETRIES, "max_retries", 10, "Maximum number of retries for one request")
	flag.StringVar(&CF.PORT, "port", "8080", "Listen server port")
	flag.StringVar(&CF.UA_LIST, "ua_list", "ua", "list with user-agent strings")

	flag.Parse()

	logger.Printf("%+v\n", &CF)
}

func main() {
	initLogger()
	parseCli()
	loadIpList()
	testGates()
	uaList = loadLinesFromFile(CF.UA_LIST)
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

func getRandomUA() string {
	rand.Seed(time.Now().UnixNano())
	return uaList[rand.Intn(len(uaList))]
}

func getRandomGate() *Gate {
	rand.Seed(time.Now().UnixNano())
	return &gates[rand.Intn(len(gates))]
}
