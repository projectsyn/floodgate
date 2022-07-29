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

func getLogger() logr.Logger {
	formatter := new(logrus.TextFormatter)
	formatter.DisableTimestamp = true
	logger := logrus.New()
	logger.SetFormatter(formatter)

	return logrusr.New(logger)
}

func main() {
	log := getLogger()
	log.Info("App", "Version", Version, "Build Date", BuildDate)
	log.Info("Go", "Version", runtime.Version(), "OS", runtime.GOOS, "ARCH", runtime.GOARCH)

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

func router(log logr.Logger) *mux.Router {
	r := mux.NewRouter()
	h := handler{
		log:      log,
		imageDay: imageDayFromEnv(defaultImageDay),
	}
	r.HandleFunc("/window/{day:[0-6]}/{hour:2[0-3]|[01][0-9]}", h.getWindow).Methods("GET")
	r.HandleFunc("/alive", h.alive).Methods("GET")
	return r
}

type handler struct {
	log      logr.Logger
	imageDay time.Weekday
}

func (t *handler) getWindow(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	day, err := strconv.Atoi(vars["day"])
	if err != nil {
		t.error(w, fmt.Errorf("error parsing day: %w", err), http.StatusUnprocessableEntity)
		return
	}

	hour, err := strconv.Atoi(vars["hour"])
	if err != nil {
		t.error(w, fmt.Errorf("error parsing hour: %w", err), http.StatusUnprocessableEntity)
		return
	}

	currentTime := time.Now()
	tag, err := getTag(t.imageDay, day, hour, currentTime)
	if err != nil {
		t.error(w, fmt.Errorf("error calculating tag: %w", err), http.StatusUnprocessableEntity)
		return
	}

	t.log.Info("serving",
		"current_time", currentTime.Format(time.RFC3339),
		"requested_day", day,
		"requested_hour", hour,
		"tag", tag,
	)

	http.Redirect(w, r, "/tag/"+tag, http.StatusFound)
}

func (t *handler) alive(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err := io.WriteString(w, `{"status":"ok"}`)
	if err != nil {
		t.log.Error(err, "failed to write aliveness check")
	}
}

func (t *handler) error(w http.ResponseWriter, err error, code int) {
	t.log.Error(err, "error handling request", "code", code)
	http.Error(w, err.Error(), code)
}

func getTag(imageDay time.Weekday, day, hour int, currentTime time.Time) (string, error) {
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
		return getCurrentTag(imageDay, currentTime), nil
	}

	return getPreviousTag(imageDay, currentTime), nil
}

func getCurrentTag(imageDay time.Weekday, currentTime time.Time) string {
	return floorToImageDay(imageDay, currentTime).Format(tagFormat)
}

// getPreviousTag returns last week's tag according to the imageDay
func getPreviousTag(imageDay time.Weekday, currentTime time.Time) string {
	if currentTime.Weekday() < imageDay {
		return getCurrentTag(imageDay, currentTime)
	}
	return getCurrentTag(imageDay, currentTime.AddDate(0, 0, -7))
}

func floorToImageDay(imageDay time.Weekday, date time.Time) time.Time {
	for date.Weekday() != imageDay {
		date = date.AddDate(0, 0, -1)
	}
	return date
}

// imageDayFromEnv returns the imageDay from the environment variable FG_IMAGE_DAY.
// Values are from 0-6 where Sunday=0
func imageDayFromEnv(defaultValue time.Weekday) time.Weekday {
	if str, ok := os.LookupEnv("FG_IMAGE_DAY"); ok {
		if d, err := strconv.Atoi(str); err != nil {
			fmt.Printf("failed to parse $%s: %v", "FG_IMAGE_DAY", err)
		} else if d < 0 || d > 6 {
			fmt.Printf("$%s must be between 0 and 6\n", "FG_IMAGE_DAY")
		} else {
			return time.Weekday(d)
		}
	}
	return defaultValue
}
