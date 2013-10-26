## Description

gAnsiWeather is a small Go program for displaying the current weather
conditions in your terminal, with support for ANSI colors and Unicode symbols.
It borrows the concept heavily from `fcambus/ansiweather`.

![gAnsiWeather Screenshot](http://i.imgur.com/WlhaFQz.png)

We use the API provided by
[Weather Underground](http://www.wunderground.com/?apiref=188a3b96201b7e85) .

## Requirements

gAnsiWeather requires a Go compiler and a working internet connection. No
libraries outside of stdlib are used.

## Build

    go build
    go test

I recommend installing in `~/.local/bin/` but anywhere on `$PATH` is fine.

## Usage

First, configure `~/.config/gansiweather.conf`. Here is an example:

    {
        "ApiKey": "XXX",
        "City": "Ann_Arbor",
        "State": "MI"
    }

Then simply invoke:

    gansiweather

Results are cached in `~/.config/gansiweather.cache.json` (by default, for 10
minutes). You can adjust the caching period by setting `CacheSeconds` in
`gansiweather.conf`.

## Configuration

For US cities, simply set the city and state appropriately. Cities must be
capitalized, with spaces replaced by underscores.

For other countries, set "State" to your country name and "City" to the city
name. Metric isn't implemented, but in the future the key "Units" should be set
to "metric".

The default is "imperial".

## Use in ZSH PS1

To enable as part of your zsh PS1 variable, you must add something like this to
your `.zshrc`:

    setopt promptsubst
    PS1='$(gansiweather -s) foo bar $ '

The `-s` option to gansiweather escapes non-printing ANSI color codes so that
zsh knows how wide the prompt is.

## License

gAnsiWeather is released under the MIT license. See `LICENSE` for details.

## Github URL

https://github.com/cemeyer/gansiweather
