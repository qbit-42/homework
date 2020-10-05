package main

import (
	log "github.com/sirupsen/logrus"
)

type MinioContainer struct {
	ID        string
	IpAddress string
	Port      int
	AccessKey string
	SecretKey string
}

func main() {
	scanForMinioContainers()
	err := setupMux()
	if err != nil {
		log.Fatal("Server failed to start ", err)
	}
}
