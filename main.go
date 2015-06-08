package main

import (
	"crypto/md5"
	"encoding/hex"
	"io"
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

const TIMEOUT_TEST_URL = "http://api.ipify.org"

var logger log.Logger
var gates []Gate

type Gate struct {
	address	*url.URL
	timeout 	[]int
}

func (g Gate) testTimeout() {
	
} 

func loadIpList() {
	content, err := ioutil.ReadFile(IP_LIST_FILENAME)
	if err != nil {
		//Do something
	}
	gateList := strings.Split(string(content), "\n")
	logger.Printf("Loaded IP list with %d items", len(gateList))
	
	for _, address := range gateList {
		proxyUrl, _ := url.Parse(address)
//		client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyUrl)}}
http.DefaultTransport = &http.Transport{Proxy: http.ProxyURL(proxyUrl)}
//		gate := Gate{address: proxyUrl}
		resp, err := http.Get(TIMEOUT_TEST_URL)
		
		if err != nil {
			logger.Println(err, "ERROR",address)
		}else{
			robots, _ := ioutil.ReadAll(resp.Body)
			logger.Printf("%s\n", string(robots))
			
		}
		
	} 
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

	addr := getRandomIp()

	h := md5.New()
	io.WriteString(h, r.URL.String())
	io.WriteString(h, addr)
	reqId := hex.EncodeToString(h.Sum(nil))

	proxyUrl, _ := url.Parse("http://" + addr)
//	http.DefaultTransport = &http.Transport{Proxy: http.ProxyURL(proxyUrl)}
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyUrl)}}

	logger.Printf("[%s] Passing request for %s through gate %s", string(reqId), r.URL.String(), addr)

	resp, _ := client.Get(r.URL.String())

	logger.Printf("[%s] Received response for %s from gate %s", reqId, r.URL.String(), addr)

	robots, _ := ioutil.ReadAll(resp.Body)
	w.Write(robots)
}

func main() {
	initLogger()
	loadIpList()	
//	createServer()
}

func getRandomIp() string {
	rand.Seed(time.Now().UnixNano())
	return gates[rand.Intn(len(gates))].address.String()
}
