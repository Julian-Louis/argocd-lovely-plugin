package main

import (
	"errors"
	"fmt"
	"github.com/otiai10/copy"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
)

var processors = []Processor{
	helmProcessor{},
	kustomizeProcessor{},
	yamlProcessor{},
	pluginProcessor{},
}

// Collection is a list of sub-applications making up this application
type Collection struct {
	baseDir string
	dirs    PackageDirectories
}

func (c *Collection) scanFile(path string, info os.DirEntry, err error) error {
	if err != nil {
		return err
	}
	if info.IsDir() {
		if c.dirs.KnownSubDirectory(path) {
			// We don't allow subdirectories of paths with yaml in
			// to be packages in their own right
			return filepath.SkipDir
		}
		return nil
	}
	yamlRegexp := regexp.MustCompile(`\.ya?ml$`)
	dir := filepath.Dir(path)
	if yamlRegexp.MatchString(path) {
		c.dirs.AddDirectory(dir)
	}
	return nil
}

func (c *Collection) scanDir(path string) error {
	return filepath.WalkDir(path, c.scanFile)
}

func (c *Collection) processAllDirs() (string, error) {
	result := ""
	for _, path := range c.dirs.GetPackages() {
		output, err := c.processOneDir(path)
		if err != nil {
			return "", err
		}
		result += output
	}
	return result, nil
}

func (c *Collection) processOneDir(path string) (string, error) {
	var result *string
	pre := preProcessor{}
	if pre.enabled(c.baseDir, path) {
		err := pre.generate(c.baseDir, path)
		if err != nil {
			return "", err
		}
	}
	for _, processor := range processors {
		if processor.enabled(c.baseDir, path) {
			out, err := processor.generate(result, c.baseDir, path)
			if err != nil {
				return "", err
			}
			result = out
		}
	}
	return *result, nil
}

// We copy the directory in case we patch some of the files for kustomize or helm
func (c *Collection) makeTmpCopy(path string) (string, error) {
	tmpPath, err := ioutil.TempDir(os.TempDir(), "lovely-plugin-")
	if err != nil {
		return tmpPath, err
	}
	err = os.RemoveAll(tmpPath)
	if err != nil {
		return tmpPath, err
	}
	err = copy.Copy(path, tmpPath)
	return tmpPath, err
}

func (c *Collection) gitClean(path string) error {
	chkout := exec.Command("git", "checkout", "HEAD", "--", ".")
	chkout.Dir = path
	_, err := chkout.Output()
	if err != nil {
		return err
	}
	log.Printf("Cleaning %s", path)
	clean := exec.Command("git", "clean", "-fdx", ".")
	clean.Dir = path
	_, err = clean.Output()
	return err
}

// Ensure we have a clean working copy
// ArgoCD doesn't guarantee us an unpatched copy when we run
func (c *Collection) ensureClean(path string) (string, func(string) error, error) {
	if AllowGitCheckout() {
		return path, c.gitClean, c.gitClean(path)
	} else {
		newPath, err := c.makeTmpCopy(path)
		return newPath, os.RemoveAll, err
	}
}

func (c *Collection) doAllDirs(path string) (string, error) {
	workingPath, cleanup, err := c.ensureClean(path)
	if err != nil {
		log.Fatal(err)
	}
	c.baseDir = workingPath
	defer cleanup(workingPath)
	err = c.scanDir(workingPath)
	if err != nil {
		log.Fatal(err)
	}
	output, err := c.processAllDirs()
	if err != nil {
		log.Fatal(err)
	}
	return output, err
}

func parseArgs() (bool, error) {
	if len(os.Args[1:]) == 0 {
		return false, nil
	}
	if len(os.Args[1:]) > 1 {
		return false, errors.New("Too many arguments. Only one optional argument allowed of 'init'")
	}
	if os.Args[1] == `init` {
		return true, nil
	}
	return false, errors.New("Invalid argument. Only one optional argument allowed of 'init'")
}

func main() {
	initMode, err := parseArgs()
	if err != nil {
		log.Fatal(err)
	}
	if initMode {
		return
	}
	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	c := Collection{}
	output, err := c.doAllDirs(dir)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(output)
}
