package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"

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
	url, _ := url.Parse("https://github.com/sebbalex/pc-web-validator")
	assert.Equal(t, getRawURL(url), "https://raw.githubusercontent.com/sebbalex/pc-web-validator/master/")

	url, _ = url.Parse("https://github.com/sebbalex/pc-web-validator.git")
	assert.Equal(t, getRawURL(url), "https://raw.githubusercontent.com/sebbalex/pc-web-validator/master/")
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

func TestValidationErrWithNoNetwork(t *testing.T) {
	var errs []ErrorInvalidValue   //[]map[string]interface{}
	var errOut []ErrorInvalidValue //[]map[string]interface{}

	fileYML, err := os.Open("../tests/missing_maintenance_contacts.yml")
	if err != nil {
		log.Fatal(err)
	}
	out, err := ioutil.ReadFile("../tests/invalid_out.log")
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
	assert.Equal(t, errs, errOut)
}

func TestValidationWithNoNetwork(t *testing.T) {
	checks := []bool{true, false}
	for _, check := range checks {
		fileYML, err := os.Open("../tests/valid.minimal.yml")
		if err != nil {
			log.Fatal(err)
		}
		out, err := ioutil.ReadFile("../tests/valid.minimal.out.yml")
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
	out, err := ioutil.ReadFile("../tests/valid.minimal.out.yml")
	if err != nil {
		log.Fatal(err)
	}
	req, _ := http.NewRequest("POST", "/pc/validate", fileYML)
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
