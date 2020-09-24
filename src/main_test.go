package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

var app App

type Es []ErrorInvalidValue

func (a Es) Len() int           { return len(a) }
func (a Es) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a Es) Less(i, j int) bool { return a[i].Key < a[j].Key }

func TestMain(m *testing.M) {
	app = App{}
	app.init()
	code := m.Run()
	os.Exit(code)
}

func TestGitHub(t *testing.T) {
	url, _ := url.Parse("https://github.com/italia/publiccode-validator")
	assert.Equal(t, getRawURL(url), "https://raw.githubusercontent.com/italia/publiccode-validator/master/")

	url, _ = url.Parse("https://github.com/italia/publiccode-validator.git")
	assert.Equal(t, getRawURL(url), "https://raw.githubusercontent.com/italia/publiccode-validator/master/")
}

func TestConversion(t *testing.T) {
	fileYML, err := os.Open("../tests/valid.minimal.yml")   // For read access.
	fileJSON, err := os.Open("../tests/valid.minimal.json") // For read access.
	if err != nil {
		log.Fatal(err)
	}
	yml, err := ioutil.ReadAll(fileYML)
	json, err := ioutil.ReadAll(fileJSON)
	assert.Equal(t, string(yaml2json(yml)), strings.TrimSpace(string(json)))
}

func TestValidationZeroPayload(t *testing.T) {
	req, _ := http.NewRequest("POST", "/pc/validate?disableNetwork=true", nil)
	response := executeRequest(req)
	checkResponseCode(t, http.StatusOK, response.Code)
	if body := response.Body.String(); body != "" {
		t.Errorf("Expected an message. Got %s", body)
	}
}

func TestValidationErrWithNetwork(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	var errs []ErrorInvalidValue   //[]map[string]interface{}
	var errOut []ErrorInvalidValue //[]map[string]interface{}

	fileYML, err := os.Open("../tests/invalid.yml")
	if err != nil {
		log.Fatal(err)
	}
	out, err := ioutil.ReadFile("../tests/out_invalid.json")
	if err != nil {
		log.Fatal(err)
	}
	req, _ := http.NewRequest("POST", "/pc/validate?disableNetwork=false", fileYML)
	response := executeRequest(req)
	err = json.Unmarshal(out, &errOut)
	if err != nil {
		log.Fatal(err)
	}
	err = json.Unmarshal(response.Body.Bytes(), &errs)
	if err != nil {
		log.Fatal(err)
	}
	sort.Sort(Es(errs))
	sort.Sort(Es(errOut))
	log.Println(errs)
	assert.Equal(t, errs, errOut)
}

func TestValidationWithNoNetwork(t *testing.T) {
	checks := []bool{true, false}
	for _, check := range checks {
		fileYML, err := os.Open("../tests/valid.minimal.yml")
		if err != nil {
			log.Fatal(err)
		}
		out, err := ioutil.ReadFile("../tests/out_valid.minimal.yml")
		if err != nil {
			log.Fatal(err)
		}
		req, _ := http.NewRequest("POST", "/pc/validate?disableNetwork="+strconv.FormatBool(check), fileYML)
		response := executeRequest(req)
		checkResponseCode(t, http.StatusOK, response.Code)
		assert.Equal(t, string(out), response.Body.String())
	}
}

func TestValidationWithNetwork(t *testing.T) {
	fileYML, err := os.Open("../tests/valid.minimal.yml")
	if err != nil {
		log.Fatal(err)
	}
	out, err := ioutil.ReadFile("../tests/out_valid.minimal.yml")
	if err != nil {
		log.Fatal(err)
	}
	req, _ := http.NewRequest("POST", "/pc/validate", fileYML)
	response := executeRequest(req)
	checkResponseCode(t, http.StatusOK, response.Code)
	assert.Equal(t, string(out), response.Body.String())
}

func TestInvalidRemoteURL(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	//url invalid
	urlString := "https://raw.githubusercontent.com/italia/404-publiccode-validator/master/tests/invalid.yml"
	req, _ := http.NewRequest("POST", "/pc/validateURL?url="+urlString, nil)
	response := executeRequest(req)
	checkResponseCode(t, http.StatusNotFound, response.Code)
	//url not present
	urlString = ""
	req, _ = http.NewRequest("POST", "/pc/validateURL?url="+urlString, nil)
	response = executeRequest(req)
	checkResponseCode(t, http.StatusNotFound, response.Code)

	// invalid
	out, err := ioutil.ReadFile("../tests/out_invalid_network.json")
	if err != nil {
		log.Fatal(err)
	}
	urlString = "https://raw.githubusercontent.com/italia/publiccode-validator/master/tests/invalid.yml"
	req, _ = http.NewRequest("POST", "/pc/validateURL?url="+urlString, nil)
	response = executeRequest(req)
	checkResponseCode(t, http.StatusUnprocessableEntity, response.Code)

	var resMessage Message
	json.Unmarshal(response.Body.Bytes(), &resMessage)
	var outMessage Message
	json.Unmarshal(out, &outMessage)

	resOut := resMessage.ValidationError
	localOut := outMessage.ValidationError

	sort.Slice(resOut, func(i, j int) bool {
		return resOut[i].Key < resOut[j].Key
	})

	sort.Slice(localOut, func(i, j int) bool {
		return localOut[i].Key < localOut[j].Key
	})
	assert.Equal(t, localOut, resOut)
}

func TestValidationRemoteURL(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	out, err := ioutil.ReadFile("../tests/out_valid.minimal.yml")
	if err != nil {
		log.Fatal(err)
	}

	urlString := "https://raw.githubusercontent.com/italia/publiccode-validator/master/tests/valid.minimal.yml"
	req, _ := http.NewRequest("POST", "/pc/validateURL?url="+urlString, nil)
	response := executeRequest(req)
	checkResponseCode(t, http.StatusOK, response.Code)
	assert.Equal(t, string(out), response.Body.String())
}

// Utility functions to make mock request and check response
func executeRequest(req *http.Request) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	app.Router.ServeHTTP(rr, req)

	return rr
}

func checkResponseCode(t *testing.T, expected, actual int) {
	if expected != actual {
		t.Errorf("Expected response code %d. Got %d\n", expected, actual)
	}
}
