package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/bombsimon/logrusr/v3"
	"github.com/go-logr/logr"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

const (
	defaultImageDay = 1
)

var (
	Version   = "unreleased"
	BuildDate = "now"
	//Should be replaced with an env var
	// 0-6, Sunday=0
	imageDay = func(defaultValue int) int {
		if str, ok := os.LookupEnv("FG_IMAGE_DAY"); ok {
			if d, err := strconv.Atoi(str); err != nil {
				fmt.Printf("failed to parse $%s: %v", "FG_IMAGE_DAY\n", err)
			} else {
				return d
			}
		}
		return defaultValue
	}(defaultImageDay)
)

type tagHandler struct {
	log logr.Logger
}

func printVersion(log logr.Logger) {
	log.Info("App", "Version", Version, "Build Date", BuildDate)
	log.Info("Go", "Version", runtime.Version(), "OS", runtime.GOOS, "ARCH", runtime.GOARCH)
}

func getLogger() logr.Logger {
	formatter := new(logrus.TextFormatter)
	formatter.DisableTimestamp = true
	logger := logrus.New()
	logger.SetFormatter(formatter)

	return logrusr.New(logger)
}

func main() {

	log := getLogger()

	printVersion(log)

	r := router(log)

	srv := &http.Server{
		Handler:      r,
		WriteTimeout: 30 * time.Second,
		ReadTimeout:  30 * time.Second,
		Addr:         ":8080",
	}
	err := srv.ListenAndServe()
	if err != nil {
		log.Error(err, "HTTP server error")
	}
}

func (t *tagHandler) getWindow(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	day, err := strconv.Atoi(vars["day"])
	if err != nil {
		w.WriteHeader(http.StatusMethodNotAllowed)
		errMessage := fmt.Sprintf("error parsing day")
		_, err := io.WriteString(w, errMessage)
		t.log.Error(err, errMessage)
		return
	}

	hour, err := strconv.Atoi(vars["hour"])
	if err != nil {
		w.WriteHeader(http.StatusMethodNotAllowed)
		errMessage := fmt.Sprintf("error parsing hour")
		_, err := io.WriteString(w, errMessage)
		fmt.Println(err, errMessage)
		return
	}

	tag := t.getTag(day, hour, time.Now())

	t.log.Info("serving", "tag", tag)

	http.Redirect(w, r, "/tag/"+tag, http.StatusFound)
}

func router(log logr.Logger) *mux.Router {
	r := mux.NewRouter()
	h := tagHandler{log: log}
	r.HandleFunc("/window/{day:[0-6]}/{hour:2[0-3]|[01][0-9]}", h.getWindow).Methods("GET")
	r.HandleFunc("/alive", h.alive).Methods("GET")
	return r
}

func (t *tagHandler) alive(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err := io.WriteString(w, `{"status":"ok"}`)
	if err != nil {
		t.log.Error(err, "failed to write aliveness check")
	}
}

func (t *tagHandler) getTag(day int, hour int, currentTime time.Time) string {

	//some input checking, if the hour and day are wrong simply return the previous tag
	//this should never get hit if the call comes from the correctly configured gorilla mux
	if day > 6 || day < 0 || hour > 23 && hour < 0 {
		return t.getPreviousTag(currentTime)
	}

	//Let's convert the diffs to the maintenance window to time.Durations, so we can easily
	//calculate if we're already past that point or not.
	diffDays := 24 * time.Duration((day - int(currentTime.Weekday()))) * time.Hour
	diffHours := time.Duration(hour-currentTime.Hour()) * time.Hour
	windowTime := currentTime.Add(diffDays + diffHours)

	if currentTime.After(windowTime) || currentTime.Equal(windowTime) {
		return t.getCurrentTag(currentTime)
	}

	return t.getPreviousTag(currentTime)
}

func (t *tagHandler) getCurrentTag(currentTime time.Time) string {
	return t.getImageDate(currentTime)
}

//getPreviousTag returns last week's tag according to the imageDay
func (t *tagHandler) getPreviousTag(currentTime time.Time) string {
	date := currentTime.AddDate(0, 0, -7)

	return t.getImageDate(date)
}

func (t *tagHandler) getImageDate(date time.Time) string {
	for int(date.Weekday()) != imageDay {
		date = date.AddDate(0, 0, -1)
	}

	return fmt.Sprintf("%d%02d%02d", date.Year(), date.Month(), date.Day())
}
