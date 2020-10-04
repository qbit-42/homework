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
	"strconv"
	"strings"
)

var bucketName string = "files"
var minioClients []minio.Client

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

		client, err := prepareClient(container)
		if err != nil {
			log.Error("Cannot configure client for ", container.ID)
		}
		err = ensureBucketExists(client)
		if err != nil {
			log.Error("Cannot create/check bucket for ", client.EndpointURL())
		}
		minioClients = append(minioClients, client)
	}
}

func prepareClient(container MinioContainer) (minio.Client, error) {
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
		return *minioClient, err
	}
	return *minioClient, err
}

func ensureBucketExists(minioClient minio.Client) error {
	ctx := context.Background()
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

func decodeEnvVariable(env []string, name string) string {
	for _, line := range env {
		if strings.HasPrefix(line, name) {
			return line[strings.LastIndex(line, "=")+1 : len(line)]
		}
	}
	return "no_value"
}

func hashId(id string) int {
	hasher := fnv.New32a()
	hasher.Write([]byte(id))
	return int(hasher.Sum32())
}

func getInstance(objectId string) (minio.Client, error) {
	var client minio.Client // to return null in case o error (?)
	if len(minioClients) < 1 {
		log.Error("No Minio containers registered")
		return client, errors.New("no Minio containers registered")
	}
	var idx = hashId(objectId) % len(minioClients)
	return minioClients[idx], nil
}
