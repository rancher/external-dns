package main

import (
	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"net/http"
	"sync"
)

var (
	router         = mux.NewRouter()
	healtcheckPort = ":1000"

	providerErrored bool
	stateLock       sync.Mutex
)

func checkProviderError(err error) {
	stateLock.Lock()
	if err != nil {
		providerErrored = true
	} else {
		providerErrored = false
	}
	stateLock.Unlock()
}

func startHealthcheck() {
	router.HandleFunc("/", healtcheck).Methods("GET", "HEAD").Name("Healthcheck")
	logrus.Info("Healthcheck handler is listening on ", healtcheckPort)
	logrus.Fatal(http.ListenAndServe(healtcheckPort, router))
}

func healtcheck(w http.ResponseWriter, req *http.Request) {
	// 1) test metadata server
	_, err := m.MetadataClient.GetSelfStack()
	if err != nil {
		logrus.Error("Healtcheck failed: unable to reach metadata")
		http.Error(w, "Failed to reach metadata server", http.StatusInternalServerError)
	} else {
		// 2) check last error value from provider method call
		stateLock.Lock()
		errored := providerErrored
		stateLock.Unlock()
		if errored {
			logrus.Error("Healtcheck failed: last call to provider failed")
			http.Error(w, "Last call to provider failed", http.StatusInternalServerError)
		} else {
			err := c.TestConnect()
			if err != nil {
				logrus.Error("Healtcheck failed: unable to reach Cattle")
				http.Error(w, "Failed to connect to Cattle ", http.StatusInternalServerError)
			}
			w.Write([]byte("OK"))
		}
	}
}
