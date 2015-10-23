package main

import (
	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"net/http"
)

var (
	router         = mux.NewRouter()
	healtcheckPort = ":1000"
)

func startHealthcheck() {
	router.HandleFunc("/", healtcheck).Methods("GET", "HEAD").Name("Healthcheck")
	logrus.Info("Healthcheck handler is listening on ", healtcheckPort)
	logrus.Fatal(http.ListenAndServe(healtcheckPort, router))
}

func healtcheck(w http.ResponseWriter, req *http.Request) {
	// 1) test metadata server
	_, err := m.GetSelfStack()
	if err != nil {
		logrus.Error("Healtcheck failed: unable to reach metadata")
		http.Error(w, "Failed to reach metadata server", http.StatusInternalServerError)
	} else {
		// 2) test provider
		_, err := provider.GetRecords()
		if err != nil {
			logrus.Error("Healtcheck failed: unable to reach a provider")
			http.Error(w, "Failed to reach an external provider ", http.StatusInternalServerError)
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
