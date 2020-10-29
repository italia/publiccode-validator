package apiv1

import (
	json "encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/ghodss/yaml"
	"github.com/gorilla/mux"
	"github.com/italia/publiccode-parser-go"
	"github.com/italia/publiccode-validator/utils"
	log "github.com/sirupsen/logrus"
)

var app utils.App

// parse returns new parsed and validated buffer and errors if any
func parse(b []byte) ([]byte, error, error) {
	url, err := utils.GetURLFromYMLBuffer(b)
	if err != nil {
		// this error should not be blocking because it just means
		// that no url is inside the request body.
		// one case for that: partial validation during editing
		log.Warnf("url not found in body (useful to get RemoteBaseURL): %s", err)
	}
	p := publiccode.NewParser()
	p.DisableNetwork = app.DisableNetwork

	if url != nil {
		p.RemoteBaseURL = utils.GetRawURL(url)
	}
	log.Debugf("parse() called with disableNetwork: %v, and remoteBaseUrl: %s", p.DisableNetwork, p.RemoteBaseURL)
	errParse := p.Parse(b)
	pc, err := p.ToYAML()

	// hack to reset global vars to default values
	app.DisableNetwork = false
	return pc, errParse, err
}

func parseRemoteURL(urlString string) ([]byte, error, error) {
	log.Infof("called parseRemoteURL() url: %s", urlString)
	p := publiccode.NewParser()
	urlString, err := utils.GetRawFile(urlString)
	if err != nil {
		return nil, nil, err
	}
	errParse := p.ParseRemoteFile(urlString)
	pc, err := p.ToYAML()

	return pc, errParse, err
}

func promptError(err error, w http.ResponseWriter, acceptHeader string,
	httpStatus int, mess string) {

	message := utils.Message{
		Status:          httpStatus,
		Message:         mess,
		ValidationError: utils.ErrorsToValidationErrors(err),
	}
	if message.ValidationError == nil {
		message.Error = err.Error()
	}

	log.Debugf("promptError message: %v", message)
	w.Header().Set("Content-type", acceptHeader)
	w.WriteHeader(message.Status)

	if acceptHeader == "application/json" {
		o, _ := json.Marshal(message)
		w.Write(o)
	} else {
		o, _ := yaml.Marshal(message)
		w.Write(o)
	}
}

func elaborate(pc []byte, errParse error, errConverting error, w http.ResponseWriter, acceptHeader string) {
	if errConverting != nil {
		promptError(errConverting, w, acceptHeader, http.StatusBadRequest, "Error converting")
		return
	}
	if errParse != nil {
		promptError(errParse, w, acceptHeader, http.StatusUnprocessableEntity, "Validation Errors")
		return
	}

	// set response CT based on client accept header
	// and return respectively content
	if acceptHeader == "application/json" {
		w.Header().Set("Content-type", "application/json")
		w.Write(utils.Yaml2json(pc))
		return
	}
	// default choice
	w.Header().Set("Content-type", "application/x-yaml")
	w.Write(pc)
}

// ValidateRemoteURL validate remote URL
func ValidateRemoteURL(w http.ResponseWriter, r *http.Request) {
	log.Info("called validateFromURL()")
	utils.SetupResponse(&w, r)
	if (*r).Method == "OPTIONS" {
		return
	}
	// Getting vars from parameters
	vars := mux.Vars(r)
	urlString := vars["url"]

	acceptHeader := "application/x-yaml"
	if r.Header.Get("Accept") != "*/*" {
		acceptHeader = r.Header.Get("Accept")
	}
	if urlString == "" {
		promptError(errors.New("URL not found"), w, acceptHeader, http.StatusNotFound, "URL error")
		return
	}

	// parsing
	pc, errParse, errConverting := parseRemoteURL(urlString)

	elaborate(pc, errParse, errConverting, w, acceptHeader)
}

// ValidateParam will take a query parameter to enable
// or disable network layer on parsing process
// and then call normal validation
func ValidateParam(w http.ResponseWriter, r *http.Request) {
	log.Info("called validateParam()")
	utils.SetupResponse(&w, r)
	if (*r).Method == "OPTIONS" {
		return
	}
	// Getting vars from parameters
	vars := mux.Vars(r)
	disableNetwork, err := strconv.ParseBool(vars["disableNetwork"])
	app.DisableNetwork = disableNetwork

	if err != nil {
		app.DisableNetwork = false
		log.Info("var disableNetwork not set, default to true")
	}

	Validate(w, r)
}

// Validate returns a YML or JSON object validated and upgraded
// to latest PublicCode version specs.
// It accepts both format as input YML|JSON
func Validate(w http.ResponseWriter, r *http.Request) {
	log.Info("/api/v1/validate")
	utils.SetupResponse(&w, r)
	if (*r).Method == "OPTIONS" {
		return
	}

	acceptHeader := "application/x-yaml"
	if r.Header.Get("Accept") != "*/*" {
		acceptHeader = r.Header.Get("Accept")
	}

	if r.Body == nil {
		promptError(fmt.Errorf("empty payload"), w, acceptHeader, http.StatusBadRequest, "Empty payload")
		return
	}

	// Closing body after operations
	defer r.Body.Close()
	// reading request
	body, err := ioutil.ReadAll(r.Body)

	if err != nil {
		promptError(err, w, acceptHeader, http.StatusBadRequest, "Error reading body")
	}

	if len(body) == 0 {
		promptError(fmt.Errorf("empty payload"), w, acceptHeader, http.StatusBadRequest, "Empty payload")
		return
	}

	// these functions take as argument a request body
	// convert content in YAML format if needed
	// and return a bytes array needed from Parser
	// to validate correctly
	// here, based on content-type header must convert
	// [yaml/json] content into []byte

	// parsing
	pc, errParse, errConverting := parse(body)

	elaborate(pc, errParse, errConverting, w, acceptHeader)
}
