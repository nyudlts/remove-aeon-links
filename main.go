package main

import (
	"bufio"
	"flag"
	"fmt"
	go_aspace "github.com/nyudlts/go-aspace"
	"log"
	"os"
	"regexp"
	"strings"
)

var (
	aspace      *go_aspace.ASClient
	config      string
	environment string
	repository  int
	resource    int
	test        bool
	aeonPtn     = regexp.MustCompile("https://aeon.library.nyu.edu")
	iFile       string
)

const scriptVersion = "v0.1.0"

func init() {
	flag.StringVar(&config, "config", "", "")
	flag.StringVar(&environment, "env", "", "")
	flag.StringVar(&iFile, "file", "", "")
	flag.BoolVar(&test, "test", false, "")
}

func setClient() {
	//create an ASClient
	var err error
	aspace, err = go_aspace.NewClient(config, environment, 20)
	if err != nil {
		log.Printf("[ERROR] %s", err.Error())
		os.Exit(1)
	}

	log.Printf("[INFO] client created for %s", aspace.RootURL)
}

func main() {
	flag.Parse()
	logFile, err := os.Create("remove-aeon-links.log")
	if err != nil {
		panic(err)
	}
	defer logFile.Close()
	log.SetOutput(logFile)

	log.Printf("[INFO] running `remove-aeon-links` %s", scriptVersion)
	setClient()

	inFile, err := os.Open(iFile)
	if err != nil {
		panic(err)
	}
	defer inFile.Close()

	scanner := bufio.NewScanner(inFile)
	for scanner.Scan() {
		log.Printf("[INFO] checking %s for aeon links", scanner.Text())
		repoId, aoID, err := go_aspace.URISplit(scanner.Text())
		if err != nil {
			log.Printf("[ERROR] %s", err.Error())
			continue
		}

		ao, err := aspace.GetArchivalObject(repoId, aoID)
		if err != nil {
			log.Printf("[ERROR] %s", err.Error())
			continue
		}

		for _, instance := range ao.Instances {
			if instance.InstanceType == "digital_object" {
				for _, doURI := range instance.DigitalObject {
					res, uri, err := hasAeonLinks(doURI)
					if err != nil {
						log.Printf("[ERROR] %s", err.Error())
						continue
					}
					if res {
						log.Printf("[INFO] deleting %s file-uri: %s", doURI, *uri)
						msg, err := deleteDO(doURI)
						if err != nil {
							log.Printf(fmt.Sprintf("[ERROR] %s", err.Error()))
							continue
						}
						log.Printf(fmt.Sprintf("[INFO] %s", *msg))

					}
				}
			}
		}

	}
}

func deleteDO(doURI string) (*string, error) {
	repoID, doID, err := go_aspace.URISplit(doURI)
	if err != nil {
		return nil, err
	}

	if test != true {
		msg, err := aspace.DeleteDigitalObject(repoID, doID)
		if err != nil {
			return nil, err
		}
		msg = strings.ReplaceAll(msg, "\n", "")
		return &msg, nil
	} else {
		msg := "test-mode skipping deletion of " + doURI
		return &msg, nil
	}

}

// check that a digital object only has 1 fileversion and that it contains an aeon link
func hasAeonLinks(doURI string) (bool, *string, error) {
	repoID, doID, err := go_aspace.URISplit(doURI)
	if err != nil {
		return false, nil, err
	}

	do, err := aspace.GetDigitalObject(repoID, doID)
	if err != nil {
		return false, nil, err
	}

	uri := do.FileVersions[0].FileURI
	if len(do.FileVersions) == 1 && aeonPtn.MatchString(uri) {
		return true, &uri, nil
	}

	return false, nil, nil
}
