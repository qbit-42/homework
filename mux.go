package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"io"
	"log"
	"net/http"
)

var bucketName string = "files"

func setupMux() {
	router := mux.NewRouter()
	router.HandleFunc("/object/{id}", GetHandler).Methods("GET")
	router.HandleFunc("/object/{id}", PutHandler).Methods("PUT")
	http.Handle("/", router)
	http.ListenAndServe(":3000", router)
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
	err := readFile(container, id, w)
	if err != nil {
		w.WriteHeader(http.StatusExpectationFailed)
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

func readFile(container MinioContainer, objectId string, writer io.Writer) error {
	ctx := context.Background()
	minioClient, err := prepareMinioClient(container, ctx)

	reader, err := minioClient.GetObject(ctx, bucketName, objectId, minio.GetObjectOptions{})
	if err != nil {
		log.Fatalln(err)
		return err
	}

	written, err := io.Copy(writer, reader)
	if err != nil {
		log.Printf("Successfully read %s , %s bytes\n", objectId, written)
	} else {
		log.Fatalln("Error reading file %s", objectId)
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
		log.Fatalln(err)
		return "Error uploading file", err
	}

	log.Printf("Successfully uploaded %s of size %d\n", objectId, n)
	return "Successfully written file " + objectId, nil
}

func prepareMinioClient(container MinioContainer, ctx context.Context) (*minio.Client, error) {
	endpoint := container.IpAddress //+ ":" + strconv.FormatInt(int64(container.Port), 10)
	accessKeyID := container.AccessKey
	secretAccessKey := container.SecretKey
	useSSL := true

	// Initialize minio client object.
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		log.Fatalln(err)
	}

	ensureBucketExists(err, minioClient, ctx)
	return minioClient, err
}

func ensureBucketExists(err error, minioClient *minio.Client, ctx context.Context) {

	location := "us-east-1"

	err = minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{Region: location})
	if err != nil {
		// Check to see if we already own this bucket (which happens if you run this twice)
		exists, errBucketExists := minioClient.BucketExists(ctx, bucketName)
		if errBucketExists == nil && exists {
			log.Printf("We already own %s\n", bucketName)
		} else {
			log.Fatalln(err)
		}
	} else {
		log.Printf("Successfully created %s\n", bucketName)
	}
}
