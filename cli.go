package main

import "flag"

type CLI struct {
	IP_LIST_FILENAME  string
	PORT              string
	GATE_TEST_TIMEOUT int
	GATE_REQ_TIMEOUT  int
	MAX_RETRIES       int
	SIM_REQ           int
	UA_LIST           string
	HTTPS             string
	EXPLORE_BIAS      float64
}

var CF CLI

func parseCli() {
	flag.StringVar(&CF.IP_LIST_FILENAME, "ip_list_file", "ip_list", "Filename with ip gateways")
	flag.IntVar(&CF.GATE_TEST_TIMEOUT, "gate_test_timeout", 10, "Maximum time (in seconds) to wait for response from gate when testing")
	flag.IntVar(&CF.GATE_REQ_TIMEOUT, "gate_req_timeout", 10, "Maximum time (in seconds) to wait for response from gate")
	flag.IntVar(&CF.MAX_RETRIES, "max_retries", 10, "Maximum number of retries for one request")
	flag.StringVar(&CF.PORT, "port", "8080", "Listen server port")
	flag.StringVar(&CF.UA_LIST, "ua_list", "ua", "list with user-agent strings")
	flag.IntVar(&CF.SIM_REQ, "sim_req", 1, "How many same requests should be fired to maximize the chance for response")
	flag.StringVar(&CF.HTTPS, "https", "", "Port number for HTTPS server. Disabled if empty")
	flag.Float64Var(&CF.EXPLORE_BIAS, "ex_bias", 0.8, "Bias for exploring gates")

	flag.Parse()

	logger.Printf("%+v\n", &CF)
}
