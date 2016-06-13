package config

import (
	"io/ioutil"
	"log"
	"os"

	"github.com/BurntSushi/toml"
)

type (
	Config struct {
		Postgresql string
		Memcache   string
		Bind       string
	}
)

var (
	Conf Config
)

func ParseConfig(path string) {
	fp, err := os.Open(path)
	if err != nil {
		log.Fatal("Could not open config " + err.Error())
	}

	defer fp.Close()

	contents, err := ioutil.ReadAll(fp)
	if err != nil {
		log.Fatal("Could not read config: " + err.Error())
	}

	if _, err = toml.Decode(string(contents), &Conf); err != nil {
		log.Fatal("Could not parse config: " + err.Error())
	}
}
