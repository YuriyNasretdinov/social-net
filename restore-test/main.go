package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/cockroachdb/cockroach/pkg/storage/engine"
	"github.com/cockroachdb/cockroach/pkg/util/encoding"
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

func decodeCity(pk string, kv engine.MVCCKeyValue) {
	v := kv.Value[6:]
	ln := v[0]

	fmt.Sprintf(
		"INSERT INTO city2(id, name, lon, lat) VALUES(%s, '%s', 0, 0);\n",
		pk, escape(string(v[1:ln+1])),
	)

	// log.Printf("City %s = '%s'", pk, string(v[1:ln+1]))
}

func decodeSocialUser(pk string, kv engine.MVCCKeyValue) {
	v := kv.Value[6:]
	ln := v[0]
	email := string(v[1 : ln+1])

	v = v[ln+2:]
	ln = v[0]
	password := string(v[1 : ln+1])

	v = v[ln+2:]
	ln = v[0]
	name := string(v[1 : ln+1])

	fmt.Sprintf(
		"INSERT INTO socialuser2(id, email, password, name) VALUES(%s, '%s', '%s', '%s');\n",
		pk, escape(email), escape(password), escape(name),
	)

	// log.Printf("User %s    email=%s   password=%s   name=%s", pk, email, password, name)
}

func readStringFirst(v []byte) ([]byte, string) {
	v, _, ln, err := encoding.DecodeNonsortingUvarint(v)
	if err != nil {
		panic("could not decode string length")
	}

	return v[ln:], string(v[0:ln])
}

func readString(v []byte) ([]byte, string) {
	if v[0] != '\026' {
		panic("invalid string prefix")
	}
	return readStringFirst(v[1:])
}

func readVarIntFirst(v []byte) ([]byte, int64) {
	res, ln := binary.Varint(v)
	if ln <= 0 {
		panic("could not read varint")
	}
	return v[ln:], res
}

func readVarInt(v []byte) ([]byte, int64) {
	if v[0] != '\023' {
		panic("invalid varint prefix")
	}
	return readVarIntFirst(v[1:])
}

func decodeUserInfo(pk string, kv engine.MVCCKeyValue) {
	v := kv.Value[6:]

	v, name := readStringFirst(v)
	v, birthdate := readVarInt(v)
	v, sex := readVarInt(v)
	v, description := readString(v)
	v, cityID := readVarInt(v)
	v, familyPosition := readVarInt(v)

	ts := time.Unix(birthdate*86400, 0)

	/*
		log.Printf(
			"User %s    name='%s'   birthdate=%v   sex=%d   description='%s'   city_id=%d   familyPosition=%d",
			pk, name, birthdate, sex, description, cityID, familyPosition,
		)
	*/

	fmt.Sprintf(
		"INSERT INTO userinfo2(user_id, name, birthdate, sex, description, city_id, family_position) VALUES(%s, '%s', '%s', %d, '%s', %d, %d);\n",
		pk, escape(name), fmt.Sprintf("%04d-%02d-%02d", ts.Year(), ts.Month(), ts.Day()),
		sex, escape(description), cityID, familyPosition,
	)
}

func decodeFriend(pk string, kv engine.MVCCKeyValue) {
	v := kv.Value[6:]

	v, userID := readVarIntFirst(v)
	v, friendUserID := readVarInt(v)
	requestAccepted := "TRUE" // does not matter too much

	/*
		log.Printf(
			"Friend %s    userID=%d   friendUserID=%d   requestAccepted=%v",
			pk, userID, friendUserID, requestAccepted,
		)
	*/

	fmt.Sprintf(
		"INSERT INTO friend2(id, user_id, friend_user_id, request_accepted) VALUES(%s, %d, %d, %v);\n",
		pk, userID, friendUserID, requestAccepted,
	)
}

//var isOut = true // out is written first

func decodeMessages(pk string, kv engine.MVCCKeyValue) {
	v := kv.Value[6:]

	v, userID := readVarIntFirst(v)
	v, userIDTo := readVarInt(v)
	v, isOut, err := encoding.DecodeBoolValue(v)
	if err != nil {
		panic(fmt.Errorf("Could not decode bool value: %s", err.Error()))
	}
	v, message := readString(v)
	v, ts := readVarInt(v)

	fmt.Printf(
		"INSERT INTO messages2(id, user_id, user_id_to, is_out, message, ts) VALUES(%s, %d, %d, %v, '%s', %d);\n",
		pk, userID, userIDTo, isOut, escape(message), ts,
	)
}

func decodeTimeline(pk string, kv engine.MVCCKeyValue) {
	v := kv.Value[6:]

	v, userID := readVarIntFirst(v)
	v, sourceUserID := readVarInt(v)
	v, message := readString(v)
	v, ts := readVarInt(v)

	fmt.Sprintf(
		"INSERT INTO timeline2(id, user_id, source_user_id, message, ts) VALUES(%s, %d, %d, '%s', %d);\n",
		pk, userID, sourceUserID, escape(message), ts,
	)
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

			const prefix = "/Table/"
			if !strings.HasPrefix(k, prefix) {
				return false, nil
			}
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

			if typ != "1" {
				return false, nil
			}

			if table == "city" {
				decodeCity(pk, kv)
			} else if table == "socialuser" {
				decodeSocialUser(pk, kv)
			} else if table == "userinfo" {
				decodeUserInfo(pk, kv)
			} else if table == "friend" {
				decodeFriend(pk, kv)
			} else if table == "messages" {
				decodeMessages(pk, kv)
			} else if table == "timeline" {
				decodeTimeline(pk, kv)
			}

			return false, nil
		},
	)

	if err != nil {
		log.Fatalf("Error during iteration: %v", err.Error())
	}

	log.Printf("Database has %d entries", cnt)
}
