package main

import (
	"context"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/minio/minio-go/v7"
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
	id, client := prepareMinioRequest(w, r)
	err := writeFile(client, id, r.Body, r.ContentLength)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "File %s could not be written", id)
	}
}

func GetHandler(w http.ResponseWriter, r *http.Request) {
	id, client := prepareMinioRequest(w, r)
	log.Info("Handling read request for file ", id, " onto container ", client.EndpointURL())
	err := readFile(client, id, w)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "File %s could not be read", id)
	}
}

func readFile(minioClient minio.Client, objectId string, writer io.Writer) error {
	ctx := context.Background()
	reader, err := minioClient.GetObject(ctx, bucketName, objectId, minio.GetObjectOptions{})
	if err != nil {
		log.Error(err)
		return err
	}
	bytesRead, err := io.Copy(writer, reader)
	if err != nil {
		log.Error("Error reading file ", objectId)
		return err
	} else {
		log.Info("Successfully read file: id ", objectId, ", size: ", bytesRead)
	}
	return nil
}

func prepareMinioRequest(w http.ResponseWriter, r *http.Request) (string, minio.Client) {
	vars := mux.Vars(r)
	id := vars["id"]
	client, error := getInstance(id)
	if error != nil {
		w.WriteHeader(http.StatusExpectationFailed)
		fmt.Fprintf(w, error.Error())
	}
	return id, client
}

func writeFile(minioClient minio.Client, objectId string, reader io.Reader, contentLength int64) error {
	ctx := context.Background()
	contentType := "application/text"

	n, err := minioClient.PutObject(ctx, bucketName, objectId, reader, contentLength, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		log.Error(err)
		return err
	}

	log.Info("Successfully uploaded file ", objectId, "of size ", n.Size)
	return nil
}
