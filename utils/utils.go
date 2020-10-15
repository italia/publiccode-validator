package utils

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	vcsurl "github.com/alranel/go-vcsurl"
	"github.com/ghodss/yaml"
	"github.com/gorilla/mux"
	"github.com/italia/publiccode-parser-go"
	log "github.com/sirupsen/logrus"
	yamlv2 "gopkg.in/yaml.v2"
)

//Message json type mapping, for test purpose
type Message struct {
	Status          int                    `json:"status"`
	Message         string                 `json:"message"`
	Publiccode      *publiccode.PublicCode `json:"pc,omitempty"`
	Error           string                 `json:"error,omitempty"`
	ValidationError []ErrorInvalidValue    `json:"validationErrors,omitempty"`
}

// App application main settings and export for tests
type App struct {
	Router         *mux.Router
	Port           string
	Debug          bool
	DisableNetwork bool
}

// GetURLFromYMLBuffer returns a valid URL string based on input object
// takes valid URL as input
func GetURLFromYMLBuffer(in []byte) (*url.URL, error) {
	var s map[interface{}]interface{}
	yamlv2.NewDecoder(bytes.NewReader(in)).Decode(&s)
	urlString := fmt.Sprintf("%v", s["url"])
	url, err := url.Parse(urlString)
	if err == nil && url.Scheme != "" && url.Host != "" {
		return url, nil
	}

	return nil, fmt.Errorf("mapping to url ko: %v", err)
}

// GetRawURL returns a valid raw root repository based on
// major code hosting platforms
func GetRawURL(url *url.URL) string {
	rawURL := vcsurl.GetRawRoot(url)
	if rawURL != nil {
		return rawURL.String()
	}
	return ""
}

// GetRawFile returns a valid raw file for
// major code hosting platforms
func GetRawFile(urlString string) (string, error) {
	url, err := url.Parse(urlString)
	if err != nil {
		return "", err
	}
	rawURL := vcsurl.GetRawFile(url)
	if rawURL == nil {
		return "", fmt.Errorf("%s", "URL is not valid")
	}
	resp, err := http.Get(rawURL.String())
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return rawURL.String(), nil
	}
	return "", errors.New("URL is not valid or response http code is not success either")
}

// ErrorsToSlice return a slice of errors
func ErrorsToSlice(errs error) (arr []ErrorInvalidValue) {
	keys := strings.Split(errs.Error(), "\n")
	for _, key := range keys {
		arr = append(arr, ErrorInvalidValue{
			Key: key,
		})
	}
	return
}

// Yaml2json yaml to json conversion
func Yaml2json(y []byte) []byte {
	r, err := yaml.YAMLToJSON(y)
	if err != nil {
		log.Errorf("Conversion to json ko:\n%v\n", err)
	}
	return r
}

// SetupResponse set CORS header
func SetupResponse(w *http.ResponseWriter, req *http.Request) {
	// cors mode
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	(*w).Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, Cache-Control, Postman-Token")
}
