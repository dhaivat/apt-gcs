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
	"os"
	"strconv"
	"strings"
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
	Headers map[string]string
}

func (a *AptMessage) Encode() string {
	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf("%v %v\n", a.Code, messageCodes[a.Code]))

	for key, value := range a.Headers {
		valueFix := strings.Replace(value, "%", "%%", -1)
		buf.WriteString(fmt.Sprintf("%v: %v\n", key, valueFix))
	}

	buf.WriteRune('\n')

	return buf.String()
}

type AptMethod struct {
}

func (a *AptMethod) Send(code int, headers map[string]string) {
	msg := AptMessage{Code: code, Headers: headers}
	fmt.Fprintf(os.Stdout, msg.Encode())
}

func (a *AptMethod) SendCapabilities() {
	a.Send(100, map[string]string{"Version": "1.0", "Single-Instance": "true"})
}

func (a *AptMethod) SendStatus(headers map[string]string) {
	a.Send(102, headers)
}

func (a *AptMethod) SendUriStart(headers map[string]string) {
	a.Send(200, headers)
}

func (a *AptMethod) SendUriDone(headers map[string]string) {
	a.Send(201, headers)
}

func (a *AptMethod) SendUriFailure(headers map[string]string) {
	a.Send(400, headers)
}

func (a *AptMethod) Run() (exitCode int) {
	for {
		message := a.readMessage()
		if len(message) == 0 {
			return 0
		}
		if message["_number"] == "600" {
			a.ReadObejct(message)
		} else {
			return 100
		}

	}
}

func (a *AptMethod) readMessage() (message map[string]string) {
	message = map[string]string{}
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err == io.EOF {
		return message
	}

	for line == "\n" {
		line, err = reader.ReadString('\n')
		if err == io.EOF {
			return message
		}
	}

	// read the last read non blank link
	s := strings.SplitN(line, " ", 2)
	message["_number"] = s[0]
	message["_text"] = strings.TrimSpace(s[1])

	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF || line == "\n" {
			return message
		}

		s := strings.SplitN(line, ":", 2)
		message[s[0]] = strings.TrimSpace(s[1])
	}
}

func (a *AptMethod) ReadObejct(message map[string]string) {
	uri := message["URI"]
	filename := message["Filename"]

	a.SendStatus(map[string]string{"URI": uri, "Message": "Waiting for headers"})

	splits := strings.SplitN(uri, "/", 4)
	bucket := splits[2]
	object := splits[3]

	if resp, err := oService.Get(bucket, object).Download(); err == nil {
		defer resp.Body.Close()
		a.SendUriStart(map[string]string{
			"URI":           uri,
			"Size":          strconv.FormatInt(resp.ContentLength, 10),
			"Last-Modified": resp.Header.Get("last-modified")})
		body, _ := ioutil.ReadAll(resp.Body)
		ioutil.WriteFile(filename, body, 0644)
		md5sum := md5.Sum(body)
		sha1sum := sha1.Sum(body)
		sha256sum := sha256.Sum256(body)
		sha512sum := sha512.Sum512(body)
		a.SendUriDone(map[string]string{
			"URI":           uri,
			"Filename":      filename,
			"Size":          strconv.FormatInt(resp.ContentLength, 10),
			"Last-Modified": resp.Header.Get("last-modified"),
			"MD5-Hash":      fmt.Sprintf("%x", md5sum),
			"MD5Sum-Hash":   fmt.Sprintf("%x", md5sum),
			"SHA1-Hash":     fmt.Sprintf("%x", sha1sum),
			"SHA256-Hash":   fmt.Sprintf("%x", sha256sum),
			"SHA512-Hash":   fmt.Sprintf("%x", sha512sum),
		})
	} else {
		a.SendUriFailure(map[string]string{
			"URI":        uri,
			"Message":    err.Error(),
			"FailReason": "really silly failure",
		})
	}
}
