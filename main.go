package main

//
// A basic driver to retrieve databox manifests from a git repo and put them in a store
//

import (
	"crypto/md5"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/mux"
	databox "github.com/me-box/lib-go-databox"
)

var databoxPlatformStore = Store{
	Name:   "Databox Official Store",
	GitUrl: "https://github.com/me-box/databox-manifest-store",
}

var storeURL *string
var arbiterURL *string

func main() {

	gitURL := flag.String("giturl", "", "databox store url normally from env vars")
	storeURL = flag.String("storeurl", "", "databox store url normally from env vars")
	arbiterURL = flag.String("arbiterurl", "", "databox arbiter url normally from env vars")
	tagOption := flag.String("tag", "", "repo version tag normally from env vars")
	flag.Parse()

	//
	// work out if we are running inside or outside databox and set up lib-go-databox accordingly
	//
	insideDatabox := false
	tag := ""
	if *storeURL != "" {
		//used for testing outside of databox
		tag = *tagOption
		databoxPlatformStore.GitUrl = *gitURL
	} else {
		tag = os.Getenv("DATABOX_VERSION")
		databoxPlatformStore.GitUrl = os.Getenv("DATABOX_STORE_URL")
		insideDatabox = true
		//give databox config time to take effect
		time.Sleep(time.Second * 10)
	}

	//
	// Register the app and driver KV datasources
	//
	registerMyDatasource()

	forceUpdateChan := make(chan int)
	go PollForManifests(tag, forceUpdateChan)

	//
	// Handel Https requests
	//
	pathPrefix := ""
	if !insideDatabox {
		pathPrefix = "/app-store"
	}
	router := mux.NewRouter()

	router.HandleFunc(pathPrefix+"/ui", DisplayUI()).Methods("GET")
	router.HandleFunc(pathPrefix+"/ui/api/addManifest", AddManifest()).Methods("POST")
	router.HandleFunc(pathPrefix+"/ui/api/addStore", AddStore(forceUpdateChan)).Methods("POST")
	router.HandleFunc(pathPrefix+"/ui/api/removeStore", removeStore(forceUpdateChan)).Methods("POST")
	router.HandleFunc(pathPrefix+"/ui/api/refresh", refresh(forceUpdateChan))

	static := http.StripPrefix(pathPrefix+"/ui/static", http.FileServer(http.Dir("./public/")))
	router.PathPrefix(pathPrefix + "/ui/static").Handler(static)

	if insideDatabox {
		log.Fatal(http.ListenAndServeTLS(":8080", databox.GetHttpsCredentials(), databox.GetHttpsCredentials(), router))
	} else {
		//use http for testing
		log.Fatal(http.ListenAndServe(":8080", router))
	}

}

func insideDatabox() bool {
	if *storeURL != "" {
		return false
	}
	return true
}

func getStoreClient() *databox.CoreStoreClient {
	var sc *databox.CoreStoreClient
	if !insideDatabox() {
		ac, _ := databox.NewArbiterClient("./", "./", *arbiterURL)
		sc = databox.NewCoreStoreClient(ac, "./", *storeURL, false)
	} else {
		DATABOX_ZMQ_ENDPOINT := os.Getenv("DATABOX_ZMQ_ENDPOINT")
		sc = databox.NewDefaultCoreStoreClient(DATABOX_ZMQ_ENDPOINT)
	}
	return sc
}

// PollForManifests will:
// Process the files in the repo looking for databox manifests in the root.
// Json decode errors are classed as warnings, but these are not written to the store.
// All valid manifests are written to the correct datasource.
//
func PollForManifests(tag string, updateChan <-chan int) {

	sc := getStoreClient()
	for {

		storeList := getStores(sc)
		fmt.Println("[storeList]", storeList)
		for _, store := range storeList {

			// Clone/open the requested git repo
			name := fmt.Sprintf("%x", md5.Sum([]byte(store.GitUrl)))
			manifestStore, err := NewGitStore(store.GitUrl, "./"+name, tag)
			if err != nil {
				fmt.Println("[Error] Getting git repo ", store.GitUrl, " ", err)
				continue
			}

			//Pull to update and return the manifests
			manifests, err := manifestStore.Get()
			if err != nil {
				fmt.Println(err)
				continue
			}

			for _, manifest := range *manifests {
				jsonStr, err := json.Marshal(manifest)
				if err != nil {
					fmt.Println(err)
					continue
				}
				fmt.Println("Adding " + manifest.Name + " from " + store.Name)
				err = sc.KVJSON.Write(string(manifest.DataboxType)+"s", manifest.Name, jsonStr)
				if err != nil {
					fmt.Println(err)
				}
				err = sc.KVJSON.Write("all", manifest.Name, jsonStr)
				if err != nil {
					fmt.Println(err)
				}
			}
		}

		//Block and wait for forced update or time out
		select {
		case <-updateChan:
			fmt.Println("Update requested")
		case <-time.After(1 * time.Minute):
			fmt.Println("Updating after time out")
		}
	}
}

//removeStore http handler to remove a store
func removeStore(forceUpdateChan chan int) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		sc := getStoreClient()

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"status":"500","error": %s}`, "Reading body")
			return
		}
		defer r.Body.Close()
		fmt.Println("[RemoveStore] data received")

		var s Store
		err = json.Unmarshal(body, &s)
		UrlOK := strings.Contains(s.GitUrl, "github.com")
		if err != nil || s.Name == "" || !UrlOK {
			fmt.Println("[Error] Mallformed store json")
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"status":"500","error": %s}`, "Mallformed store json")
			return
		}
		fmt.Println("[RemoveStore] decoded")
		fmt.Printf("%+v\n", s)

		name := fmt.Sprintf("%x", md5.Sum([]byte(s.GitUrl)))
		sc.KVJSON.Delete("registeredStores", name)

		os.RemoveAll("./" + name)

		sc.KVJSON.DeleteAll("all")
		sc.KVJSON.DeleteAll("apps")
		sc.KVJSON.DeleteAll("drivers")

		fmt.Println("[RemoveStore] done")

		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status": %s}`, "200")

		//send a message to force an update
		forceUpdateChan <- 1
	}
}

//AddStore http handler to add another store to the driver-app-store its should be a git repo
//that is viable publicly (read only)
func AddStore(forceUpdateChan chan int) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {

		sc := getStoreClient()

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"status":"500","error": %s}`, "Reading body")
			return
		}
		defer r.Body.Close()
		fmt.Println("[Add Store] data received")

		var s Store
		err = json.Unmarshal(body, &s)
		UrlOK := strings.Contains(s.GitUrl, "github.com")
		if err != nil || s.Name == "" || !UrlOK {
			fmt.Println("[Error] Mallformed store json")
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"status":"500","error": %s}`, "Mallformed store json")
			return
		}
		fmt.Println("[add Store] decoded")
		fmt.Printf("%+v\n", s)

		name := fmt.Sprintf("%x", md5.Sum([]byte(s.GitUrl)))
		sc.KVJSON.Write("registeredStores", name, body)

		fmt.Println("[add Store] data written")

		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status": %s}`, "200")

		//send a message to force an update
		forceUpdateChan <- 1
	}
}

//AddManifest http handler to manually adda manifest to the manifest store used in development
func AddManifest() func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {

		sc := getStoreClient()

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

func refresh(forceUpdateChan chan int) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status": %s}`, "200")

		//send a message to force an update
		forceUpdateChan <- 1
	}
}

func DisplayUI() func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {

		sc := getStoreClient()

		//create manifest list
		manifestList, _ := sc.KVJSON.ListKeys("all")
		manifestLis := ""
		for _, m := range manifestList {
			manifestLis += fmt.Sprintf("<li class=\"installed\" id=\"%s\">%s</li>\n", m, m)
		}

		//create store list
		storeList, _ := sc.KVJSON.ListKeys("registeredStores")
		storeLis := `<li class="installed">` + databoxPlatformStore.Name + `</li>`
		for _, sid := range storeList {
			storeJson, _ := sc.KVJSON.Read("registeredStores", sid)
			var s Store
			json.Unmarshal(storeJson, &s)
			storeLis += fmt.Sprintf(`<li class="installed">%s <button onclick="UninstallStore('%s','%s')">remove</button></li>`, s.Name, s.Name, s.GitUrl)
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
				<p><button onclick="RefreshStore()">Refresh</button></p>
				<div id="manifestList">
					<ul>
						%s
					</ul>
				</div>
				<h2>Available Stores</h2>
				<div id="manifestList">
					<ul>
						%s
					</ul>
				</div>
				<h2>Add External Store</h2>
				<p>Here you can add a new store to get manifests from another provider. This is useful if you want to install apps from the SDK or other providers. </p>
				<p>Warning code outside of the official repo is not supported or vetted by databox. Repo URLs must be on githib.com</p>
				<div>
				<label for="repoName">Name</label><input type="text" id="repoName" /> <br/>
				<label for="repoUrl">GitHub Url</label><input type="text" id="repoUrl" /> <br/>
				<button onclick="AddStore()">Add External Store</button> <br/>
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
		w.Write([]byte(fmt.Sprintf(html, manifestLis, storeLis)))
	}
}

//registerMyDatasource just sets up the data sources
func registerMyDatasource() {

	sc := getStoreClient()

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
}

type Store struct {
	Name   string `json:"name"`
	GitUrl string `json:"giturl"`
}

//returns a list of store form the datastore
//will always contain databoxPlatformStore
func getStores(sc *databox.CoreStoreClient) []Store {

	keys, _ := sc.KVJSON.ListKeys("registeredStores")
	installedStores := []Store{databoxPlatformStore}
	for _, storekey := range keys {
		storeJson, _ := sc.KVJSON.Read("registeredStores", storekey)
		var s Store
		json.Unmarshal(storeJson, &s)
		installedStores = append(installedStores, s)
	}

	return installedStores
}
