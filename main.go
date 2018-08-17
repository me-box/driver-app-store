package main

//
// A basic driver to retrieve databox manifests from a git repo and put them in a store
//

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	databox "github.com/toshbrown/lib-go-databox"
)

const databoxGitURL = "https://github.com/toshbrown/databox-manifest-store"

func main() {

	//give databox config time to take effect
	time.Sleep(time.Second * 10)

	gitURL := flag.String("giturl", databoxGitURL, "Git repo url")
	storeURL := flag.String("storeurl", "", "databox store url normally from env vars")
	arbiterURL := flag.String("arbiterurl", "", "databox arbiter url normally from env vars")
	flag.Parse()

	//
	// work out if we are running inside or outside databox and set up lib-go-databox accordingly
	//
	var sc *databox.CoreStoreClient
	insideDatabox := false
	if *storeURL != "" {
		//used for testing outside of databox
		ac, _ := databox.NewArbiterClient("./", "./", *arbiterURL)
		sc = databox.NewCoreStoreClient(ac, "./", *storeURL, false)
	} else {
		//look in the standard databox places for config data.
		DATABOX_ZMQ_ENDPOINT := os.Getenv("DATABOX_ZMQ_ENDPOINT")
		sc = databox.NewDefaultCoreStoreClient(DATABOX_ZMQ_ENDPOINT)
		insideDatabox = true
	}

	//
	// Register the app and driver KV datasources
	//
	metadata := databox.DataSourceMetadata{
		Description:    "Databox app manifests",
		ContentType:    "application/json",
		Vendor:         "Databox",
		DataSourceType: "databox:manifests:app",
		DataSourceID:   "apps",
		StoreType:      "kv",
		IsActuator:     false,
		Unit:           "",
		Location:       "",
	}
	err := sc.RegisterDatasource(metadata)
	if err != nil {
		fmt.Println(err)
	}

	metadata = databox.DataSourceMetadata{
		Description:    "Databox driver manifests",
		ContentType:    "application/json",
		Vendor:         "Databox",
		DataSourceType: "databox:manifests:driver",
		DataSourceID:   "drivers",
		StoreType:      "kv",
		IsActuator:     false,
		Unit:           "",
		Location:       "",
	}
	err = sc.RegisterDatasource(metadata)
	if err != nil {
		fmt.Println(err)
	}

	metadata = databox.DataSourceMetadata{
		Description:    "Databox manifests",
		ContentType:    "application/json",
		Vendor:         "Databox",
		DataSourceType: "databox:manifests:all",
		DataSourceID:   "all",
		StoreType:      "kv",
		IsActuator:     false,
		Unit:           "",
		Location:       "",
	}
	err = sc.RegisterDatasource(metadata)
	if err != nil {
		fmt.Println(err)
	}

	//
	// Clone the requested git repo
	//
	os.RemoveAll("./store")
	manifestStore, err := NewGitStore(*gitURL, "./store")
	if err != nil {
		fmt.Println(err)
	}

	go PollForManifests(manifestStore, sc)

	//
	// Handel Https requests
	//
	pathPrefix := ""
	if !insideDatabox {
		pathPrefix = "/app-store"
	}
	router := mux.NewRouter()

	router.HandleFunc(pathPrefix+"/ui", DisplayUI(sc)).Methods("GET")
	router.HandleFunc(pathPrefix+"/ui/api/addManifest", AddManifest(sc)).Methods("POST")

	static := http.StripPrefix(pathPrefix+"/ui/static", http.FileServer(http.Dir("./public/")))
	router.PathPrefix(pathPrefix + "/ui/static").Handler(static)

	if insideDatabox {
		log.Fatal(http.ListenAndServeTLS(":8080", databox.GetHttpsCredentials(), databox.GetHttpsCredentials(), router))
	} else {
		//use http for testing
		log.Fatal(http.ListenAndServe(":8080", router))
	}

}

// PollForManifests will:
// Process the files in the repo looking for databox manifests in the root.
// Json decode errors are classed as warnings, but these are not written to the store.
// All valid manifests are written to the correct datasource.
//
func PollForManifests(manifestStore *manitestStoreage, sc *databox.CoreStoreClient) {

	for {
		manifests, err := manifestStore.Get()

		if err != nil {
			fmt.Println(err)
		}

		for _, manifest := range *manifests {
			jsonStr, err := json.Marshal(manifest)
			if err != nil {
				fmt.Println(err)
				continue
			}
			err = sc.KVJSON.Write(string(manifest.DataboxType)+"s", manifest.Name, jsonStr)
			if err != nil {
				fmt.Println(err)
			}
			err = sc.KVJSON.Write("all", manifest.Name, jsonStr)
			if err != nil {
				fmt.Println(err)
			}
		}

		//poll once a hour for new manifests
		time.Sleep(1 * time.Hour)
	}
}

func AddManifest(sc *databox.CoreStoreClient) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"status":"500","error": %s}`, "Reading body")
			return
		}
		defer r.Body.Close()
		fmt.Println("[add manifest] data received")

		var manifest databox.Manifest
		err = json.Unmarshal(body, &manifest)
		if err != nil {
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"status":"500","error": %s}`, "Mallformed manifest")
			return
		}
		fmt.Println("[add manifest] decoded")
		fmt.Printf("%+v\n", manifest)

		err = sc.KVJSON.Write("all", manifest.Name, body)
		if err != nil {
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"status":"500","error": %s}`, "Writing request to store")
			return
		}
		err = sc.KVJSON.Write(string(manifest.DataboxType)+"s", manifest.Name, body)
		if err != nil {
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"status":"500","error": %s}`, "Writing request to store")
			return
		}

		fmt.Println("[add manifest] data written")

		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status": %s}`, "200")
	}
}

func DisplayUI(sc *databox.CoreStoreClient) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		manifestList, _ := sc.KVJSON.ListKeys("all")

		htmlLis := ""
		for _, m := range manifestList {
			htmlLis += fmt.Sprintf("<li class=\"installed\" id=\"%s\">%s</li>\n", m, m)
		}

		html := `
		<!DOCTYPE html>
		<html lang="en">
			<head>
				<meta charset="utf-8">
				<meta http-equiv="X-UA-Compatible" content="IE=edge">
				<meta name="viewport" content="width=device-width,initial-scale=1.0">
				<title>ui</title>
				<script type="text/javascript" src="ui/static/main.js"></script>
			</head>
			<body>
				<noscript>
				<strong>We're sorry but ui doesn't work properly without JavaScript enabled. Please enable it to continue.</strong>
				</noscript>
				<h2>Installed manifests</h2>
				<div id="manifestList">
					<ul>
						%s
					</ul>
				</div>
				<h2>Upload local Manifest</h2>
				<p>Here you can upload a databox manifest. This is useful if you want to test an App/Driver that you are developing. </p>
				<div>
					<form id="file-form">
						<input type="file" id="file-select" name="photos[]" multiple/>
						<button type="submit" id="upload-button">Add manifest</button>
					</form>
				</div>
			</body>
		</html>
		`
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(html, htmlLis)))
	}
}
