/*
   gAnsiWeather 0.1

   Copyright 2013 Conrad Meyer <cemeyer@uw.edu>

   https://github.com/cemeyer/gansiweather

   Released under the terms of the MIT license; see LICENSE.
*/

package main

import "encoding/json"
import "flag"
import "fmt"
import "io/ioutil"
import "net/http"
import "os"
import "syscall"
import "time"

var sflag = false
var configfile = ".config/gansiweather.conf"
var cachefile = ".config/gansiweather.cache.json"
var cachelkfile = ".config/gansiweather.cache.lk"
var home = ""

// defaults
var api_key = ""
var cache_seconds time.Duration = 10 * time.Minute
var city = "Seattle"
var state = "WA"
var units = "imperial"

type Config struct {
	ApiKey       string
	CacheSeconds uint64
	City         string
	State        string
	Units        string
}

type WData struct {
	City       string
	Conditions string
	Humidity   string
	Temp       string
}

func init() {
	flag.BoolVar(&sflag, "shell", false, "Escape ANSI color sequences so "+
		"they are ignored for length purposes")
}

func main() {
	var data WData

	err := start()
	if err != nil {
		goto out
	}

	data, err = queryWService()
	if err != nil {
		goto out
	}

	print(formatWData(data))

out:
	if err != nil {
		fmt.Print(err, "\n")
		os.Exit(1)
	}
}

func start() (err error) {
	flag.Parse()

	home = os.Getenv("HOME")
	if home == "" {
		return fmt.Errorf("Could not read $HOME")
	}

	cfile := home + "/" + configfile
	fi, err := os.Stat(cfile)
	if err != nil {
		if os.IsNotExist(err) {
			err = nil
		}
		return
	}

	if !fi.Mode().IsRegular() {
		return fmt.Errorf("%s: Config is not a regular file", cfile)
	}

	err = readConfig(cfile)
	if err != nil {
		return
	}

	return
}

func readConfig(cfile string) (err error) {
	f, err := os.Open(cfile)
	if err != nil {
		return
	}

	defer f.Close()

	b, err := ioutil.ReadAll(f)
	if err != nil {
		return
	}

	var m Config
	err = json.Unmarshal(b, &m)
	if err != nil {
		return
	}

	if m.ApiKey == "" {
		return fmt.Errorf("%s: ApiKey is not set", cfile)
	} else {
		api_key = m.ApiKey
	}
	if m.CacheSeconds > 0 {
		cache_seconds = time.Duration(m.CacheSeconds) * time.Second
	}
	if m.City != "" {
		city = m.City
	}
	if m.State != "" {
		state = m.State
	}
	if m.Units != "" {
		units = m.Units
	}

	if units != "imperial" && units != "metric" {
		return fmt.Errorf("Bad units: %s", units)
	}
	return
}

func openCacheLock(how int) (lkf *os.File, err error) {
	lkf, err = os.OpenFile(home+"/"+cachelkfile, os.O_RDWR|os.O_CREATE,
		os.FileMode(0600))
	if err != nil {
		return
	}

	err = syscall.Flock(int(lkf.Fd()), how)
	if err != nil {
		lkf.Close()
		lkf = nil
		return
	}

	return
}

func queryWService() (res WData, err error) {
	present := false
	stale := false

	fqchfile := home + "/" + cachefile

	fi, err := os.Stat(fqchfile)
	if err != nil {
		if !os.IsNotExist(err) {
			return
		}
		err = nil
	} else {
		present = true
		if time.Since(fi.ModTime()) > (5 * time.Minute) {
			stale = true
		}
		// XXX we don't know that the location hasn't changed since our cache.
		// If it has, we should treat this as stale.
	}

	// If cache present, always parse and return cached result immediately.
	// If cache is stale, return stale data and fork HTTP GET worker
	// If cache absent, fg HTTP worker and return results when we have em.

	var body []byte
	if present {
		var lkf, c *os.File
		lkf, err = openCacheLock(syscall.LOCK_SH)
		if err != nil {
			return
		}
		defer lkf.Close()

		c, err = os.Open(fqchfile)
		if err != nil {
			return
		}
		defer c.Close()

		body, err = ioutil.ReadAll(c)
		if err != nil {
			return
		}
	}

	if !present || stale {
		var newbody []byte
		newbody, err = updateCache(present)
		if err != nil {
			return
		}

		// We can't background update cache anyways, so we might as well serve
		// the fresh data
		/*
			if !present {
				body = newbody
			}
		*/
		body = newbody
	}

	err = parseWJson(body, &res)
	return
}

func updateCache(runInBg bool) (body []byte, err error) {
	// We can't do this in background without re-invoking our process (probably
	// with some hidden? flag). Damn you issue 227.
	body, err = queryHttp()
	if err != nil {
		body = nil
		return
	}

	lkf, err := openCacheLock(syscall.LOCK_EX)
	if err != nil {
		body = nil
		return
	}
	defer lkf.Close()

	fqchfile := home + "/" + cachefile
	err = ioutil.WriteFile(fqchfile, body, os.FileMode(0600))
	if err != nil {
		body = nil
	}
	return
}

// api_key, ST, City_Name
var conditions_query = "http://api.wunderground.com/api/%s/conditions/q/" +
	"%s/%s.json"

func queryHttp() (res []byte, err error) {
	url := fmt.Sprintf(conditions_query, api_key, state, city)
	resp, err := http.Get(url)
	if err != nil {
		return
	}

	defer resp.Body.Close()

	res, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	if resp.StatusCode != 200 {
		panic(fmt.Sprint(res))
	}
	return
}

type DisplayLocationResp struct {
	City string
}
type CurrentObservationResp struct {
	DisplayLocation  DisplayLocationResp `json:"display_location"`
	FeelsLikeC       string              `json:"feelslike_c"`
	FeelsLikeF       string              `json:"feelslike_f"`
	Humidity         string              `json:"relative_humidity"`
	Icon             string
	ObservationEpoch string  `json:"observation_epoch"`
	TempC            float64 `json:"temp_c"`
	TempF            float64 `json:"temp_f"`
	Weather          string
}
type ErrorStatus struct {
	Description string
	Type        string
}
type ResponseStatus struct {
	Error ErrorStatus
}
type ConditionsResp struct {
	CurrentObservation CurrentObservationResp `json:"current_observation"`
	Response           ResponseStatus
}

func parseWJson(d []byte, wd *WData) (err error) {
	var obs ConditionsResp
	err = json.Unmarshal(d, &obs)
	if err != nil {
		return
	}

	if et := obs.Response.Error.Type; et != "" {
		ed := obs.Response.Error.Description
		if et == "keynotfound" {
			return fmt.Errorf("The service rejected your API key: %s", ed)
		} else if et == "querynotfound" {
			return fmt.Errorf("%s. Only US cities work; please separate "+
				"names with underscores. E.g., 'Ann_Arbor'", ed)
		}

		return fmt.Errorf("%s: %s", et, ed)
	}

	co := &obs.CurrentObservation

	wd.City = co.DisplayLocation.City
	wd.Temp = fmt.Sprintf("%.02f°F", co.TempF)
	wd.Conditions = co.Weather
	wd.Humidity = co.Humidity
	return
}

var colors map[string]string = map[string]string{
	"clear": "\033[0m",
	"dash":  "\033[34m",
	"data":  "\033[33;1m",
	"delim": "\033[35m",
	"text":  "\033[36;1m",
}

func color(c string) (res string) {
	res, ok := colors[c]
	if !ok {
		panic("color")
	}
	if sflag {
		res = "%{" + res + "%}"
	}
	return res
}

func formatWData(d WData) string {
	chars := map[string]string{
		"dash":  ",",
		"delim": ":",
	}

	res := ""
	res += color("text") + d.City
	res += color("delim") + chars["delim"]
	res += color("data") + " " + d.Temp + " "
	//res += icon
	res += color("dash") + chars["dash"]
	res += color("text") + " Humidity"
	res += color("delim") + chars["delim"]
	res += color("data") + " " + d.Humidity
	if sflag {
		res += "%"
	}

	res += color("clear")
	return res
}
