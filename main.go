package main

//
// A basic driver to retrieve databox manifests from a git repo and put them in a store
//

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

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
	if *storeURL != "" {
		//used for testing outside of databox
		ac, _ := databox.NewArbiterClient("./", "./", *arbiterURL)
		sc = databox.NewCoreStoreClient(ac, "./", *storeURL, false)
	} else {
		//look in the standard databox places for config data.
		DATABOX_ZMQ_ENDPOINT := os.Getenv("DATABOX_ZMQ_ENDPOINT")
		sc = databox.NewDefaultCoreStoreClient(DATABOX_ZMQ_ENDPOINT)
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

	//
	// Clone the requested git repo
	//
	os.RemoveAll("./store")
	manifestStore, err := NewGitStore(*gitURL, "./store")
	if err != nil {
		fmt.Println(err)
	}

	//
	// Process the files in the repo looking for databox manifests in the root.
	// Json decode errors are classed as warnings, but these are not written to the store.
	// All valid manifests are written to the correct datasource.
	//
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
		}

		//poll once a hour for new manifests
		time.Sleep(1 * time.Hour)
	}
}
