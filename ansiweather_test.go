package main

import "io/ioutil"
import "os"
import "testing"

func TestParseWJson(t *testing.T) {
    testf, err := os.Open("demo.json")
    if err != nil {
        t.Error(err)
    }
    defer testf.Close()

    body, err := ioutil.ReadAll(testf)
    if err != nil {
        t.Error(err)
    }

    var wd WData
    err = parseWJson(body, &wd)
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
