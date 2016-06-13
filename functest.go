package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/YuriyNasretdinov/social-net/protocol"
	"golang.org/x/net/websocket"
)

const TEST_PASSWORD = "test"
const TEST_MSG_TEXT = "Hello from test"
const TEST_USER_ID = 1

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
			time.Sleep(time.Second)
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
				if value["UserFrom"].(string) != fmt.Sprint(TEST_USER_ID) {
					time.Sleep(time.Second)
					log.Fatalf("Improper event new message, expected UserFrom=%d: %+v", TEST_USER_ID, value)
				}
				setFlag(newmsg)
			default:
				respChan <- value
			}
		}
	}
}

type TestConn struct {
	c        *websocket.Conn
	respChan chan map[string]interface{}
	seqId    int
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

	log.Printf("Reply to %s: %+v", msg, resp)

	p.seqId++
	return nil
}

func testGetMessages(p *TestConn) {
	var reply protocol.ReplyGetMessages

	log.Printf("Testing get messages")

	err := p.callMethod("REQUEST_GET_MESSAGES", "REPLY_MESSAGES_LIST", &protocol.RequestGetMessages{Limit: 10, UserTo: TEST_USER_ID}, &reply)
	if err != nil {
		panic(err)
	}

	if len(reply.Messages) != 1 {
		log.Panicf("Received unexpected number of messages (%d) instead of 1", len(reply.Messages))
	}

	msg := reply.Messages[0]

	if msg.UserFrom != fmt.Sprint(TEST_USER_ID) {
		log.Panicf("Unexpected user from: requested user id=%d, got %s", TEST_USER_ID, msg.UserFrom)
	}

	if msg.Text != TEST_MSG_TEXT {
		log.Panicf("Unexpected msg text: expected '%s', got '%s'", TEST_MSG_TEXT, msg.Text)
	}

	if msg.IsOut != protocol.MSG_TYPE_OUT {
		log.Panicf("Unexpected msg type: expected '%v', got '%v'", protocol.MSG_TYPE_OUT, msg.IsOut)
	}
}

func testSendMessage(p *TestConn) error {
	var reply protocol.ReplyGeneric
	log.Printf("Testing send message")

	err := p.callMethod("REQUEST_SEND_MESSAGE", "REPLY_GENERIC", &protocol.RequestSendMessage{UserTo: TEST_USER_ID, Text: TEST_MSG_TEXT}, &reply)
	if err != nil {
		panic(err)
	}

	return nil
}

func checkFlag(fl chan bool, name string) {
	if !haveFlag(fl) {
		panic("Did not receive " + name)
	}
}

func testConvertUnderscoreToCamelCase() {
	exp := "RequestGetMessages"
	res := convertUnderscoreToCamelCase("REQUEST_GET_MESSAGES")
	if res != exp {
		log.Panicf("Unexpected result from convertUnderscoreToCamelCase, expected '%s', got '%s'", exp, res)
	}
}

func runTest(addr string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			switch v := r.(type) {
			case error:
				err = v
			case string:
				err = errors.New(v)
			default:
				panic(r)
			}
		}
	}()

	c, err := setupAndGetConnection(addr)
	if err != nil {
		return
	}

	testConvertUnderscoreToCamelCase()

	online := make(chan bool, 1)
	connected := make(chan bool, 1)
	newmsg := make(chan bool, 1)
	respChan := make(chan map[string]interface{}, 1)

	go responseReaderThread(c, online, connected, newmsg, respChan)
	p := &TestConn{c: c, respChan: respChan, seqId: 0}

	testSendMessage(p)
	testGetMessages(p)

	time.Sleep(time.Millisecond * 100)

	checkFlag(online, "online users list")
	checkFlag(connected, "user connected")
	checkFlag(newmsg, "new message event")

	return nil
}
