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

func setupAndGetConnection(addr string) (*websocket.Conn, error) {
	time.Sleep(time.Millisecond * 100)

	conf, err := websocket.NewConfig("ws://"+addr+"/events", "/")
	if err != nil {
		return nil, err
	}

	name := fmt.Sprintf("test%d", time.Now().Unix())
	email := name + "@example.org"
	err, dup := registerUser(email, TEST_PASSWORD, name)
	if err != nil {
		return nil, err
	}
	if dup {
		return nil, errors.New("User already exists")
	}

	sessionId, err := loginUser(email, TEST_PASSWORD)
	if err != nil {
		return nil, err
	}

	conf.Header.Add("Cookie", "id="+sessionId)

	return websocket.DialConfig(conf)
}

func responseReaderThread(c *websocket.Conn, online, connected, newmsg chan bool, respChan chan map[string]interface{}) {
	rd := json.NewDecoder(c)
	for {
		var value map[string]interface{}

		if err := rd.Decode(&value); err != nil {
			log.Fatalln("Could not decode value: ", err.Error())
		}

		msgType, ok := value["Type"].(string)

		if ok {
			switch msgType {
			case "EVENT_ONLINE_USERS_LIST":
				setFlag(online)
			case "EVENT_USER_CONNECTED":
				setFlag(connected)
			case "EVENT_NEW_MESSAGE":
				if value["UserFrom"].(string) != "1" {
					log.Fatalf("Improper event new message, expected UserFrom=1: %+v", value)
				}
				setFlag(newmsg)
			default:
				respChan <- value
			}
		}
	}
}

type TestConn struct {
	c *websocket.Conn
	respChan chan map[string]interface{}
	seqId int
}

func (p *TestConn) callMethod(msg, respMsg string, req, resp interface{}) error {
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	fmt.Fprintf(p.c, "%s %d\n%s\n", msg, p.seqId, data)
	rawResp := <-p.respChan

	if rawResp["Type"].(string) != respMsg {
		return fmt.Errorf("Invalid response, expected %s for %s: %+v\n", respMsg, msg, rawResp)
	}

	if rawResp["SeqId"].(float64) != float64(p.seqId) {
		return fmt.Errorf("Invalid SeqId: %+v\n", rawResp)
	}

	data, err = json.Marshal(rawResp)
	if err != nil {
		return err
	}
	json.Unmarshal(data, resp)

	p.seqId++
	return nil
}

func testGetMessages(p *TestConn) error {
	var reply ReplyGetMessages

	err := p.callMethod("REQUEST_GET_MESSAGES", "REPLY_MESSAGES_LIST", &RequestGetMessages{Limit: 10}, &reply)
	if err != nil {
		return err
	}

	fmt.Printf("Reply to REQUEST_GET_MESSAGES: %+v\n", reply)
	return nil
}

func testSendMessage(p *TestConn) error {
	var reply ReplyGeneric
	err := p.callMethod("REQUEST_SEND_MESSAGE", "REPLY_GENERIC", &RequestSendMessage{UserTo: 1, Text: "Hello from test"}, &reply)
	if err != nil {
		return err
	}

	fmt.Printf("Reply to REQUEST_SEND_MESSAGE: %+v\n", reply)
	return nil
}

func runTest(addr string) error {
	c, err := setupAndGetConnection(addr)
	if err != nil {
		return err
	}

	online := make(chan bool, 1)
	connected := make(chan bool, 1)
	newmsg := make(chan bool, 1)
	respChan := make(chan map[string]interface{}, 1)

	go responseReaderThread(c, online, connected, newmsg, respChan)
	p := &TestConn{c: c, respChan: respChan, seqId: 0}

	if err = testGetMessages(p); err != nil {
		return err
	}

	if !haveFlag(online) {
		return errors.New("Did not receive online users list")
	}

	if !haveFlag(connected) {
		return errors.New("Did not receive user connected")
	}

	if err = testSendMessage(p); err != nil {
		return err
	}

	time.Sleep(time.Millisecond * 100)

	if !haveFlag(newmsg) {
		return errors.New("Did not receive user connected")
	}

	return nil
}
