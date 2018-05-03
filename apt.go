package gcs

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	storage "google.golang.org/api/storage/v1"
)

const (
	scope = storage.DevstorageReadOnlyScope
)

var (
	messageCodes = map[int]string{
		100: "Capabilities",
		102: "Status",
		200: "URI Start",
		201: "URI Done",
		400: "URI Failure",
		600: "URI Acquire",
		601: "Configuration",
	}
)

type AptMessage struct {
	Code    int
	Text    string
	Headers map[string][]string
}

func (a *AptMessage) Encode() string {
	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf("%v %v\n", a.Code, messageCodes[a.Code]))

	for key, valarr := range a.Headers {
		for _, val := range valarr {
			valueFix := strings.Replace(val, "%", "%%", -1)
			buf.WriteString(fmt.Sprintf("%v: %v\n", key, valueFix))
		}
	}

	buf.WriteRune('\n')

	return buf.String()
}

func (a *AptMessage) GetField(name string) string {
	return a.Headers[name][0]
}

func (a *AptMessage) GetFields(name string) []string {
	return a.Headers[name]
}

type Header struct {
	Name  string
	Value string
}

type AptMethod struct {
	Debugging   bool
	Credentials map[string]string
	Service     map[string]*storage.ObjectsService
	Input       *bufio.Reader
}

func (a *AptMethod) Send(code int, headers ...Header) {
	headersx := map[string][]string{}
	for _, header := range headers {
		headersx[header.Name] = append(headersx[header.Name], header.Value)
	}
	msg := AptMessage{Code: code, Headers: headersx}
	fmt.Fprintf(os.Stdout, msg.Encode())
}

func (a *AptMethod) SendCapabilities() {
	a.Send(100, Header{"Version", "1.0"}, Header{"Single-Instance", "true"}, Header{"Send-Config", "true"})
}

func (a *AptMethod) SendStatus(headers ...Header) {
	a.Send(102, headers...)
}

func (a *AptMethod) SendUriStart(headers ...Header) {
	a.Send(200, headers...)
}

func (a *AptMethod) SendUriDone(headers ...Header) {
	a.Send(201, headers...)
}

func (a *AptMethod) SendUriFailure(headers ...Header) {
	a.Send(400, headers...)
}

func (a *AptMethod) Run() (exitCode int) {
	a.Input = bufio.NewReader(os.Stdin)
	for {
		message := a.readMessage()
		if len(message.Headers) == 0 {
			return 0
		}
		if message.Code == 600 {
			a.ReadObject(message)
		} else if message.Code == 601 {
			a.ReadConfig(message)
		} else {
			return 100
		}

	}
}

func (a *AptMethod) Debug(s string) {
	if a.Debugging {
		fmt.Fprint(os.Stderr, s)
	}
}

func (a *AptMethod) InitService(bucket string) {
	var client *http.Client

	credFile, _ := a.Credentials[bucket]
	if credFile != "" {
		data, err := ioutil.ReadFile(credFile)
		if err != nil {
			log.Fatalf("Unable to read credentials file: %v", err)
		}

		conf, err := google.JWTConfigFromJSON(data, scope)
		if err != nil {
			log.Fatalf("Invalid credentials file %s: %v", credFile, err)
		}

		client = conf.Client(context.Background())
	} else {
		var err error
		client, err = google.DefaultClient(context.Background(), scope)
		if err != nil {
			log.Fatalf("Unable to get default client: %v", err)
		}
	}

	service, err := storage.New(client)
	if err != nil {
		log.Fatalf("Unable to create storage service: %v", err)
	}

	if a.Service == nil {
		a.Service = map[string]*storage.ObjectsService{}
	}
	a.Service[bucket] = storage.NewObjectsService(service)
	if err != nil {
		log.Fatalf("Unable to create objects storage service: %v", err)
	}
}

func (a *AptMethod) readMessage() (message AptMessage) {
	message = AptMessage{}
	line, err := a.Input.ReadString('\n')
	if err == io.EOF {
		return message
	}

	for line == "\n" {
		line, err = a.Input.ReadString('\n')
		if err == io.EOF {
			return message
		}
	}

	// read the last read non blank link
	s := strings.SplitN(line, " ", 2)
	message.Code, _ = strconv.Atoi(s[0])
	message.Text = strings.TrimSpace(s[1])
	message.Headers = map[string][]string{}

	for {
		line, err := a.Input.ReadString('\n')
		if err == io.EOF || line == "\n" {
			return message
		}

		s := strings.SplitN(line, ":", 2)
		message.Headers[s[0]] = append(message.Headers[s[0]], strings.TrimSpace(s[1]))
	}

	a.Debug(fmt.Sprintf("Got message: %v\n", message))

	return
}

func (a *AptMethod) ReadConfig(message AptMessage) {
	items := message.GetFields("Config-Item")
	credFiles := map[string]string{}
	for _, item := range items {
		if strings.HasPrefix(item, "Debug::Acquire::gcs") {
			val := strings.SplitN(item, "=", 2)[1]
			if val == "yes" || val == "true" || val == "with" ||
			    val == "on" || val == "enable" || val == "1" {
				a.Debugging = true
			} else if val == "no" || val == "false" || val == "without" ||
			    val == "off" || val == "disable" || val == "0" {
				a.Debugging = false
			}
		}
		if strings.HasPrefix(item, "Acquire::gcs::") {
			// "Acquire::gcs::<bucket>::CredentialsFile"
			kvarr := strings.SplitN(item, "=", 2)
			keyparts := strings.Split(kvarr[0], "::")[2:]
			if len(keyparts) == 2 && keyparts[1] == "CredentialsFile" {
				credFiles[keyparts[0]] = kvarr[1]
			}
		}
	}
	a.Credentials = credFiles
	a.Debug(fmt.Sprintf("Google Cloud Service credentials:\n%v\n", credFiles))
}

func (a *AptMethod) ReadObject(message AptMessage) {
	uri := message.GetField("URI")
	filename := message.GetField("Filename")

	a.SendStatus(Header{"URI", uri}, Header{"Message", "Waiting for headers"})

	splits := strings.SplitN(uri, "/", 4)
	bucket := splits[2]
	object := splits[3]

	if a.Service == nil || a.Service[bucket] == nil {
		a.InitService(bucket)
	}
	if resp, err := a.Service[bucket].Get(bucket, object).Download(); err == nil {
		defer resp.Body.Close()
		a.SendUriStart(
			Header{"URI",           uri},
			Header{"Size",          strconv.FormatInt(resp.ContentLength, 10)},
			Header{"Last-Modified", resp.Header.Get("last-modified")})
		body, _ := ioutil.ReadAll(resp.Body)
		ioutil.WriteFile(filename, body, 0644)
		md5sum := md5.Sum(body)
		sha1sum := sha1.Sum(body)
		sha256sum := sha256.Sum256(body)
		sha512sum := sha512.Sum512(body)
		a.SendUriDone(
			Header{"URI",           uri},
			Header{"Filename",      filename},
			Header{"Size",          strconv.FormatInt(resp.ContentLength, 10)},
			Header{"Last-Modified", resp.Header.Get("last-modified")},
			Header{"MD5-Hash",      fmt.Sprintf("%x", md5sum)},
			Header{"MD5Sum-Hash",   fmt.Sprintf("%x", md5sum)},
			Header{"SHA1-Hash",     fmt.Sprintf("%x", sha1sum)},
			Header{"SHA256-Hash",   fmt.Sprintf("%x", sha256sum)},
			Header{"SHA512-Hash",   fmt.Sprintf("%x", sha512sum)})
	} else {
		a.SendUriFailure(
			Header{"URI",        uri},
			Header{"Message",    fmt.Sprintf("%v", err)})
	}
}
