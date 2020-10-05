package main

import (
	"context"
	"errors"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	log "github.com/sirupsen/logrus"
	"hash/fnv"
	"io"
	"strconv"
	"strings"
)

var bucketName = "files"

type MinioClient interface {
	GetObject(objectId string) (io.Reader, error)
	PutObject(objectId string, reader io.Reader, contentLength int64) (int64, error)
	EnsureBucketExists() error
	GetName() string
}

type DockerMinioClient struct {
	actualClient minio.Client
	bucketName   string
	name         string
}

func (c DockerMinioClient) GetName() string {
	return c.name
}

func (c DockerMinioClient) GetObject(objectId string) (io.Reader, error) {
	return c.actualClient.GetObject(context.Background(), bucketName, objectId, minio.GetObjectOptions{})
}

func (c DockerMinioClient) PutObject(objectId string, reader io.Reader, contentLength int64) (int64, error) {
	uploadInfo, err := c.actualClient.PutObject(context.Background(),
		bucketName,
		objectId,
		reader,
		contentLength,
		minio.PutObjectOptions{ContentType: "application/text"})
	return uploadInfo.Size, err
}

func (c DockerMinioClient) EnsureBucketExists() error {
	ctx := context.Background()
	log.Info("Ensuring bucket exists: ", bucketName)
	err := c.actualClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
	if err != nil {
		// Check to see if we already own this bucket (which happens if you run this twice)
		exists, errBucketExists := c.actualClient.BucketExists(ctx, bucketName)
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

//I know it was supposed to be stateless
//but re-generating and re-configuring clients
//on every request seemed _very_ sub-optimal
var minioClients []MinioClient

//TODO: harder problem of re-balancing existing files
// when number of available backends changes
func scanForMinioContainers() {
	log.Info("Scanning for Minio containers")
	cli, err := client.NewEnvClient()
	if err != nil {
		panic(err) //if we can't create client then something is really wrong
	}
	containers := listMinioContainers(err, cli)
	collectContainersData(containers, cli)
	log.Info("Prepared Minio clients: ", minioClients)
}

func listMinioContainers(err error, cli *client.Client) []types.Container {
	containerListFilters := filters.NewArgs()
	containerListFilters.Add("name", "amazin")
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{Filters: containerListFilters})
	if err != nil || len(containers) < 1 {
		log.Error("Cannot find any Minio containers under local docker daemon ", err)
	}
	return containers
}

func collectContainersData(containers []types.Container, cli *client.Client) {
	for _, container := range containers {
		containerJson, err := cli.ContainerInspect(context.Background(), container.ID)
		if err != nil {
			log.Error("Cannot inspect container ", container.ID)
			continue
		}

		container := MinioContainer{
			ID:        container.ID,
			IpAddress: container.NetworkSettings.Networks[containerJson.HostConfig.NetworkMode.NetworkName()].IPAddress,
			Port:      9000,
			AccessKey: decodeEnvVariable(containerJson.Config.Env, "MINIO_ACCESS_KEY"),
			SecretKey: decodeEnvVariable(containerJson.Config.Env, "MINIO_SECRET_KEY"),
		}

		minioClient, err := prepareClient(container)
		if err != nil {
			log.Error("Cannot configure client for ", container.ID)
		}
		err = minioClient.EnsureBucketExists()
		if err != nil {
			log.Error("Cannot create/check bucket for ", minioClient.name)
		} else {
			minioClients = append(minioClients, minioClient)
		}
	}
}

func prepareClient(container MinioContainer) (DockerMinioClient, error) {
	endpoint := container.IpAddress + ":" + strconv.FormatInt(int64(container.Port), 10)
	accessKeyID := container.AccessKey
	secretAccessKey := container.SecretKey
	// Initialize minio client object.
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: false,
	})
	newClient := DockerMinioClient{
		actualClient: *minioClient,
		name:         endpoint,
	}
	if err != nil {
		log.Error(err)
		return newClient, err
	}

	return newClient, err
}

func decodeEnvVariable(env []string, name string) string {
	for _, line := range env {
		if strings.HasPrefix(line, name) {
			return line[strings.LastIndex(line, "=")+1:]
		}
	}
	return "no_value"
}

func hashId(id string) int {
	hashCalc := fnv.New32a()
	_, err := hashCalc.Write([]byte(id))
	if err != nil {
		return -1
	}
	return int(hashCalc.Sum32())
}

func getInstance(objectId string) (MinioClient, error) {
	if len(minioClients) < 1 {
		log.Error("No Minio containers registered")
		return nil, errors.New("no Minio containers registered")
	}
	var idx = hashId(objectId) % len(minioClients)
	return minioClients[idx], nil
}
