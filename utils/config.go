package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	ForeverDuration = "2000000h"
)

// A lot of things don't parse durations correctly; I need it.
type Duration time.Duration

func (d *Duration) UnmarshalText(b []byte) error {
	bs := string(b)
	if bs == "never" || bs == "infinite" {
		bs = ForeverDuration
	}
	x, err := time.ParseDuration(bs)
	if err != nil {
		return err
	}
	*d = Duration(x)
	return nil
}

// Read a stack of configs, starting with basename and going through base0.ext -> baseN.ext
func ReadConfigStack(basename string, apply func(string, []byte) error, maxRead int) ([]string, error) {
	configs := make([]string, maxRead+1)
	results := make([]string, 0)
	configs[0] = basename
	extension := filepath.Ext(basename)
	pre := basename[:len(basename)-len(extension)]
	for i := range maxRead {
		configs[i+1] = fmt.Sprintf("%s%d%s", pre, i, extension)
	}
	// read each one, applying it using the given marshal function
	for _, configfile := range configs {
		_, err := os.Stat(configfile)
		if os.IsNotExist(err) {
			// this is fine, the file just doesn't exist
			continue
		}
		configData, err := os.ReadFile(configfile)
		if err != nil {
			return results, err
		}
		err = apply(configfile, configData)
		if err != nil {
			return results, err
		}
		results = append(results, configfile)
	}
	return results, nil
}
