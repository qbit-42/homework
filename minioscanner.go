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
	"time"
)

var bucketName = "files"

type MinioClient interface {
	GetObject(objectId string) (io.Reader, error)
	PutObject(objectId string, reader io.Reader, contentLength int64) (int64, error)
	GetAllIds() ([]string, error)
	EnsureBucketExists() error
	GetName() string
	IsListable() bool
}

type DockerMinioClient struct {
	actualClient minio.Client
	bucketName   string
	name         string
	listable     string
}

func (c DockerMinioClient) String() string {
	return c.name
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
	if err != nil {
		return -1, err
	}
	return uploadInfo.Size, err
}

func (c DockerMinioClient) GetAllIds() ([]string, error) {
	var ids []string
	for obj := range c.actualClient.ListObjects(context.Background(), bucketName, minio.ListObjectsOptions{}) {
		if obj.Err != nil {
			return nil, obj.Err
		}
		ids = append(ids, obj.Key)
	}
	return ids, nil
}

func (c DockerMinioClient) IsListable() bool {
	return strings.Compare(c.listable, "true") == 0
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
	time.Sleep(2 * time.Second)
	log.Info("Scanning for Minio containers")
	cli, err := client.NewEnvClient()
	if err != nil {
		panic(err) //if we can't create client then something is really wrong
	}
	containers, err := listMinioContainers(cli)
	if err != nil {
		panic(err) //if we can't create client then something is really wrong
	}
	collectContainersData(containers, cli)
	log.Info("Prepared Minio clients: ", minioClients)
}

func listMinioContainers(cli *client.Client) ([]types.Container, error) {
	containerListFilters := filters.NewArgs()
	containerListFilters.Add("name", "amazin")
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{Filters: containerListFilters})
	if err != nil || len(containers) < 1 {
		return nil, errors.New("Cannot find any Minio containers under local docker daemon ")
	}
	return containers, nil
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
			Listable:  decodeEnvVariable(containerJson.Config.Env, "LISTABLE"),
		}

		minioClient, err := prepareClient(container)
		if err != nil {
			log.Error("Cannot configure client for ", container.ID)
			continue
		}
		err = minioClient.EnsureBucketExists()
		if err != nil {
			log.Error("Cannot create/check bucket for ", minioClient.name)
			continue
		}
		minioClients = append(minioClients, minioClient)
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
	if err != nil {
		log.Error("Cannot prepare client for ", endpoint, err)
		return DockerMinioClient{}, err
	}
	newClient := DockerMinioClient{
		actualClient: *minioClient,
		name:         endpoint,
		listable:     container.Listable,
	}
	return newClient, err
}

func decodeEnvVariable(env []string, name string) string {
	value := "no_value"
	for _, line := range env {
		if strings.HasPrefix(line, name) {
			value = line[strings.LastIndex(line, "=")+1:]
			break
		}
	}
	log.Info("Env var ", name, "=", value)
	return value
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
