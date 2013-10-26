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
import "strconv"
import "strings"
import "syscall"
import "time"

var sflag = false
var dflag = false
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
	City        string
	Conditions  string
	CurrentTime uint64 /* minutes since midnight */
	Humidity    string
	SunriseTime uint64
	SunsetTime  uint64
	Temp        string
}

func init() {
	flag.BoolVar(&sflag, "s", false, "Escape ANSI color sequences so "+
		"they are ignored for length purposes")
	flag.BoolVar(&dflag, "d", false, "Debug")
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

	fmt.Print(formatWData(data))
	if !sflag {
		fmt.Print("\n")
	}

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

	if dflag {
		fmt.Printf("XXX attempting to flock cache: %d\n", how)
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
		body, err = readCache()
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

func readCache() (body []byte, err error) {
	var lkf, c *os.File
	lkf, err = openCacheLock(syscall.LOCK_SH)
	if err != nil {
		return
	}
	defer lkf.Close()

	fqchfile := home + "/" + cachefile

	c, err = os.Open(fqchfile)
	if err != nil {
		return
	}
	defer c.Close()

	body, err = ioutil.ReadAll(c)
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
var conditions_query = "http://api.wunderground.com/api/%s/conditions" +
	"/astronomy/q/%s/%s.json"

func queryHttp() (res []byte, err error) {
	url := fmt.Sprintf(conditions_query, api_key, state, city)
	if dflag {
		fmt.Printf(">>> GET %s\n", url)
	}
	resp, err := http.Get(url)
	if dflag {
		fmt.Printf("<<< GET complete\n", url)
	}
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
type HMTime struct {
	Hour   string
	Minute string
}
type MoonPhaseResp struct {
	CurrentTime HMTime `json:"current_time"`
	Sunrise     HMTime
	Sunset      HMTime
}
type ConditionsResp struct {
	CurrentObservation CurrentObservationResp `json:"current_observation"`
	MoonPhase          MoonPhaseResp          `json:"moon_phase"`
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

	mp := &obs.MoonPhase
	if mp.CurrentTime.Hour != "" {
		wd.CurrentTime = hmTimeToMinutes(&mp.CurrentTime)
		wd.SunriseTime = hmTimeToMinutes(&mp.Sunrise)
		wd.SunsetTime = hmTimeToMinutes(&mp.Sunset)
	}
	return
}

func hmTimeToMinutes(hmt *HMTime) (res uint64) {
	h, err := strconv.ParseUint(hmt.Hour, 10, 64)
	if err != nil {
		panic(err)
	}
	m, err := strconv.ParseUint(hmt.Minute, 10, 64)
	if err != nil {
		panic(err)
	}

	return (h * 60) + m
}

var colors map[string]string = map[string]string{
	"clear":  "\033[0m",
	"clouds": "\033[37;1m",
	"dash":   "\033[34m",
	"data":   "\033[33;1m",
	"delim":  "\033[35m",
	"moon":   "\033[36m",
	"sun":    "\033[33;1m",
	"text":   "\033[36;1m",
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
	conditions := map[string]string{
		"Clear":    "clear",
		"Cloud":    "cloudy",
		"Overcast": "cloudy",
		"Haze":     "cloudy",
		"Fog":      "cloudy",
		"Mist":     "rain",
		"Rain":     "rain",
		"Snow":     "snow",
		"Ice":      "snow",
	}

	cond := d.Conditions
	for k, v := range conditions {
		if strings.Contains(cond, k) {
			cond = v
			break
		}
	}

	if cond == "clear" {
		if d.CurrentTime < d.SunsetTime &&
			d.CurrentTime > d.SunriseTime {
			cond = "sun"
		} else {
			cond = "moon"
		}
	}

	type wSymbol struct {
		color  string
		symbol string
	}

	secconditions := map[string]wSymbol{
		"sun":    {color: "sun", symbol: "☀"},
		"moon":   {color: "moon", symbol: "☾"},
		"cloudy": {color: "clouds", symbol: "☁"},
		"rain":   {symbol: "☔"},
		"snow":   {symbol: "❄"},
	}

	icon := color("text") + "(" + cond + ")"
	if symcol, ok := secconditions[cond]; ok {
		col := ""
		if _, ok := colors[symcol.color]; ok {
			col = color(symcol.color)
		}
		icon = col + symcol.symbol
	}

	chars := map[string]string{
		"dash":  ",",
		"delim": ":",
	}

	res := ""
	res += color("text") + d.City
	res += color("delim") + chars["delim"]
	res += color("data") + " " + d.Temp + " "
	res += icon
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
