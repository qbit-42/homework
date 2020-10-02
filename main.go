package main

import (
	"github.com/carlescere/scheduler"
	"log"
)

type MinioContainer struct {
	ID        string
	IpAddress string
	Port      int
	AccessKey string
	SecretKey string
}

var minioContainers []MinioContainer

func main() {
	scheduler.Every(30).Seconds().Run(scanForMinioContainers)
	setupMux()
	log.Output(2, "Homework server started")
}
