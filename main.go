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
	aeonPtn     = regexp.MustCompile("^https://aeon.library.nyu.edu")
	iFile       string
)

const scriptVersion = "v1.0.0"

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

		removeInstances := []int{}
		removeDOs := []string{}

		//iterate through the instances
		for i, instance := range ao.Instances {
			//check if the instance is a digital object
			if instance.InstanceType == "digital_object" {
				//iterate through the digital object map
				for _, doURI := range instance.DigitalObject {
					//check for aeon link objects
					res, err := hasAeonLinks(doURI)
					if err != nil {
						log.Printf("[ERROR] %s", err.Error())
						continue
					}
					if res {
						removeInstances = append(removeInstances, i)
						removeDOs = append(removeDOs, doURI)
					}
				}
			}
		}

		if len(removeInstances) > 0 {
			for _, ii := range removeInstances {
				do := ao.Instances[ii].DigitalObject
				log.Printf("[INFO] unlinking of do %s from ao %s", do["ref"], ao.URI)

				if test == true {
					log.Printf("[INFO] test-mode -- skipping unlinking of do %s from ao %s", do["ref"], ao.URI)
					continue
				}

				msg, err := unlinkDO(repoId, aoID, ao, ii)
				if err != nil {
					log.Printf(fmt.Sprintf("[ERROR] %s", err.Error()))
					continue
				}
				log.Printf(fmt.Sprintf("[INFO] %s", msg))
			}
		}

		if len(removeDOs) > 0 {
			for _, doURI := range removeDOs {
				log.Printf("[INFO] deleting %s", doURI)
				if err != nil {
					log.Printf("[ERROR] %s", err.Error())
					continue
				}

				if test == true {
					log.Printf("[INFO] test-mode -- skipping delete of %s\n", doURI)
					continue
				}

				msg, err := deleteDO(doURI)
				if err != nil {
					log.Printf("[ERROR] %s", err.Error())
					continue
				}
				log.Printf("[INFO] %s", *msg)

			}
		}

	}
}

func unlinkDO(repoID int, aoID int, ao go_aspace.ArchivalObject, ii int) (string, error) {
	//remove the instance from instance slice
	oLength := len(ao.Instances)
	ao.Instances = append(ao.Instances[:ii], ao.Instances[ii+1:]...)
	nLength := len(ao.Instances)

	//check that the instance was removed
	if nLength != oLength-1 {
		return "", fmt.Errorf("%d is not equal to %d -1", nLength, oLength)
	}

	msg, err := aspace.UpdateArchivalObject(repoID, aoID, ao)
	if err != nil {
		return "", err
	}

	return msg, nil
}

func deleteDO(doURI string) (*string, error) {
	repoID, doID, err := go_aspace.URISplit(doURI)
	if err != nil {
		return nil, err
	}

	if test != true {
		do, err := aspace.GetDigitalObject(repoID, doID)
		if err != nil {
			return nil, err
		}

		msg, err := aspace.DeleteDigitalObject(repoID, doID)
		if err != nil {
			return nil, err
		}
		msg = strings.ReplaceAll(msg, "\n", "")
		msg = fmt.Sprintf("%s {\"file-uri\"=\"%s\",\"title\"=\"%s\"}", msg, do.FileVersions[0].FileURI, do.Title)
		return &msg, nil
	} else {
		msg := "test-mode, skipping deletion of " + doURI
		return &msg, nil
	}

}

// check that a digital object only has 1 fileversion and that it contains an aeon link
func hasAeonLinks(doURI string) (bool, error) {
	repoID, doID, err := go_aspace.URISplit(doURI)
	if err != nil {
		return false, err
	}

	do, err := aspace.GetDigitalObject(repoID, doID)
	if err != nil {
		return false, err
	}

	uri := do.FileVersions[0].FileURI
	if len(do.FileVersions) == 1 && aeonPtn.MatchString(uri) {
		return true, nil
	}

	return false, nil
}
