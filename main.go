package main

import (
	"flag"
	go_aspace "github.com/nyudlts/go-aspace"
	"log"
	"regexp"
)

var (
	aspace      *go_aspace.ASClient
	config      string
	environment string
	repository  int
	resource    int
	test        bool
	aeonPtn     = regexp.MustCompile("aeon.library.nyu.edu")
)

func init() {
	flag.StringVar(&config, "config", "", "")
	flag.StringVar(&environment, "env", "", "")
	flag.IntVar(&repository, "repo", 0, "")
	flag.IntVar(&resource, "res", 0, "")
	flag.BoolVar(&test, "test", false, "")
}

func main() {
	flag.Parse()

	//create an ASClient
	var err error
	aspace, err = go_aspace.NewClient(config, environment, 20)
	if err != nil {
		panic(err)
	}

	//get the digital objects attached to a resource
	doURIs, err := aspace.GetDigitalObjectIDsForResource(repository, resource)
	if err != nil {
		panic(err)
	}

	for i := range doURIs {
		result, err := hasAeonLinks(doURIs[i])
		if err != nil {
			//log the error
			continue
		}

		if result == false {
			//skip the do
			continue
		}

		err = removeAeonLink(doURIs[i])
		if err != nil {
			log.Println(err.Error())
			continue
		}
	}
}

func removeAeonLink(string) error {
	return nil
}

func hasAeonLinks(doURI string) (bool, error) {
	repoID, doID, err := go_aspace.URISplit(doURI)
	if err != nil {
		return false, err
	}

	do, err := aspace.GetDigitalObject(repoID, doID)
	if err != nil {
		return false, err
	}

	if len(do.FileVersions) == 1 && aeonPtn.MatchString(do.FileVersions[0].FileURI) {
		return true, nil
	}

	return false, nil
}
