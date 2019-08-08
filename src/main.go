package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/ghodss/yaml"
	"github.com/gorilla/mux"
	publiccode "github.com/italia/publiccode-parser-go"
)

//Enc encoder struct which includes essential datatypes
type Enc struct {
	PublicCode publiccode.PublicCode
}

//Message json type mapping, for test purpose
type Message struct {
	Status     string                `json:"status"`
	Message    string                `json:"message"`
	Publiccode publiccode.PublicCode `json:"pc"`
}

func main() {
	port := "5000"
	//init router
	router := mux.NewRouter()

	router.HandleFunc("/pc/validate", validate).Methods("POST", "OPTIONS")

	log.Printf("server is starting at port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}

func setupResponse(w *http.ResponseWriter, req *http.Request) {
	//cors mode
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	(*w).Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
}

func parse(b []byte) ([]byte, error, error) {
	p := publiccode.NewParser()
	p.DisableNetwork = true
	errParse := p.Parse(b)
	pc, err := p.ToYAML()
	return pc, errParse, err
}

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
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(mess)
		}
		log.Print("CT json")
	} else {
		m = body
		log.Print("CT yaml: ", string(m), err)
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

func yaml2json(y []byte) []byte {
	r, err := yaml.YAMLToJSON(y)
	if err != nil {
		fmt.Printf("Conversion to json ko:\n%v\n", err)
	}
	return r
}

func (d *Enc) json2yaml() []byte {
	log.Print(d.PublicCode)
	m, err := yaml.Marshal(d.PublicCode)
	// log.Print(string(m))
	if err != nil {
		fmt.Printf("Marshall to yaml ko:\n%v\n", err)
		// return
	}

	r, err := yaml.JSONToYAML(m)
	// log.Print(string(r))
	if err != nil {
		fmt.Printf("Conversion to yaml ko:\n%v\n", err)
		// return
	}
	return r
}
