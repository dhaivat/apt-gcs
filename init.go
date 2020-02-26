package gcs

import (
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	storage "google.golang.org/api/storage/v1"
)

const (
	scope                  = storage.DevstorageReadOnlyScope
	accessTokenPath        = "/etc/apt/gcs_access_token"
	serviceAccountJSONPath = "/etc/apt/gcs_sa_json"
)

var (
	client   *context.Context
	service  *storage.Service
	oService *storage.ObjectsService
)

var ctx context.Context = context.Background()

// InitConfig creates the google storage client that is used from the apt package
func InitConfig() {
	client, err := getClient()
	if err != nil {
		log.Fatalf("Unable to get client: %v", err)
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

// getClient returns an authenticated http client based on a different set of GCP
// auth methods, cascading in the following order:
// if access_token (bearer) is present in /etc/apt/gcs_access_token use it,
// else if Service Account JSON key is present in /etc/apt/gcs_sa_json use it,
// else try to get Application Default credentials https://github.com/golang/oauth2/blob/master/google/default.go

func getClient() (client *http.Client, err error) {
	switch {
	case fileExists(accessTokenPath):
		client, err = clientFromAccessToken(accessTokenPath)
		if err != nil {
			log.Fatalf("Unable to get client: %v", err)
		}
	case fileExists(serviceAccountJSONPath):
		client, err = clientFromServiceAccount(serviceAccountJSONPath)
		if err != nil {
			log.Fatalf("Unable to get client: %v", err)
		}
	default:
		client, err = google.DefaultClient(ctx, scope)
		if err != nil {
			log.Fatalf("Unable to get client: %v", err)
		}
	}
	return client, err
}

// clientFromAccessToken creates an http client authenticated using a GCS access_token (gcloud auth print-access-token)
func clientFromAccessToken(accessTokenPath string) (client *http.Client, err error) {
	tokenBytes, err := ioutil.ReadFile(accessTokenPath)
	if err != nil {
		log.Fatalf("Error while reading access_token file: %v", err)
	}
	token := oauth2.Token{
		AccessToken: string(tokenBytes),
	}
	tokenSource := oauth2.StaticTokenSource(&token)
	return oauth2.NewClient(ctx, tokenSource), err
}

// clientFromServiceAccount creates an http client authenticated using a GCS Service account JSON key
func clientFromServiceAccount(serviceAccountJSONPath string) (client *http.Client, err error) {
	JSONBytes, err := ioutil.ReadFile(serviceAccountJSONPath)
	if err != nil {
		log.Fatalf("Error while reading SA json file: %v", err)
	}
	credentials, err := google.CredentialsFromJSON(ctx, JSONBytes, scope)
	tokenSource := credentials.TokenSource
	return oauth2.NewClient(ctx, tokenSource), err
}

// fileExists checks if a file exists
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
