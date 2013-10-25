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

var sflag = false
var configfile = ".config/gansiweather.conf"
var home = ""

// defaults
var api_key = ""
var cache_seconds uint64 = 10 * 60
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
		fmt.Print(err)
		os.Exit(1)
	}
}

func start() (err error) {
	flag.Parse()

	home = os.Getenv("HOME")
	if len(home) == 0 {
		return fmt.Errorf("Could not read $HOME")
	}

	cfile := home + "/" + configfile
	fi, err := os.Stat(cfile)
	if err != nil {
		if !os.IsNotExist(err) {
			return
		}
	} else {
		if !fi.Mode().IsRegular() {
			return fmt.Errorf("%s: Config is not a regular file", cfile)
		}

		err = readConfig(cfile)
		if err != nil {
			return
		}

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
		cache_seconds = m.CacheSeconds
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

// api_key, ST, City_Name
var conditions_query = "http://api.wunderground.com/api/%s/conditions/q/" +
	"%s/%s.json"

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

func queryWService() (res WData, err error) {
	url := fmt.Sprintf(conditions_query, api_key, state, city)
	resp, err := http.Get(url)
	if err != nil {
		return
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	if resp.StatusCode != 200 {
		panic(fmt.Sprint(body))
	}

	err = parseWJson(body, &res)
	if err != nil {
		return
	}

	return
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
