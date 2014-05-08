package main

import (
	"flag"
	"fmt"
	"github.com/howeyc/fsnotify"
	"log"
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	DirectoryMode os.FileMode
	FileMode      os.FileMode
}

func checkMode(path string, info os.FileInfo, config Config) error {
	targetMode := config.FileMode
	if info.IsDir() {
		targetMode = config.DirectoryMode
	}
	if info.Mode().Perm() != targetMode {
		log.Printf("%s has permissions %s, changing to %s\n", path, info.Mode().String(), targetMode.String())
		err := os.Chmod(path, targetMode)
		if err != nil {
			return err
		}
	}

	return nil
}

func getWalker(config Config) filepath.WalkFunc {
	w := func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			err := watchDir(path, config)
			if err != nil {
				log.Printf("Error starting watcher on %s: %s\n", path, err.Error())
			}
		}
		err = checkMode(path, info, config)
		if err != nil {
			log.Printf("Error fixing permissions for %s: %s\n", path, err.Error())
		}
		return nil
	}
	return w
}

func watchDir(path string, config Config) error {
	log.Println("Watching directory", path)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case ev := <-watcher.Event:
				if ev.IsCreate() {
					info, err := os.Stat(ev.Name)
					if err != nil {
						log.Printf("Error calling stat() on %s: %s", ev.Name, err.Error())
						continue
					}

					if info.IsDir() {
						err = watchDir(ev.Name, config)
						if err != nil {
							log.Printf("Error starting a watcher on %s\n", err.Error())
						}
						err = checkMode(ev.Name, info, config)
						if err != nil {
							log.Printf("Could not fix permissions for %s: %s", ev.Name, err.Error())
						}
						// check permissions on all files inside it and watch any subdirectories
						filepath.Walk(ev.Name, getWalker(config))

					} else {
						err = checkMode(ev.Name, info, config)
						if err != nil {
							log.Printf("Could not fix permissions for %s: %s", ev.Name, err.Error())
						}
					}
				}
			case err := <-watcher.Error:
				log.Printf("Watcher err: %s\n", err.Error())
			}
		}
	}()

	watcher.Watch(path)
	return nil
}

func usage() {
	fmt.Println("Usage: permd <directory to watch>")
}

func main() {
	var dirMode = flag.Int("dirMode", 0755, "The desired directory mode")
	var fileMode = flag.Int("fileMode", 0755, "The desired file mode")
	var setGid = flag.Bool("setGid", false, "")
	var setUid = flag.Bool("setUid", false, "")

	flag.Parse()
	args := flag.Args()
	if len(args) < 1 {
		usage()
		return
	}

	config := Config{os.FileMode(*dirMode), os.FileMode(*fileMode)}
	if *setGid {
		config.DirectoryMode = config.DirectoryMode | os.ModeSetgid
		config.FileMode = config.FileMode | os.ModeSetgid
	}
	if *setUid {
		config.DirectoryMode = config.DirectoryMode | os.ModeSetuid
		config.FileMode = config.FileMode | os.ModeSetuid
	}

	filepath.Walk(args[0], getWalker(config))
	for {
		time.Sleep(1 * time.Second)
	}
}
