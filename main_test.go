package main

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_getTag(t *testing.T) {
	type args struct {
		day         int
		hour        int
		currentTime time.Time
		imageDay    int
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "monday, same month",
			want: "20200601",
			args: args{
				day:         2,
				hour:        23,
				currentTime: time.Date(2020, time.June, 5, 22, 0, 0, 0, time.Local),
				imageDay:    1,
			},
		},
		{
			name: "sunday, previous month",
			want: "20200531",
			args: args{
				day:         2,
				hour:        23,
				currentTime: time.Date(2020, time.June, 5, 22, 0, 0, 0, time.Local),
				imageDay:    0,
			},
		},
		{
			name: "window not yet met",
			want: "20200524",
			args: args{
				day:         6,
				hour:        23,
				currentTime: time.Date(2020, time.June, 5, 22, 0, 0, 0, time.Local),
				imageDay:    0,
			},
		},
		{
			name: "window just met",
			want: "20200601",
			args: args{
				day:         5,
				hour:        22,
				currentTime: time.Date(2020, time.June, 5, 22, 0, 0, 0, time.Local),
				imageDay:    1,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			imageDay = tt.args.imageDay

			if got := getTag(tt.args.day, tt.args.hour, tt.args.currentTime); got != tt.want {
				t.Errorf("getTag() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getWindow(t *testing.T) {
	type args struct {
		method string
		URL    string
	}
	type want struct {
		status int
		body   string
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "302",
			args: args{
				method: "GET",
				URL:    "/window/5/22",
			},
			want: want{
				status: http.StatusFound,
				body:   "Found",
			},
		},
		{
			name: "wrong method",
			args: args{
				method: "POST",
				URL:    "/window/5/22",
			},
			want: want{
				status: http.StatusMethodNotAllowed,
				body:   "",
			},
		},
		{
			name: "invalid DoW",
			args: args{
				method: "GET",
				URL:    "/window/11/22",
			},
			want: want{
				status: http.StatusNotFound,
				body:   "",
			},
		},
		{
			name: "invalid hour",
			args: args{
				method: "GET",
				URL:    "/window/5/29",
			},
			want: want{
				status: http.StatusNotFound,
				body:   "",
			},
		},
		{
			name: "daytime morning",
			args: args{
				method: "GET",
				URL:    "/window/5/09",
			},
			want: want{
				status: http.StatusFound,
				body:   "Found",
			},
		},
		{
			name: "daytime afternoon",
			args: args{
				method: "GET",
				URL:    "/window/5/14",
			},
			want: want{
				status: http.StatusFound,
				body:   "Found",
			},
		},
		{
			name: "morning single digit",
			args: args{
				method: "GET",
				URL:    "/window/5/4",
			},
			want: want{
				status: http.StatusNotFound,
				body:   "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			rr := httptest.NewRecorder()
			req, err := http.NewRequest(tt.args.method, tt.args.URL, nil)
			assert.NoError(t, err)

			router().ServeHTTP(rr, req)

			if rr.Code != tt.want.status {
				t.Errorf("getWindow() = %v, want %v", rr.Code, tt.want.status)
			}

			body, err := ioutil.ReadAll(rr.Body)
			assert.NoError(t, err)

			if !strings.Contains(string(body), tt.want.body) {
				t.Errorf("getWindow() = %s, want %s", body, tt.want.body)
			}

		})
	}
}
