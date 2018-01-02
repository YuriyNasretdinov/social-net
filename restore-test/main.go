package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/cockroachdb/cockroach/pkg/storage/engine"
	"github.com/cockroachdb/cockroach/pkg/util/hlc"
)

var mapping = map[string]string{
	"51": "city",
	"52": "friend",
	"53": "messages",
	"54": "socialuser",
	"55": "timeline",
	"56": "userinfo",
}

func escape(q string) string {
	var b bytes.Buffer
	for _, c := range q {
		b.WriteRune(c)

		if c == '\'' {
			b.WriteRune(c)
		}
	}

	return b.String()
}

func main() {
	db, err := engine.NewRocksDB(engine.RocksDBConfig{
		Dir:       "/Users/yuriy/tmp/vbambuke",
		MustExist: true,
	}, engine.NewRocksDBCache(1000000))

	if err != nil {
		log.Fatalf("Could not open cockroach rocksdb: %v", err.Error())
	}

	cnt := 0

	prevPK := ""
	prevTable := ""

	err = db.Iterate(
		engine.MVCCKey{Timestamp: hlc.MinTimestamp},
		engine.MVCCKeyMax,
		func(kv engine.MVCCKeyValue) (bool, error) {
			cnt++

			if kv.Key.Key == nil {
				return false, nil
			}

			k := fmt.Sprint(kv.Key)

			if cnt%300000 == 0 {
				log.Printf("Iterated %d entries", cnt)
			}

			if bytes.Contains([]byte(kv.Value), []byte("safari@apple.com")) {
				log.Printf("Email key: %s", k)
			}

			const prefix = "/Table/"
			if strings.HasPrefix(k, prefix) {
				parts := strings.SplitN(strings.TrimPrefix(k, prefix), "/", 2)
				table, key := parts[0], parts[1]
				if t, ok := mapping[table]; ok {
					table = t
				}
				fp, _ := os.OpenFile("tables/"+table, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
				fp.WriteString(fmt.Sprintf("%s    =     %s\n", key, kv.Value))
				fp.Close()

				parts1 := strings.Split(key, "/")
				typ := parts1[0]
				pk := parts1[1]

				if pk == prevPK && table == prevTable {
					return false, nil
				}

				prevPK = pk
				prevTable = table

				if typ == "1" {
					if table == "city" {
						v := kv.Value[6:]
						ln := v[0]

						fmt.Printf(
							"INSERT INTO city2(id, name, lon, lat) VALUES(%s, '%s', 0, 0);\n",
							pk, escape(string(v[1:ln+1])),
						)

						// log.Printf("City %s = '%s'", pk, string(v[1:ln+1]))
					} else if table == "socialuser" {
						v := kv.Value[6:]
						ln := v[0]
						email := string(v[1 : ln+1])

						v = v[ln+2:]
						ln = v[0]
						password := string(v[1 : ln+1])

						v = v[ln+2:]
						ln = v[0]
						name := string(v[1 : ln+1])

						fmt.Printf(
							"INSERT INTO socialuser2(id, email, password, name) VALUES(%s, '%s', '%s', '%s');\n",
							pk, escape(email), escape(password), escape(name),
						)

						// log.Printf("User %s    email=%s   password=%s   name=%s", pk, email, password, name)
					} else if table == "userinfo" {
						v := kv.Value[6:]
						ln := int(v[0])
						name := string(v[1 : ln+1])

						if v[ln+1] != '\023' {
							return true, errors.New("userinfo NOT 023 (name)")
						}

						v = v[ln+2:]
						birthdate, ln := binary.Varint(v)

						if v[ln] != '\023' {
							return true, errors.New("userinfo NOT 023 (birthdate)")
						}

						v = v[ln+1:]
						sex := v[0]

						if v[1] != '\026' {
							return true, errors.New("userinfo NOT 023 (sex)")
						}

						v = v[2:]
						ln = int(v[0])
						description := string(v[1 : ln+1])

						if v[ln+1] != '\023' {
							return true, errors.New("userinfo NOT 023 (description)")
						}

						v = v[ln+2:]
						cityID, ln := binary.Varint(v)

						if v[ln] != '\023' {
							return true, errors.New("userinfo NOT 023 (city_id)")
						}

						v = v[ln+1:]
						familyPosition := v[0]

						ts := time.Unix(birthdate*86400, 0)

						/*
							log.Printf(
								"User %s    name='%s'   birthdate=%v   sex=%d   description='%s'   city_id=%d   familyPosition=%d",
								pk, name, birthdate, sex, description, cityID, familyPosition,
							)
						*/

						fmt.Printf(
							"INSERT INTO userinfo2(user_id, name, birthdate, sex, description, city_id, family_position) VALUES(%s, '%s', '%s', %d, '%s', %d, %d);\n",
							pk, escape(name), fmt.Sprintf("%04d-%02d-%02d", ts.Year(), ts.Month(), ts.Day()),
							sex, escape(description), cityID, familyPosition,
						)
					}
				}
			}

			// log.Println("Key = ", kv.Key)
			// log.Printf("Value = %s", kv.Value)
			return false, nil
		},
	)

	if err != nil {
		log.Fatalf("Error during iteration: %v", err.Error())
	}

	log.Printf("Database has %d entries", cnt)
}
