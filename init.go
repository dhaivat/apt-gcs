package gcs

import (
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	storage "google.golang.org/api/storage/v1"
	"log"
)

const (
	scope = storage.DevstorageReadOnlyScope
)

var (
	client   *context.Context
	service  *storage.Service
	oService *storage.ObjectsService
)

func InitConfig() {

	client, err := google.DefaultClient(context.Background(), scope)
	if err != nil {
		log.Fatalf("Unable to get default client: %v", err)
	}

	service, err = storage.New(client)
	if err != nil {
		log.Fatalf("Unable to create storage service: %v", err)
	}

	oService = storage.NewObjectsService(service)
	if err != nil {
		log.Fatalf("Unable to create objects storage service: %v", err)
	}

}
