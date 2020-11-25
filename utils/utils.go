package utils

import (
	"context"
	"encoding/base64"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

type LogFile struct {
	Name string
	File *os.File
}

var (
	logDir       string
	relayVersion string
	logfiles     []*LogFile
)

func SetLogdir(ld string) {
	logDir = ld
}

func SetVersion(v string) {
	relayVersion = v
}

func GetSourceFromRequest(r *http.Request) (string, string) {
	ipAddress := r.RemoteAddr
	fwdAddress := r.Header.Get("X-Forwarded-For") // capitalisation doesn't matter
	if fwdAddress != "" {
		// Got X-Forwarded-For
		ipAddress = fwdAddress // If it's a single IP, then awesome!

		// If we got an array... grab the first IP
		ips := strings.Split(fwdAddress, ", ")
		if len(ips) > 1 {
			ipAddress = ips[0]
		}
		return ipAddress, fwdAddress
	}
	return ipAddress, ipAddress
}

func ChanToSlice(ch interface{}) interface{} {
	chv := reflect.ValueOf(ch)
	slv := reflect.MakeSlice(reflect.SliceOf(reflect.TypeOf(ch).Elem()), 0, 0)
	for {
		v, ok := chv.Recv()
		if !ok {
			return slv.Interface()
		}
		slv = reflect.Append(slv, v)
	}
}

func ResetLogFiles() {
	logfiles = nil
}

func CloseLogFiles() {
	for _, f := range logfiles {
		err := f.File.Close()
		if err != nil {
			log.Error().Msgf("Error on close log file %s:  Err: %s", f.Name, err)
		}
		log.Info().Msgf("log file %s: closed ok!", f.Name)
	}
}

func GetConsoleLogFormated(logfile string, level string) *zerolog.Logger {

	var i *os.File
	var filename string
	if len(logfile) > 0 {
		filename = logfile
		if !filepath.IsAbs(logfile) {
			filename = filepath.Join(logDir, filename)
		}
		log.Info().Msgf("trying to open log file %s .....", filename)
		file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			log.Error().Msgf("Error on opening file %s : ERROR: %s", filename, err)
		}
		i = file
		logfiles = append(logfiles, &LogFile{Name: filename, File: i})
	} else {
		i = os.Stderr
	}
	writer := zerolog.ConsoleWriter{Out: i, TimeFormat: "2006-01-02 15:04:05"}
	f := log.Output(writer)
	var logger zerolog.Logger
	switch level {
	case "panic":
		logger = f.Level(zerolog.PanicLevel)
	case "fatal":
		logger = f.Level(zerolog.FatalLevel)
	case "Error", "error":
		logger = f.Level(zerolog.ErrorLevel)
	case "warn", "warning":
		logger = f.Level(zerolog.WarnLevel)
	case "info":
		logger = f.Level(zerolog.InfoLevel)
	case "debug":
		logger = f.Level(zerolog.DebugLevel)
	default:
		logger = f.Level(zerolog.InfoLevel)
	}
	//log.Printf("---------------------------------------------")
	//log.Printf("Logger for file %s : %v: %+v\n", filename, i, &logger)
	return &logger
}

func GetUserFromRequest(r *http.Request) string {

	username := ""
	found := false
	//check authorization
	auth := strings.SplitN(r.Header.Get("Authorization"), " ", 2)

	if len(auth) != 2 || auth[0] != "Basic" {
		found = false
	} else {
		payload, _ := base64.StdEncoding.DecodeString(auth[1])
		pair := strings.SplitN(string(payload), ":", 2)
		username = pair[0]
		found = true
	}

	if !found {
		queryParams := r.URL.Query()
		username = queryParams.Get("u")
	}

	if len(username) > 0 {
		return username
	}
	return "-"

}

func AddInfluxPingHeaders(w http.ResponseWriter, version string) {
	w.Header().Add("X-InfluxDB-Version", version)
	w.Header().Add("X-Influx-SRelay-Version", relayVersion)
	w.Header().Add("Content-Length", "0")
}

func getInfluxPingHeaderInfo() string {
	clientNew := &http.Client{
		Timeout: time.Second * 5,
	}

	rep, err := http.NewRequest("GET", "http://127.0.0.1:8086/ping", nil)
	if err != nil {
		log.Error().Msgf("new http post request error: %s", err.Error())
		return ""
	}
	getResp, err := clientNew.Do(rep.WithContext(context.TODO()))
	if err != nil {
		log.Error().Msgf("new http post request error: %s", err.Error())
		return ""
	}

	defer getResp.Body.Close()

	if getResp.StatusCode == http.StatusOK || getResp.StatusCode == http.StatusNoContent {
		log.Info().Msgf("influx version: %s", getResp.Header.Get("X-Influxdb-Version"))
		return getResp.Header.Get("X-Influxdb-Version")
	} else {
		log.Error().Msgf("http status: %s", getResp.Status)
		return ""
	}
}

func GetInfluxPingVersion() string {
	version := os.Getenv("INFLUXDB_VERSION")
	if version != "" {
		return version
	}
	version = getInfluxPingHeaderInfo()
	if version == "" {
		version = "relay"
	}
	return version
}
