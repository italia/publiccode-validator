package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"

	yamlv2 "gopkg.in/yaml.v2"

	"github.com/ghodss/yaml"
	"github.com/gorilla/mux"
	publiccode "github.com/italia/publiccode-parser-go"
	vcsurl "github.com/sebbalex/go-vcsurl"
)

//Message json type mapping, for test purpose
type Message struct {
	Status     string                `json:"status"`
	Message    string                `json:"message"`
	Publiccode publiccode.PublicCode `json:"pc"`
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
	// app.initLogger()
	app.initializeRouters()
}

func (app *App) initLogger() {
	f, err := os.OpenFile("app.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Fatal(err)
	}
	// defer f.Close()
	log.SetOutput(f)
	log.Println("Application logging started")
}

// Run http server
func (app *App) Run() {
	log.Printf("server is starting at port %s", app.Port)
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
	log.Printf("mapping to url ko:\n%v\n", err)
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
	log.Printf("parse() called with disableNetwork: %v, and remoteBaseUrl: %s", p.DisableNetwork, p.RemoteBaseURL)
	errParse := p.Parse(b)
	pc, err := p.ToYAML()

	// hack to reset global vars to default values
	app.DisableNetwork = false
	return pc, errParse, err
}

// validateParam will take a query parameter to enable
// or disable network layer on parsing process
// and then call normal validation
func (app *App) validateParam(w http.ResponseWriter, r *http.Request) {
	log.Print("called validateParam()")
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
		log.Printf("var disableNetwork not set, default to true")
	}

	app.validate(w, r)
}

// validate returns a YML or JSON onbject validated and upgraded
// to latest PublicCode version specs.
// It accepts both format as input YML|JSON
func (app *App) validate(w http.ResponseWriter, r *http.Request) {
	log.Print("called validate()")
	setupResponse(&w, r)
	if (*r).Method == "OPTIONS" {
		return
	}

	if r.Body == nil {
		return
	}

	//Closing body after operations
	defer r.Body.Close()
	//reading request
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading body: %v", err)
		mess := Message{Status: string(http.StatusBadRequest), Message: "can't read body"}
		w.Header().Set("Content-type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(mess)
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
			log.Printf("Conversion to json ko:\n%v\n", err)
			mess := Message{Status: string(http.StatusBadRequest), Message: "Conversion to json ko"}
			w.Header().Set("Content-type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(mess)
		}
	} else {
		m = body
	}

	//parsing
	var pc []byte
	var errParse, errConverting error
	pc, errParse, errConverting = app.parse(m)

	if errConverting != nil {
		log.Printf("Error converting: %v", errConverting)
	}
	if errParse != nil {
		// log.Printf("Error parsing: %v", errParse)
		w.Header().Set("Content-type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)

		json, _ := json.Marshal(errParse)
		w.Write(json)
		// json.NewEncoder(w).Encode(errParse)
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
		log.Printf("Conversion to json ko:\n%v\n", err)
	}
	return r
}
