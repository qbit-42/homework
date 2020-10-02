package main

import (
	"errors"
	"github.com/docker/distribution/context"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
	"hash/fnv"
	"strings"
)

func scanForMinioContainers() {
	log.Debug("Scanning for Minio containers")
	cli, err := client.NewEnvClient()
	if err != nil {
		panic(err) //if we can't create client then something is really wrong
	}
	containers := listMinioContainers(err, cli)
	collectContainersData(containers, cli)
	log.Debug("Found Minio containers: ", minioContainers)
}

func collectContainersData(containers []types.Container, cli *client.Client) {
	for _, container := range containers {
		containerJson, err := cli.ContainerInspect(context.Background(), container.ID)
		if err != nil {
			log.Error("Cannot inspect container ", container.ID)
			continue
		}

		minioContainers = append(minioContainers, MinioContainer{
			ID:        container.ID,
			IpAddress: container.NetworkSettings.Networks[containerJson.HostConfig.NetworkMode.NetworkName()].IPAddress,
			Port:      9000,
			AccessKey: decodeEnvVariable(containerJson.Config.Env, "MINIO_ACCESS_KEY"),
			SecretKey: decodeEnvVariable(containerJson.Config.Env, "MINIO_SECRET_KEY"),
		})
	}
}

func listMinioContainers(err error, cli *client.Client) []types.Container {
	containerListFilters := filters.NewArgs()
	containerListFilters.Add("name", "amazin")
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{Filters: containerListFilters})
	minioContainers = []MinioContainer{}
	if err != nil || len(containers) < 1 {
		log.Error("Cannot find any Minio containers under local docker daemon ", err)
	}
	return containers
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

func getInstance(objectId string) (MinioContainer, error) {
	var container MinioContainer // to return null in case o error (?)
	if len(minioContainers) < 1 {
		log.Error("No Minio containers registered")
		return container, errors.New("No Minio containers registered")
	}
	var idx = hashId(objectId) % len(minioContainers)
	return minioContainers[idx], nil
}
