package main

import (
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestGitHub(t *testing.T) {
	url, _ := url.Parse("https://github.com/sebbalex/pc-web-validator")
	AssertEqual(t, getRawURL(url), "https://raw.githubusercontent.com/sebbalex/pc-web-validator/master/")

	url, _ = url.Parse("https://github.com/sebbalex/pc-web-validator.git")
	AssertEqual(t, getRawURL(url), "https://raw.githubusercontent.com/sebbalex/pc-web-validator/master/")
}

func TestConversion(t *testing.T) {
	fileYML, err := os.Open("../tests/valid.minimal.yml")   // For read access.
	fileJSON, err := os.Open("../tests/valid.minimal.json") // For read access.
	if err != nil {
		log.Fatal(err)
	}
	yml, err := ioutil.ReadAll(fileYML)
	json, err := ioutil.ReadAll(fileJSON)
	AssertEqual(t, string(yaml2json(yml)), strings.TrimSpace(string(json)))
}

// AssertEqual checks if values are equal
func AssertEqual(t *testing.T, a interface{}, b interface{}) {
	if a == b {
		return
	}
	// debug.PrintStack()
	_, fn, line, _ := runtime.Caller(1)
	t.Errorf("%s:%d: Received %v (type %v), expected %v (type %v)", fn, line, a, reflect.TypeOf(a), b, reflect.TypeOf(b))
}

// AssertNil checks if a value is nil
func AssertNil(t *testing.T, a interface{}) {
	if reflect.ValueOf(a).IsNil() {
		return
	}
	//debug.PrintStack()
	_, fn, line, _ := runtime.Caller(1)
	t.Errorf("%s:%d: Received %v (type %v), expected nil", fn, line, a, reflect.TypeOf(a))
}
