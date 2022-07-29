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
	defaultImageDay = time.Monday
	tagFormat       = "20060102"
)

var (
	Version   = "unreleased"
	BuildDate = "now"
)

type tagHandler struct {
	log      logr.Logger
	imageDay int
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
		fullErr := fmt.Errorf("error parsing day: %w", err)
		t.log.Error(fullErr, "failed to parse day")
		http.Error(w, fullErr.Error(), http.StatusUnprocessableEntity)
		return
	}

	hour, err := strconv.Atoi(vars["hour"])
	if err != nil {
		fullErr := fmt.Errorf("error parsing hour: %w", err)
		t.log.Error(fullErr, "failed to parse hour")
		http.Error(w, fullErr.Error(), http.StatusUnprocessableEntity)
		return
	}

	tag, err := t.getTag(day, hour, time.Now())
	if err != nil {
		fullErr := fmt.Errorf("error calculating tag: %w", err)
		t.log.Error(fullErr, "failed to calculate tag")
		http.Error(w, fullErr.Error(), http.StatusUnprocessableEntity)
		return
	}

	t.log.Info("serving", "tag", tag)

	http.Redirect(w, r, "/tag/"+tag, http.StatusFound)
}

func router(log logr.Logger) *mux.Router {
	r := mux.NewRouter()
	h := tagHandler{
		log:      log,
		imageDay: imageDayFromEnv(defaultImageDay),
	}
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

func (t *tagHandler) getTag(day int, hour int, currentTime time.Time) (string, error) {
	//this should never get hit if the call comes from the correctly configured gorilla mux
	if day > 6 || day < 0 || hour > 23 && hour < 0 {
		return "", fmt.Errorf("invalid day (%d) or hour (%d)", day, hour)
	}

	//Let's convert the diffs to the maintenance window to time.Durations, so we can easily
	//calculate if we're already past that point or not.
	diffDays := 24 * time.Duration((day - int(currentTime.Weekday()))) * time.Hour
	diffHours := time.Duration(hour-currentTime.Hour()) * time.Hour
	windowTime := currentTime.Add(diffDays + diffHours)

	if currentTime.After(windowTime) || currentTime.Equal(windowTime) {
		return t.getCurrentTag(currentTime), nil
	}

	return t.getPreviousTag(currentTime), nil
}

func (t *tagHandler) getCurrentTag(currentTime time.Time) string {
	return t.floorToImageDay(currentTime).Format(tagFormat)
}

// getPreviousTag returns last week's tag according to the imageDay
func (t *tagHandler) getPreviousTag(currentTime time.Time) string {
	if int(currentTime.Weekday()) < t.imageDay {
		return t.getCurrentTag(currentTime)
	}
	return t.getCurrentTag(currentTime.AddDate(0, 0, -7))
}

func (t *tagHandler) floorToImageDay(date time.Time) time.Time {
	for int(date.Weekday()) != t.imageDay {
		date = date.AddDate(0, 0, -1)
	}
	return date
}

// imageDayFromEnv returns the imageDay from the environment variable FG_IMAGE_DAY.
// Values are from 0-6 where Sunday=0
func imageDayFromEnv(defaultValue time.Weekday) int {
	if str, ok := os.LookupEnv("FG_IMAGE_DAY"); ok {
		if d, err := strconv.Atoi(str); err != nil {
			fmt.Printf("failed to parse $%s: %v", "FG_IMAGE_DAY\n", err)
		} else {
			return d
		}
	}
	return int(defaultValue)
}
