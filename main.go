package main

import (
	"net/http"
)

func main() {

	server_mux := http.NewServeMux()
	server_mux.Handle("/app/", http.StripPrefix("/app/", http.FileServer(http.Dir("./files"))))
	server_mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	server := &http.Server{
		Addr:    ":8080",
		Handler: server_mux,
	}
	server.ListenAndServe()
}
