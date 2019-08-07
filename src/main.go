package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/ghodss/yaml"
	"github.com/gorilla/mux"
	publiccode "github.com/italia/publiccode-parser-go"
	yamlv2 "gopkg.in/yaml.v2"
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

	router.HandleFunc("/pc/validate", pcPayLoad).Methods("POST", "OPTIONS")
	router.HandleFunc("/pc/y2j", y2j).Methods("POST")

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

func pcPayLoad(w http.ResponseWriter, r *http.Request) {
	log.Print("called pcPayload()")
	setupResponse(&w, r)
	if (*r).Method == "OPTIONS" {
		return
	}
	w.Header().Set("Content-type", "application/json")
	// w.Header().Set("Content-type", "text/yaml")

	body, err := ioutil.ReadAll(r.Body)
	log.Print(string(body))
	if err != nil {
		log.Printf("Error reading body: %v", err)
		mess := Message{Status: string(http.StatusBadRequest), Message: "can't read body"}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(mess)
	}

	m, err := yaml.JSONToYAML(body)
	if err != nil {
		fmt.Printf("Conversion to json ko:\n%v\n", err)
		mess := Message{Status: string(http.StatusBadRequest), Message: "Conversion to json ko"}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(mess)
	}
	// y := d.json2yaml()
	// b := y.yaml2bytes()

	// log.Print(y)
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
		w.Write(yaml2json(pc))
	}
}

func yaml2json(y []byte) []byte {
	r, err := yaml.YAMLToJSON(y)
	if err != nil {
		fmt.Printf("Conversion to json ko:\n%v\n", err)
	}
	return r
}

func (d *Enc) yaml2json() []byte {
	log.Print(d.PublicCode)
	m, err := yaml.Marshal(d.PublicCode)
	r, err := yaml.YAMLToJSON(m)
	if err != nil {
		fmt.Printf("Conversion to json ko:\n%v\n", err)
		// return
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

func (d *Enc) yaml2bytes() []byte {
	reqBodyBytes := new(bytes.Buffer)
	json.NewEncoder(reqBodyBytes).Encode(d)
	return reqBodyBytes.Bytes()
}

func y2j(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-type", "application/json")
	var d Enc
	yamlv2.NewDecoder(r.Body).Decode(&d.PublicCode)
	y := d.yaml2json()
	w.Write(y)
	// json.NewEncoder(w).Encode(string(y))
}
