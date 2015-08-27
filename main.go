package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/dustin/go-humanize"
	"github.com/julienschmidt/httprouter"
)

var t torrent.Torrent
var seed *bool
var vlc *bool

func main() {
	var client *torrent.Client

	seed = flag.Bool("seed", true, "Seed after finished downloading")
	vlc = flag.Bool("vlc", false, "Open vlc to play the file")
	flag.Parse()
	if len(flag.Args()) == 0 {
		usage()
		os.Exit(1)
	}

	client, err := torrent.NewClient(&torrent.Config{
		DataDir:  os.TempDir(),
		NoUpload: !(*seed),
	})

	if err != nil {
		log.Fatal(err)
		os.Exit(3)
	}

	if t, err = client.AddMagnet(flag.Arg(0)); err != nil {
		log.Fatal(err)
		os.Exit(2)
	}

	// Start downloading files.
	go func() {
		<-t.GotInfo()
		t.DownloadAll()
	}()

	// Http handler.
	go func() {
		router := httprouter.New()
		router.GET("/", getFile)
		log.Fatal(http.ListenAndServe(":8080", router))
	}()

	if *vlc {
		go func() {
			for !readyForPlayback() {
				time.Sleep(time.Second)
				log.Printf("Not playing")
			}
			log.Printf("Playing in vlc")

			//out, err := exec.Command("/usr/bin/open -a vlc \"http://localhost:8080\"").Output()
			err := exec.Command("/usr/bin/open", "-a", "-vlc", "http://localhost:8080").Run()
			log.Printf("err = %#v\n", err)
		}()
	}

	// Cli render loop.
	for true {
		render()
		time.Sleep(time.Second)
	}
}

func readyForPlayback() bool {
	percentage := float64(t.BytesCompleted()) / float64(t.Length())

	return percentage > 0.05
}

func render() {
	percentage := float64(t.BytesCompleted()) / float64(t.Length()) * 100
	complete := humanize.Bytes(uint64(t.BytesCompleted()))
	size := humanize.Bytes(uint64(t.Length()))
	connections := len(t.Conns)

	print("\033[H\033[2J")
	fmt.Println(t.Name())
	fmt.Println("=============================================================")
	fmt.Printf("%s/%s %.2f%%\n", complete, size, percentage)
	fmt.Printf("Connections: %d\n", connections)
}

func usage() {
	flag.Usage()
}

func getLargestFile() torrent.File {
	var target torrent.File
	var maxSize int64

	for _, file := range t.Files() {
		if maxSize < file.Length() {
			maxSize = file.Length()
			target = file
		}
	}

	return target
}

func getFile(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	target := getLargestFile()
	entry := NewFileReader(target)
	defer entry.Close()

	http.ServeContent(w, r, target.DisplayPath(), time.Now(), entry)
}