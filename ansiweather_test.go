package main

import "io/ioutil"
import "os"
import "testing"

func readFile(fn string, t *testing.T) []byte {
	fh, err := os.Open(fn)
	if err != nil {
		t.Error(err)
	}
	defer fh.Close()

	body, err := ioutil.ReadAll(fh)
	if err != nil {
		t.Error(err)
	}
	return body
}

func TestParseWJson(t *testing.T) {
	body := readFile("demo.json", t)

	var wd WData
	err := parseWJson(body, &wd)
	if err != nil {
		t.Error("parse: ", err)
	}

	expTmp := "66.30°F"
	if wd.Temp != expTmp {
		t.Error("temp:", wd.Temp, "!=", expTmp)
	}

	expWeather := "Partly Cloudy"
	if wd.Conditions != expWeather {
		t.Error("cond:", wd.Conditions, "!=", expWeather)
	}

	expHumidity := "65%"
	if wd.Humidity != expHumidity {
		t.Error("humidity:", wd.Humidity, "!=", expHumidity)
	}
}

func TestParseWJsonErrorApiKey(t *testing.T) {
	bak := readFile("bad_api_key.json", t)

	var wd WData
	err := parseWJson(bak, &wd)
	if err == nil {
		t.Error("Expected error")
	}

	em := err.Error()
	expm := "The service rejected your API key: this key does not exist"

	if em != expm {
		t.Errorf("bad_api_key: error '%s' doesn't match expected '%s'", em,
			expm)
	}
}

func TestParseWJsonErrorCityNoEnt(t *testing.T) {
	bak := readFile("bad_city.json", t)

	var wd WData
	err := parseWJson(bak, &wd)
	if err == nil {
		t.Error("Expected error")
	}

	em := err.Error()
	expm := "No cities match your search query. Only US cities work; please " +
		"separate names with underscores. E.g., 'Ann_Arbor'"

	if em != expm {
		t.Errorf("bad_city: error '%s' doesn't match expected '%s'", em,
			expm)
	}
}
