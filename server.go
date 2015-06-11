package main

import "net/http"

type ReqHandler struct {
}

func (m *ReqHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	activeRequests++
	handle(w, r, 1)
	activeRequests--
}

func createHttpServer() {
	logger.Printf("Server listening on port %s", CF.PORT)
	err := http.ListenAndServe(":"+CF.PORT, &ReqHandler{})

	if err != nil {
		logger.Fatal(err)
	}
}

func createHttpsServer() {
	logger.Printf("Https Server listening on port 8443")
	err := http.ListenAndServeTLS(":8443", "server.crt", "server.key", &ReqHandler{})

	if err != nil {
		logger.Fatal(err)
	}
}
