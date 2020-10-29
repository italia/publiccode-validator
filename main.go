package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"regexp"
	"runtime/debug"
	"strconv"

	log "github.com/sirupsen/logrus"

	"github.com/ghodss/yaml"
	"github.com/gorilla/mux"
	publiccode "github.com/italia/publiccode-parser-go"
	"github.com/italia/publiccode-validator/apiv1"
	"github.com/italia/publiccode-validator/utils"
)

var (
	version string
	date    string
)

// App localize type
type App utils.App

func init() {
	if version == "" {
		version = "devel"
		if info, ok := debug.ReadBuildInfo(); ok {
			version = info.Main.Version
		}
	}
	if date == "" {
		date = "(latest)"
	}
	log.Infof("version %s compiled %s\n", version, date)
}

// main server start
func main() {
	app := App{}
	app.Port = "5000"
	app.DisableNetwork = false
	app.initializeRouters()

	// server run here because of tests
	// https://github.com/gorilla/mux#testing-handlers
	log.Infof("server is starting at port %s", app.Port)
	log.Fatal(http.ListenAndServe(":"+app.Port, app.Router))
}

func (app *App) initializeRouters() {
	app.Router = mux.NewRouter()
	var api = app.Router.PathPrefix("/api").Subrouter()
	var api1 = api.PathPrefix("/v1").Subrouter()

	// v0
	app.Router.
		HandleFunc("/pc/validate", app.validateParam).
		Methods("POST", "OPTIONS").
		Queries("disableNetwork", "{disableNetwork}")

	app.Router.
		HandleFunc("/pc/validate", app.validate).
		Methods("POST", "OPTIONS")

	app.Router.
		HandleFunc("/pc/validateURL", app.validateRemoteURL).
		Methods("POST", "OPTIONS").
		Queries("url", "{url}")

	// v1
	api1.
		HandleFunc("/validate", apiv1.ValidateParam).
		Methods("POST", "OPTIONS").
		Queries("disableNetwork", "{disableNetwork}")

	api1.
		HandleFunc("/validate", apiv1.Validate).
		Methods("POST", "OPTIONS")

	api1.
		HandleFunc("/validateURL", apiv1.ValidateRemoteURL).
		Methods("POST", "OPTIONS").
		Queries("url", "{url}")
}

// parse returns new parsed and validated buffer and errors if any
func (app *App) parse(b []byte) ([]byte, error, error) {
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

func (app *App) parseRemoteURL(urlString string) ([]byte, error, error) {
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
		ValidationError: utils.ErrorsToValidationErrors(err),
	}

	w.Header().Set("Content-type", "application/json")
	w.WriteHeader(message.Status)
	messageJSON, _ := json.Marshal(message)
	w.Write(messageJSON)
}

func (app *App) validateRemoteURL(w http.ResponseWriter, r *http.Request) {
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
	pc, errParse, errConverting := app.parseRemoteURL(urlString)

	if errConverting != nil {
		promptError(errConverting, w, http.StatusBadRequest, "Error converting")
		return
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

// validateParam will take a query parameter to enable
// or disable network layer on parsing process
// and then call normal validation
func (app *App) validateParam(w http.ResponseWriter, r *http.Request) {
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

	app.validate(w, r)
}

// validate returns a YML or JSON onbject validated and upgraded
// to latest PublicCode version specs.
// It accepts both format as input YML|JSON
func (app *App) validate(w http.ResponseWriter, r *http.Request) {
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
		return
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
			return
		}
	} else {
		m = body
	}

	// parsing
	pc, errParse, errConverting := app.parse(m)

	if errConverting != nil {
		promptError(errConverting, w, http.StatusBadRequest, "Error converting")
		return
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
