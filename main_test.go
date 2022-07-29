package main

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			want: "20200525",
			args: args{
				day:         6,
				hour:        23,
				currentTime: time.Date(2020, time.June, 5, 22, 0, 0, 0, time.Local),
				imageDay:    1,
			},
		},
		{
			name: "window not yet met and date before image day",
			want: "20200525",
			args: args{
				day:         6,
				hour:        23,
				currentTime: time.Date(2020, time.May, 31, 22, 0, 0, 0, time.Local),
				imageDay:    1,
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

			th := tagHandler{
				log: testr.New(t),
			}

			imageDay = tt.args.imageDay

			tag, err := th.getTag(tt.args.day, tt.args.hour, tt.args.currentTime)
			require.NoError(t, err)
			require.Equal(t, tt.want, tag, "wrong value from getTag()")
		})
	}
}

func Test_getTag_TagAlwaysIncreases(t *testing.T) {
	// The tag should always increase as time progresses.
	imageDay = 1
	weekday := int(time.Tuesday)
	hour := 22

	th := tagHandler{
		log: testr.New(t),
	}

	stop, err := time.Parse(time.RFC3339, "2022-07-29T10:00:00+02:00")
	require.NoError(t, err)

	prevTimeFromTag := time.Time{}
	for c := stop.AddDate(0, -1, 0); c.Before(stop); c = c.Add(time.Hour) {
		tag, err := th.getTag(weekday, hour, c)
		assert.NoError(t, err)
		t.Logf("TIME: (%s/%v)\t%s TAG: %s", c.Format("Mon"), int(c.Weekday()), c.Format(time.RFC3339), tag)

		timeFromTag, err := time.ParseInLocation(tagFormat, tag, time.Local)
		require.NoError(t, err)
		assert.Truef(t,
			(timeFromTag.Equal(prevTimeFromTag) || timeFromTag.After(prevTimeFromTag)),
			"tag `%s` should be at the same time or after previous tag `%s`, current time: %s",
			timeFromTag.Format(tagFormat), prevTimeFromTag.Format(tagFormat), c.Format(time.RFC3339))

		prevTimeFromTag = timeFromTag
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

			router(testr.New(t)).ServeHTTP(rr, req)

			if rr.Code != tt.want.status {
				t.Errorf("getWindow() = %v, want %v", rr.Code, tt.want.status)
			}

			body, err := ioutil.ReadAll(rr.Body)
			require.NoError(t, err)
			require.Contains(t, string(body), tt.want.body, "wrong value from getWindow()")
		})
	}
}
