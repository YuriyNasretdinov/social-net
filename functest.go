package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/net/websocket"
	"log"
	"time"
)

const TEST_PASSWORD = "test"

func setFlag(fl chan bool) {
	select {
	case fl <- true:
	default:
	}
}

func haveFlag(fl chan bool) bool {
	select {
	case <-fl:
		return true
	default:
	}

	return false
}

func runTest(addr string) error {
	time.Sleep(time.Second)

	conf, err := websocket.NewConfig("ws://"+addr+"/events", "/")
	if err != nil {
		return err
	}

	name := fmt.Sprintf("test%d", time.Now().Unix())
	email := name + "@example.org"
	err, dup := registerUser(email, TEST_PASSWORD, name)
	if err != nil {
		return err
	}
	if dup {
		return errors.New("User already exists")
	}

	sessionId, err := loginUser(email, TEST_PASSWORD)
	if err != nil {
		return err
	}

	conf.Header.Add("Cookie", "id="+sessionId)

	c, err := websocket.DialConfig(conf)
	if err != nil {
		return err
	}

	hadOnlineUsersList := make(chan bool, 1)
	hadUserConnected := make(chan bool, 1)
	respChan := make(chan map[string]interface{}, 1)

	go func() {
		rd := json.NewDecoder(c)
		var value map[string]interface{}

		for {
			if err := rd.Decode(&value); err != nil {
				log.Println("Could not decode value: ", err.Error())
				return
			}

			msgType, ok := value["Type"].(string)

			if ok {
				switch msgType {
				case "EVENT_ONLINE_USERS_LIST":
					setFlag(hadOnlineUsersList)
				case "EVENT_USER_CONNECTED":
					setFlag(hadUserConnected)
				default:
					respChan <- value
				}
			}
		}
	}()

	c.Write([]byte(`REQUEST_GET_MESSAGES 0
{"Limit":10}
`))
	rawResp := <-respChan

	if rawResp["Type"].(string) != "REPLY_MESSAGES_LIST" {
		return fmt.Errorf("Invalid response: %+v\n", rawResp)
	}

	if rawResp["SeqId"].(float64) != 0 {
		return fmt.Errorf("Invalid SeqId: %+v\n", rawResp)
	}

	data, _ := json.Marshal(rawResp)
	var reply ReplyGetMessages
	json.Unmarshal(data, &reply)

	if !haveFlag(hadOnlineUsersList) {
		return errors.New("Did not receive online users list")
	}

	if !haveFlag(hadUserConnected) {
		return errors.New("Did not receive user connected")
	}

	fmt.Printf("Reply: %+v\n", reply)

	return nil
}
