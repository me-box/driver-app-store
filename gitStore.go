package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	databox "github.com/me-box/lib-go-databox"
	git "gopkg.in/src-d/go-git.v4"
)

type manitestStoreage struct {
	repo *git.Repository
	path string
}

func NewGitStore(repoUrl string, path string) (*manitestStoreage, error) {

	r, err := git.PlainClone(path, false, &git.CloneOptions{
		URL:      repoUrl,
		Progress: os.Stdout,
	})

	if err == git.ErrRepositoryAlreadyExists {
		r, err = git.PlainOpen(path)
	}

	return &manitestStoreage{
		repo: r,
		path: path,
	}, err

}

func (ms *manitestStoreage) Get() (*[]databox.Manifest, error) {

	var mlist []databox.Manifest

	wt, err := ms.repo.Worktree()
	wt.Pull(&git.PullOptions{
		Depth:    1,
		Progress: os.Stdout,
	})

	files, err := ioutil.ReadDir(ms.path)
	if err != nil {
		log.Fatal(err)
	}

	for _, f := range files {
		if filepath.Ext(f.Name()) == ".json" {
			fmt.Println(f.Name())
			data, _ := ioutil.ReadFile(ms.path + "/" + f.Name())
			var manifest databox.Manifest
			err := json.Unmarshal(data, &manifest)
			if err != nil {
				fmt.Println("[Error parsing file] ", f.Name(), err)
				continue
			}
			mlist = append(mlist, manifest)
		}
	}

	return &mlist, nil
}
