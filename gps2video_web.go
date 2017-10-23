package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/koding/multiconfig"
	"github.com/teawater/go.strava"
)

type Server struct {
	GPS2VideoDir   string `required:"true"`
	ClientId       int    `required:"true"`
	ClientSecret   string `required:"true"`
	Port           int    `default:"0"` //0 means auto
	DomainName     string `required:"true"`
	DomainDir      string `default:"/"`
	SSL            bool   `default:"False"`
	SSLcertFile    string `default:""`
	SSLkeyFile     string `default:""`
	WorkDir        string `default:"./work/"`
	Ffmpeg         string `default:"ffmpeg"`
	Google_map_key string `required:"true"`
	SmtpServer     string `default:""`
	SmtpPort       int    `default:"25"`
	SmtpEmail      string `default:""`
	SmtpPassword   string `default:""`
}

var serverConf *Server
var baseURL string
var authenticator *strava.OAuthAuthenticator

func main() {
	args_len := len(os.Args)
	if args_len != 2 && args_len != 3 {
		log.Fatalf("Usage: %s config [log]", os.Args[0])
	}

	if args_len == 3 {
		logFile, err := os.Create(os.Args[2])
		if err != nil {
			log.Fatalln("os.Create", os.Args[2], err)
		}
		defer logFile.Close()
		log.SetOutput(logFile)
	}

	//Setup serverConf
	m := multiconfig.NewWithPath(os.Args[1])
	serverConf = new(Server)
	m.MustLoad(serverConf)
	if serverConf.Port == 0 {
		if serverConf.SSL {
			serverConf.Port = 443
		} else {
			serverConf.Port = 80
		}
	}
	if serverConf.SSL {
		if serverConf.SSLcertFile == "" || serverConf.SSLkeyFile == "" {
			log.Fatalln("If 'SSL' is true, field 'SSLcertFile' and 'SSLkeyFile' is required")
		}
	}

	users.Init(serverConf.WorkDir)

	httpInit()

	if serverConf.SSL {
		log.Fatal(http.ListenAndServeTLS(fmt.Sprintf(":%d", serverConf.Port), serverConf.SSLcertFile, serverConf.SSLkeyFile, nil))
	} else {
		log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", serverConf.Port), nil))
	}
}

func dir_check_creat(dir string, remove_wrong bool) (err error) {
	var fi os.FileInfo
	fi, err = os.Stat(dir)

	if err == nil {
		if fi.IsDir() {
			return
		}
		if remove_wrong {
			os.RemoveAll(dir)
		} else {
			err = errors.New(dir + " is not a directory.")
			return
		}
	}

	if err == nil || os.IsNotExist(err) {
		err = os.Mkdir(dir, os.FileMode(0700))
	}

	return
}

func fileIsExist(dir string) (exist bool, err error) {
	_, err = os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			err = nil
			exist = false
		}
		return
	}

	exist = true
	return
}
