package apiv1

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"regexp"
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
	url := utils.GetURLFromYMLBuffer(b)
	p := publiccode.NewParser()
	p.DisableNetwork = app.DisableNetwork

	if url != nil {
		p.DisableNetwork = app.DisableNetwork
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
	errParse := p.ParseRemoteFile(utils.GetRawFile(urlString))
	pc, err := p.ToYAML()

	return pc, errParse, err
}

func promptError(err error, w http.ResponseWriter,
	httpStatus int, mess string) {

	log.Errorf(mess+": %v", err)

	message := utils.Message{
		Status:  httpStatus,
		Message: mess,
		Error:   err.Error(),
	}
	log.Debugf("message: %v", message)
	w.Header().Set("Content-type", "application/json")
	w.WriteHeader(message.Status)
	json.NewEncoder(w).Encode(message)
}

func promptValidationErrors(err error, w http.ResponseWriter,
	httpStatus int, mess string) {

	log.Errorf(mess+": %v", err)

	message := utils.Message{
		Status:          httpStatus,
		Message:         mess,
		ValidationError: utils.ErrorsToSlice(err),
	}

	w.Header().Set("Content-type", "application/json")
	w.WriteHeader(message.Status)
	messageJSON, _ := json.Marshal(message)
	w.Write(messageJSON)
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
	if urlString == "" {
		promptError(errors.New("Not found"), w, http.StatusNotFound, "URL error")
		return
	}

	// parsing
	pc, errParse, errConverting := parseRemoteURL(urlString)

	if errConverting != nil {
		promptError(errConverting, w, http.StatusBadRequest, "Error converting")
	}
	if errParse != nil {
		if match, _ := regexp.MatchString(`404`, errParse.Error()); match {
			promptError(errors.New("Not found"), w, http.StatusNotFound, "URL error")
			return
		}
		promptValidationErrors(errParse, w, http.StatusUnprocessableEntity, "Validation Errors")
	} else {
		// set response CT based on client accept header
		// and return respectively content
		if r.Header.Get("Accept") == "application/json" {
			w.Header().Set("Content-type", "application/json")
			w.Write(utils.Yaml2json(pc))
			return
		}
		// default choice
		w.Header().Set("Content-type", "application/x-yaml")
		w.Write(pc)
	}
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

// Validate returns a YML or JSON onbject validated and upgraded
// to latest PublicCode version specs.
// It accepts both format as input YML|JSON
func Validate(w http.ResponseWriter, r *http.Request) {
	log.Info("called validate()")
	utils.SetupResponse(&w, r)
	if (*r).Method == "OPTIONS" {
		return
	}

	if r.Body == nil {
		log.Info("empty payload")
		return
	}

	// Closing body after operations
	defer r.Body.Close()
	// reading request
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		promptError(err, w, http.StatusBadRequest, "Error reading body")
	}

	// these functions take as argument a request body
	// convert content in YAML format if needed
	// and return a bytes array needed from Parser
	// to validate correctly
	// here, based on content-type header must convert
	// [yaml/json] content into []byte
	var m []byte

	if r.Header.Get("Content-Type") == "application/json" {
		//converting to YML
		m, err = yaml.JSONToYAML(body)
		if err != nil {
			promptError(err, w, http.StatusBadRequest, "Conversion to json ko")
		}
	} else {
		m = body
	}

	// parsing
	var pc []byte
	var errParse, errConverting error
	pc, errParse, errConverting = parse(m)

	if errConverting != nil {
		promptError(errConverting, w, http.StatusBadRequest, "Error converting")
	}
	if errParse != nil {
		log.Debugf("Validation Errors: %s", errParse)

		// consider switch to promptError()
		w.Header().Set("Content-type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)

		json, _ := json.Marshal(errParse)
		w.Write(json)
		// promptError(errParse, w, http.StatusUnprocessableEntity, "Error parsing")
	} else {
		// set response CT based on client accept header
		// and return respectively content
		if r.Header.Get("Accept") == "application/json" {
			w.Header().Set("Content-type", "application/json")
			w.Write(utils.Yaml2json(pc))
			return
		}
		// default choice
		w.Header().Set("Content-type", "application/x-yaml")
		w.Write(pc)
	}
}
