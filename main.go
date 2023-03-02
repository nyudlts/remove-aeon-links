package main

import (
	"bufio"
	"flag"
	"fmt"
	go_aspace "github.com/nyudlts/go-aspace"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	aspace          *go_aspace.ASClient
	config          string
	environment     string
	test            bool
	aeonPtn         = regexp.MustCompile("^https://aeon.library.nyu.edu")
	iFile           string
	dosRemovedCount int
)

type DORef struct {
	URI   string
	Index int
}

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
	fmt.Printf("[INFO] client created for %s\n", aspace.RootURL)
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
	fmt.Printf("[INFO] running `remove-aeon-links` %s\n", scriptVersion)

	setClient()

	inFile, err := os.Open(iFile)
	if err != nil {
		panic(err)
	}
	defer inFile.Close()
	absFilepath, _ := filepath.Abs(iFile)
	log.Printf("[INFO] using source file `%s`", absFilepath)
	fmt.Printf("[INFO] using source file `%s`\n", absFilepath)

	if test {
		log.Printf("[INFO] running in test mode, no dos will be unlinked or deleted")
		fmt.Printf("[INFO] running in test mode, no dos will be unlinked or deleted\n", absFilepath)
	}

	dosRemovedCount = 0
	scanner := bufio.NewScanner(inFile)
	for scanner.Scan() {
		log.Printf("[INFO] checking %s for aeon links", scanner.Text())
		fmt.Printf("[INFO] checking %s for aeon links\n", scanner.Text())
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

		DORefs := []DORef{}

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
						DORefs = append(DORefs, DORef{URI: doURI, Index: i})
					}
				}
			}
		}

		//if there are no DOs to remove, continue to next ao
		if len(DORefs) < 1 {
			log.Printf("[INFO] no Aeon Links found in %s", scanner.Text())
			continue
		}

		for _, doRef := range DORefs {
			//unlink the DO from the AO
			log.Printf("[INFO] unlinking of do %s from ao %s", doRef.URI, ao.URI)
			if test == true {
				log.Printf("[INFO] test-mode -- skipping unlinking of do %s from ao %s", doRef.URI, ao.URI)
			} else {
				msg, err := unlinkDO(repoId, aoID, ao, doRef.Index)
				if err != nil {
					log.Printf(fmt.Sprintf("[ERROR] %s", err.Error()))
					continue
				}
				log.Printf(fmt.Sprintf("[INFO] %s", msg))
			}

			//delete the DO
			log.Printf("[INFO] deleting %s", doRef.URI)
			if test == true {
				log.Printf("[INFO] test-mode -- skipping delete of %s\n", doRef.URI)
				continue
			} else {
				msg, err := deleteDO(doRef.URI)
				if err != nil {
					log.Printf("[ERROR] %s", err.Error())
					continue
				}
				log.Printf("[INFO] %s", *msg)
			}

			dosRemovedCount = dosRemovedCount + 1
		}

	}
	log.Printf("[INFO] remove-aeon-links complete, %d digital objects unlinked and removed", dosRemovedCount)
	fmt.Printf("[INFO] remove-aeon-links complete, %d digital objects unlinked and removed\n", dosRemovedCount)
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
