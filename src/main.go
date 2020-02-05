package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	yamlv2 "gopkg.in/yaml.v2"

	vcsurl "github.com/alranel/go-vcsurl"
	"github.com/ghodss/yaml"
	"github.com/gorilla/mux"
	publiccode "github.com/italia/publiccode-parser-go"
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

// main server start
func main() {
	app := App{}
	app.init()
	app.Run()
}

func (app *App) init() {
	app.Port = "5000"
	app.DisableNetwork = false
	app.Router = mux.NewRouter()
	app.initializeRouters()
}

// Run http server
func (app *App) Run() {
	log.Infof("server is starting at port %s", app.Port)
	log.Fatal(http.ListenAndServe(":"+app.Port, app.Router))
}

func (app *App) initializeRouters() {
	//init router
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
}

// setupResponse set CORS header
func setupResponse(w *http.ResponseWriter, req *http.Request) {
	//cors mode
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	(*w).Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
}

// getURLFromYMLBuffer returns a valid URL string based on input object
// takes valid URL as input
func getURLFromYMLBuffer(in []byte) *url.URL {
	var s map[interface{}]interface{}
	yamlv2.NewDecoder(bytes.NewReader(in)).Decode(&s)
	urlString := fmt.Sprintf("%v", s["url"])
	url, err := url.Parse(urlString)
	if err == nil && url.Scheme != "" && url.Host != "" {
		return url
	}
	log.Errorf("mapping to url ko:\n%v\n", err)
	return nil
}

// getRawURL returns a valid raw root repository based on
// major code hosting platforms
func getRawURL(url *url.URL) string {
	rawURL := vcsurl.GetRawRoot(url)
	if rawURL != nil {
		return rawURL.String()
	}
	return ""
}

// parse returns new parsed and validated buffer and errors if any
func (app *App) parse(b []byte) ([]byte, error, error) {
	url := getURLFromYMLBuffer(b)
	p := publiccode.NewParser()
	p.DisableNetwork = app.DisableNetwork

	if url != nil {
		p.DisableNetwork = app.DisableNetwork
		p.RemoteBaseURL = getRawURL(url)
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
	errParse := p.ParseRemoteFile(urlString)
	pc, err := p.ToYAML()

	return pc, errParse, err
}

func promptError(err error, w http.ResponseWriter,
	httpStatus int, mess string) {

	log.Errorf(mess+": %v", err)

	message := Message{
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

	message := Message{
		Status:          httpStatus,
		Message:         mess,
		ValidationError: errorsToSlice(err),
	}

	w.Header().Set("Content-type", "application/json")
	w.WriteHeader(message.Status)
	messageJSON, _ := json.Marshal(message)
	w.Write(messageJSON)
}

func errorsToSlice(errs error) (arr []ErrorInvalidValue) {
	keys := strings.Split(errs.Error(), "\n")
	for _, key := range keys {
		arr = append(arr, ErrorInvalidValue{
			Key: key,
		})
	}
	return
}

func (app *App) validateRemoteURL(w http.ResponseWriter, r *http.Request) {
	log.Info("called validateFromURL()")
	setupResponse(&w, r)
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

	//parsing
	pc, errParse, errConverting := app.parseRemoteURL(urlString)

	if errConverting != nil {
		log.Debugf("Error converting: %s", errConverting)
		promptError(errConverting, w, http.StatusBadRequest, "Error converting")
	}
	if errParse != nil {
		if match, _ := regexp.MatchString(`404`, errParse.Error()); match {
			log.Debugf("URL Error: %s", errParse)
			promptError(errors.New("Not found"), w, http.StatusNotFound, "URL error")
			return
		}
		log.Debugf("Validation Errors: %s", errParse)
		promptValidationErrors(errParse, w, http.StatusUnprocessableEntity, "Validation Errors")
	} else {
		//set response CT based on client accept header
		//and return respectively content
		if r.Header.Get("Accept") == "application/json" {
			w.Header().Set("Content-type", "application/json")
			w.Write(yaml2json(pc))
			return
		}
		//default choice
		w.Header().Set("Content-type", "application/x-yaml")
		w.Write(pc)
	}
}

// validateParam will take a query parameter to enable
// or disable network layer on parsing process
// and then call normal validation
func (app *App) validateParam(w http.ResponseWriter, r *http.Request) {
	log.Info("called validateParam()")
	setupResponse(&w, r)
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
	setupResponse(&w, r)
	if (*r).Method == "OPTIONS" {
		return
	}

	if r.Body == nil {
		log.Info("empty payload")
		return
	}

	//Closing body after operations
	defer r.Body.Close()
	//reading request
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

	//parsing
	var pc []byte
	var errParse, errConverting error
	pc, errParse, errConverting = app.parse(m)

	if errConverting != nil {
		log.Debugf("Error converting: %s", errConverting)
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
		//set response CT based on client accept header
		//and return respectively content
		if r.Header.Get("Accept") == "application/json" {
			w.Header().Set("Content-type", "application/json")
			w.Write(yaml2json(pc))
			return
		}
		//default choice
		w.Header().Set("Content-type", "application/x-yaml")
		w.Write(pc)
	}
}

// yaml2json yaml to json conversion
func yaml2json(y []byte) []byte {
	r, err := yaml.YAMLToJSON(y)
	if err != nil {
		log.Errorf("Conversion to json ko:\n%v\n", err)
	}
	return r
}
