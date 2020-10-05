package main

import (
	"fmt"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
)

func setupMux() error {
	router := mux.NewRouter()
	router.HandleFunc("/object/{id}", GetHandler).Methods("GET")
	router.HandleFunc("/object/{id}", PutHandler).Methods("PUT")
	http.Handle("/", router)
	return http.ListenAndServe(":3000", router)
}

func PutHandler(w http.ResponseWriter, r *http.Request) {
	id := getIdFromRequest(r)
	client, err := prepareMinioRequest(id)
	if err != nil {
		writeErrorToResponse(w, err, "File %s could not be read", id)
		return
	}
	log.Info("Handling write request for file ", id, " onto container ", client.GetName())
	err = writeFile(client, id, r.Body, r.ContentLength)
	writeErrorToResponse(w, err, "File %s could not be written", id)
}

func GetHandler(w http.ResponseWriter, r *http.Request) {
	id := getIdFromRequest(r)
	client, err := prepareMinioRequest(id)
	if err != nil {
		writeErrorToResponse(w, err, "File %s could not be read", id)
		return
	}
	log.Info("Handling read request for file ", id, " onto container ", client.GetName())
	err = readFile(client, id, w)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		writeErrorToResponse(w, err, "File %s could not be read", id)
	}
}

func writeErrorToResponse(w http.ResponseWriter, err error, message string, id string) {
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, err = fmt.Fprintf(w, message, id)
		if err != nil {
			log.Error("Failed to write response body for file ", id, ", error: ", err)
		}
	}
}

func readFile(minioClient MinioClient, objectId string, writer io.Writer) error {
	reader, err := minioClient.GetObject(objectId)
	if err != nil {
		log.Error(err)
		return err
	}
	bytesRead, err := io.Copy(writer, reader)
	if err != nil {
		log.Error("Error reading file ", objectId)
		return err
	}
	log.Info("Successfully read file: id ", objectId, ", size: ", bytesRead)
	return nil
}

func writeFile(minioClient MinioClient, objectId string, reader io.Reader, contentLength int64) error {
	n, err := minioClient.PutObject(
		objectId,
		reader,
		contentLength)
	if err != nil {
		log.Error(err)
		return err
	}
	log.Info("Successfully uploaded file ", objectId, " of size ", n)
	return nil
}

func getIdFromRequest(r *http.Request) string {
	vars := mux.Vars(r)
	return vars["id"]
}

func prepareMinioRequest(id string) (MinioClient, error) {
	client, err := getInstance(id)
	if err != nil {
		return nil, err
	}
	return client, nil
}
