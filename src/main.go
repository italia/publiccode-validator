package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"

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

// main server start
func main() {
	port := "5000"
	//init router
	router := mux.NewRouter()

	router.HandleFunc("/pc/validate", validate).Methods("POST", "OPTIONS")

	log.Printf("server is starting at port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
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
	if vcsurl.GetRawRoot(url) != nil {
		return vcsurl.GetRawRoot(url).String()
	}
	return ""
}

// parse returns new parsed and validated buffer and errors if any
func parse(b []byte) ([]byte, error, error) {
	url := getURLFromYMLBuffer(b)
	p := publiccode.NewParser()
	p.DisableNetwork = true
	if url != nil {
		p.DisableNetwork = false
		p.RemoteBaseURL = getRawURL(url)
	}
	log.Printf("parse() called with disableNetwork: %v, and remoteBaseUrl: %s", p.DisableNetwork, p.RemoteBaseURL)
	errParse := p.Parse(b)
	pc, err := p.ToYAML()
	return pc, errParse, err
}

// validate returns a YML or JSON onbject validated and upgraded
// to latest PublicCode version specs.
// It accepts both format as input YML|JSON
func validate(w http.ResponseWriter, r *http.Request) {
	log.Print("called validate()")
	setupResponse(&w, r)
	if (*r).Method == "OPTIONS" {
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
	pc, errParse, errConverting = parse(m)

	if errConverting != nil {
		log.Printf("Error converting: %v", errConverting)
	}
	if errParse != nil {
		log.Printf("Error parsing: %v", errParse)
		w.Header().Set("Content-type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(errParse)
	} else {
		//set response CT based on client accept header
		//and return respectively content
		if r.Header.Get("Accept") == "application/json" {
			w.Header().Set("Content-type", "application/json")
			w.Write(yaml2json(pc))
			return
		}
		//default choice
		w.Header().Set("Content-type", "text/yaml")
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
