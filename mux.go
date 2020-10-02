package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"strconv"
)

var bucketName string = "files"

func setupMux() error {
	router := mux.NewRouter()
	router.HandleFunc("/object/{id}", GetHandler).Methods("GET")
	router.HandleFunc("/object/{id}", PutHandler).Methods("PUT")
	http.Handle("/", router)
	return http.ListenAndServe(":3000", router)
}

func PutHandler(w http.ResponseWriter, r *http.Request) {
	id, container := prepareMinioRequest(w, r)

	buf := new(bytes.Buffer)
	buf.ReadFrom(r.Body)
	content := buf.String()
	output, err := writeFile(container, id, content)

	handleErrors(w, err, output)
}

func GetHandler(w http.ResponseWriter, r *http.Request) {
	id, container := prepareMinioRequest(w, r)
	log.Info("Handling read request for file ", id, " onto container ", container.IpAddress)
	err := readFile(container, id, w)
	if err != nil {
		w.WriteHeader(http.StatusExpectationFailed)
	}
}

func readFile(container MinioContainer, objectId string, writer io.Writer) error {
	ctx := context.Background()
	minioClient, err := prepareMinioClient(container, ctx)
	if err != nil {
		log.Error(err)
		return err
	}
	reader, err := minioClient.GetObject(ctx, bucketName, objectId, minio.GetObjectOptions{})
	if err != nil {
		log.Error(err)
		return err
	}
	//buf := new(strings.Builder)
	//written, err := io.Copy(buf, reader)
	//// check errors
	//log.Info(buf.String())
	//
	//fmt.Fprintf(writer, buf.String())
	written, err := io.Copy(writer, reader)
	if err != nil {
		log.Error("Error reading file ", objectId)
	} else {
		log.Info("Successfully read file: id ", objectId, ", size: ", written)
		return err
	}
	return nil
}

func handleErrors(w http.ResponseWriter, err error, output string) {
	if err != nil {
		w.WriteHeader(http.StatusExpectationFailed)
		fmt.Fprintf(w, output)
	} else {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, output)
	}
}

func prepareMinioRequest(w http.ResponseWriter, r *http.Request) (string, MinioContainer) {
	vars := mux.Vars(r)
	id := vars["id"]
	container, error := getInstance(id)
	if error != nil {
		w.WriteHeader(http.StatusExpectationFailed)
		fmt.Fprintf(w, error.Error())
	}
	return id, container
}

func writeFile(container MinioContainer, objectId string, content string) (string, error) {
	ctx := context.Background()
	minioClient, err := prepareMinioClient(container, ctx)

	contentType := "application/text"

	reader := bytes.NewReader([]byte(content))
	n, err := minioClient.PutObject(ctx, bucketName, objectId, reader, int64(len(content)), minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		log.Error(err)
		return "Error uploading file", err
	}

	log.Info("Successfully uploaded file ", objectId, "of size ", n.Size)
	return "Successfully written file " + objectId, nil
}

func prepareMinioClient(container MinioContainer, ctx context.Context) (*minio.Client, error) {
	endpoint := container.IpAddress + ":" + strconv.FormatInt(int64(container.Port), 10)
	accessKeyID := container.AccessKey
	secretAccessKey := container.SecretKey
	useSSL := false

	// Initialize minio client object.
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		log.Error(err)
		return minioClient, err
	}

	err = ensureBucketExists(minioClient, ctx)
	return minioClient, err
}

func ensureBucketExists(minioClient *minio.Client, ctx context.Context) error {
	log.Info("Ensuring bucket exists: ", bucketName)
	err := minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
	if err != nil {
		// Check to see if we already own this bucket (which happens if you run this twice)
		exists, errBucketExists := minioClient.BucketExists(ctx, bucketName)
		if errBucketExists == nil && exists {
			log.Info("We already own bucket ", bucketName)
		} else {
			log.Error(err)
			return err
		}
	} else {
		log.Info("Successfully created bucket ", bucketName)
	}
	return nil
}
