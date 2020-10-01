package main

import (
	"fmt"
	"github.com/gorilla/mux"
	"net/http"
	//"github.com/minio/minio-go/v7"
	//"github.com/minio/minio-go/v7/pkg/credentials"
)

func main() {
	router := mux.NewRouter()
	router.HandleFunc("/object/{id}", GetHandler).Methods("GET")
	//router.HandleFunc("/object/{id}", PutHandler).Methods("PUT")
	http.Handle("/", router)
	http.ListenAndServe(":3000", router)
}

func GetHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "ID: %v\n", id)
}

func PutHandler(w http.ResponseWriter, r *http.Request) {

}
