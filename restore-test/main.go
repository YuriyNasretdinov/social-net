package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"

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

func main() {
	db, err := engine.NewRocksDB(engine.RocksDBConfig{
		Dir:       "/Users/yuriy/tmp/vbambuke",
		MustExist: true,
	}, engine.NewRocksDBCache(1000000))

	if err != nil {
		log.Fatalf("Could not open cockroach rocksdb: %v", err.Error())
	}

	cnt := 0

	err = db.Iterate(
		engine.MVCCKey{Timestamp: hlc.MinTimestamp},
		engine.MVCCKeyMax,
		func(kv engine.MVCCKeyValue) (bool, error) {
			cnt++

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

				if table == "city" {
					parts := strings.Split(key, "/")
					typ := parts[0]
					if typ == "1" {
						pk := parts[1]
						v := kv.Value[6:]
						ln := v[0]
						log.Printf("City %s = '%s'", pk, string(v[1:ln+1]))
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
