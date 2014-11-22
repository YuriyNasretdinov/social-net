package main

import (
	"code.google.com/p/go.net/websocket"
	"crypto/md5"
	"crypto/sha1"
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	EVENT_USER_CONNECTED = iota
	EVENT_USER_DISCONNECTED
	EVENT_ONLINE_USERS_LIST
	EVENT_USER_REPLY
	EVENT_NEW_MESSAGE

	REQUEST_GET_MESSAGES = iota
	REQUEST_SEND_MESSAGE
	REQUEST_GET_TIMELINE
	REQUEST_ADD_TO_TIMELINE

	REPLY_ERROR = iota
	REPLY_MESSAGES_LIST
	REPLY_GENERIC
	REPLY_GET_TIMELINE

	MAX_MESSAGES_LIMIT = 100
	MAX_TIMELINE_LIMIT = 100

	MSG_TYPE_OUT = "Out"
	MSG_TYPE_IN  = "In"
)

type (
	BaseEvent struct {
		Type string
	}

	UserInfo struct {
		Name string
		Id   string
	}

	BaseRequest struct {
		SeqId   int
		Type    string
		ReqData string
	}

	RequestGetMessages struct {
		UserTo  uint64
		DateEnd string
		Limit   uint64
	}

	RequestSendMessage struct {
		UserTo uint64
		Text   string
	}

	RequestGetTimeline struct {
		DateEnd string
		Limit   uint64
	}

	RequestAddToTimeline struct {
		Message string
	}

	BaseReply struct {
		SeqId int
		Type  string
	}

	Message struct {
		Id       uint64
		UserFrom string
		Ts       string
		MsgType  string
		Text     string
	}

	TimelineMessage struct {
		Id       uint64
		UserId   string
		UserName string
		Text     string
		Ts       string
	}

	ReplyGetMessages struct {
		BaseReply
		Messages []Message
	}

	ReplyGetTimeline struct {
		BaseReply
		Messages []TimelineMessage
	}

	ReplyGeneric struct {
		BaseReply
		Success bool
	}

	ReplyError struct {
		BaseReply
		Message string
	}

	EventUserConnected struct {
		BaseEvent
		UserInfo
	}

	EventUserDisconnected struct {
		BaseEvent
		UserInfo
	}

	EventOnlineUsersList struct {
		BaseEvent
		Users []UserInfo
	}

	EventNewMessage struct {
		BaseEvent
		Message
	}

	ControlEvent struct {
		evType   uint8
		info     map[string]string
		reply    interface{}
		listener chan interface{}
	}
)

var (
	db *sql.DB

	// Authorization
	loginStmt *sql.Stmt

	// Messages
	getMessagesStmt *sql.Stmt
	sendMessageStmt *sql.Stmt

	// Timeline
	getFromTimelineStmt *sql.Stmt
	addToTimelineStmt   *sql.Stmt

	eventsFlow = make(chan *ControlEvent, 200)
)

func serveStatic(filename string, w http.ResponseWriter) {
	fp, err := os.Open(filename)
	if err != nil {
		w.WriteHeader(404)
		w.Write([]byte("Could not find file: " + filename))
		return
	}
	defer fp.Close()

	if strings.HasSuffix(filename, ".css") {
		w.Header().Add("Content-type", "text/css")
	} else if strings.HasSuffix(filename, ".js") {
		w.Header().Add("Content-type", "application/javascript")
	}

	io.Copy(w, fp)
}

func StaticServer(w http.ResponseWriter, req *http.Request) {
	serveStatic(req.URL.Path[len("/"):], w)
}

func LoginHandler(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	email := req.Form.Get("email")
	userPassword := req.Form.Get("password")

	if email == "" || userPassword == "" {
		fmt.Fprintf(w, "You must provide both email and password")
		return
	}

	var id uint64
	var password, name string

	err := loginStmt.QueryRow(email).Scan(&id, &password, &name)
	if err != nil {
		if err == sql.ErrNoRows {
			w.Write([]byte("You are not registered, sorry"))
		} else {
			w.Write([]byte("Error occured, sorry"))
			log.Println("Db error: " + err.Error())
		}
		return
	}

	if passwordHash(userPassword) != password {
		w.Write([]byte("Incorrect password"))
		return
	}

	sessionId, err := createSession(map[string]string{"id": fmt.Sprint(id), "name": name})

	cookie := &http.Cookie{
		Name:    "id",
		Value:   string(sessionId),
		Path:    "/",
		Domain:  req.Header.Get("Host"),
		Expires: time.Now().Add(365 * 24 * time.Hour),
	}

	http.SetCookie(w, cookie)
	w.Header().Add("Location", "/")
	w.WriteHeader(302)
}

func LogoutHandler(w http.ResponseWriter, req *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: "id"})
	w.Header().Add("Location", "/")
	w.WriteHeader(302)
}

func passwordHash(password string) string {
	sh := sha1.New()
	io.WriteString(sh, password)

	md := md5.New()
	io.WriteString(md, password)

	return fmt.Sprintf("%x:%x", sh.Sum(nil), md.Sum(nil))
}

func serveAuthPage(info map[string]string, w http.ResponseWriter) {
	authTpl.Execute(w, info)
}

func getAuthUserInfo(cookies []*http.Cookie) map[string]string {
	for _, cook := range cookies {
		if cook.Name == "id" && cook.Value != "" {
			info, err := getSessionInfo(cook.Value)
			if err == nil {
				return info
			} else {
				fmt.Println("Error: " + err.Error())
			}
		}
	}

	return nil
}

func IndexHandler(w http.ResponseWriter, req *http.Request) {
	// validate session
	if info := getAuthUserInfo(req.Cookies()); info != nil {
		serveAuthPage(info, w)
		return
	}

	serveStatic("static/index.html", w)
}

func handleUserConnected(listenerMap map[chan interface{}]map[string]string, userListeners map[uint64]map[chan interface{}]bool, ev *ControlEvent) {
	userId, err := strconv.ParseUint(ev.info["id"], 10, 64)
	if err != nil {
		log.Println("Cannot parse id: ", err.Error())
		return
	}

	currentUsers := make([]UserInfo, 0, len(listenerMap))
	for _, info := range listenerMap {
		currentUsers = append(currentUsers, UserInfo{Name: info["name"], Id: info["id"]})
	}

	ouEvent := new(EventOnlineUsersList)
	ouEvent.Type = "EVENT_ONLINE_USERS_LIST"
	ouEvent.Users = currentUsers
	ev.listener <- ouEvent

	listenerMap[ev.listener] = ev.info

	if userListeners[userId] == nil {
		userListeners[userId] = make(map[chan interface{}]bool)
	}

	userListeners[userId][ev.listener] = true

	for listener := range listenerMap {
		if len(listener) >= cap(listener) {
			return
		}

		event := new(EventUserConnected)
		event.Type = "EVENT_USER_CONNECTED"
		event.Name = ev.info["name"]
		event.Id = ev.info["id"]
		listener <- event
	}
}

func handleUserDisconnected(listenerMap map[chan interface{}]map[string]string, userListeners map[uint64]map[chan interface{}]bool, ev *ControlEvent) {
	userId, err := strconv.ParseUint(ev.info["id"], 10, 64)
	if err != nil {
		log.Println("Cannot parse id: ", err.Error())
		return
	}

	delete(listenerMap, ev.listener)
	if userListeners[userId] != nil {
		delete(userListeners[userId], ev.listener)
		if len(userListeners[userId]) == 0 {
			delete(userListeners, userId)
		}
	}

	for listener := range listenerMap {
		if len(listener) >= cap(listener) {
			return
		}

		event := new(EventUserDisconnected)
		event.Type = "EVENT_USER_DISCONNECTED"
		event.Name = ev.info["name"]
		event.Id = ev.info["id"]
		listener <- event
	}
}

func handleNewMessage(listenerMap map[chan interface{}]map[string]string, userListeners map[uint64]map[chan interface{}]bool, ev *ControlEvent) {
	userIdFrom, err := strconv.ParseUint(ev.info["UserFrom"], 10, 64)
	if err != nil {
		log.Println("Cannot decode userIdFrom: ", err.Error())
		return
	}

	userIdTo, err := strconv.ParseUint(ev.info["UserTo"], 10, 64)
	if err != nil {
		log.Println("Cannot decode userIdTo: ", err.Error())
		return
	}

	event := new(EventNewMessage)
	event.Type = "EVENT_NEW_MESSAGE"
	event.Ts = ev.info["Ts"]
	event.Text = ev.info["Text"]

	if userListeners[userIdFrom] != nil {
		for listener := range userListeners[userIdFrom] {
			if len(listener) >= cap(listener) {
				continue
			}
			fromEv := new(EventNewMessage)
			*fromEv = *event
			fromEv.UserFrom = ev.info["UserTo"]
			fromEv.MsgType = MSG_TYPE_OUT
			listener <- fromEv
		}
	}

	if userListeners[userIdTo] != nil {
		for listener := range userListeners[userIdTo] {
			if len(listener) >= cap(listener) {
				continue
			}
			toEv := new(EventNewMessage)
			*toEv = *event
			toEv.UserFrom = ev.info["UserFrom"]
			toEv.MsgType = MSG_TYPE_IN
			listener <- toEv
		}
	}

}

func EventsDispatcher() {
	listenerMap := make(map[chan interface{}]map[string]string)
	userListeners := make(map[uint64]map[chan interface{}]bool)

	for ev := range eventsFlow {
		if ev.evType == EVENT_USER_CONNECTED {
			handleUserConnected(listenerMap, userListeners, ev)
		} else if ev.evType == EVENT_USER_DISCONNECTED {
			handleUserDisconnected(listenerMap, userListeners, ev)
		} else if ev.evType == EVENT_NEW_MESSAGE {
			handleNewMessage(listenerMap, userListeners, ev)
		} else if ev.evType == EVENT_USER_REPLY {
			if _, ok := listenerMap[ev.listener]; !ok {
				continue
			}

			if len(ev.listener) >= cap(ev.listener) {
				continue
			}

			ev.listener <- ev.reply
		}
	}
}

func sendError(seqId int, recvChan chan interface{}, message string) {
	reply := new(ReplyError)
	reply.SeqId = seqId
	reply.Type = "REPLY_ERROR"
	reply.Message = message

	eventsFlow <- &ControlEvent{
		evType:   EVENT_USER_REPLY,
		listener: recvChan,
		reply:    reply,
	}
}

func processGetMessages(req *RequestGetMessages, seqId int, recvChan chan interface{}, userId uint64) {
	dateEnd := req.DateEnd

	if dateEnd == "" {
		dateEnd = fmt.Sprint(time.Now().UnixNano())
	}

	limit := req.Limit
	if limit > MAX_MESSAGES_LIMIT {
		limit = MAX_MESSAGES_LIMIT
	}

	if limit <= 0 {
		sendError(seqId, recvChan, "Limit must be greater than 0")
		return
	}

	rows, err := getMessagesStmt.Query(userId, req.UserTo, dateEnd, limit)
	if err != nil {
		sendError(seqId, recvChan, "Cannot select messages")
		log.Println(err.Error())
		return
	}

	reply := new(ReplyGetMessages)
	reply.SeqId = seqId
	reply.Type = "REPLY_MESSAGES_LIST"
	reply.Messages = make([]Message, 0)

	defer rows.Close()
	for rows.Next() {
		var msg Message
		if err = rows.Scan(&msg.Id, &msg.Text, &msg.Ts, &msg.MsgType); err != nil {
			log.Println(err.Error())
			return
		}

		reply.Messages = append(reply.Messages, msg)
	}

	eventsFlow <- &ControlEvent{
		evType:   EVENT_USER_REPLY,
		listener: recvChan,
		reply:    reply,
	}
}

func processGetTimeline(req *RequestGetTimeline, seqId int, recvChan chan interface{}, userId uint64) {
	dateEnd := req.DateEnd

	if dateEnd == "" {
		dateEnd = fmt.Sprint(time.Now().UnixNano())
	}

	limit := req.Limit
	if limit > MAX_TIMELINE_LIMIT {
		limit = MAX_TIMELINE_LIMIT
	}

	if limit <= 0 {
		sendError(seqId, recvChan, "Limit must be greater than 0")
		return
	}

	rows, err := getFromTimelineStmt.Query(userId, dateEnd, limit)
	if err != nil {
		sendError(seqId, recvChan, "Cannot select timeline")
		log.Println(err.Error())
		return
	}

	reply := new(ReplyGetTimeline)
	reply.SeqId = seqId
	reply.Type = "REPLY_GET_TIMELINE"
	reply.Messages = make([]TimelineMessage, 0)

	defer rows.Close()
	for rows.Next() {
		var msg TimelineMessage
		if err = rows.Scan(&msg.Id, &msg.UserId, &msg.UserName, &msg.Text, &msg.Ts); err != nil {
			log.Println(err.Error())
			return
		}

		reply.Messages = append(reply.Messages, msg)
	}

	eventsFlow <- &ControlEvent{
		evType:   EVENT_USER_REPLY,
		listener: recvChan,
		reply:    reply,
	}
}

func processSendMessage(req *RequestSendMessage, seqId int, recvChan chan interface{}, userId uint64) {
	// TODO: verify that user has rights to send message to the specified person
	var (
		err error
		now = time.Now().UnixNano()
	)

	_, err = sendMessageStmt.Exec(userId, req.UserTo, MSG_TYPE_OUT, req.Text, now)
	if err != nil {
		log.Println(err.Error())
		sendError(seqId, recvChan, "Could not log outgoing message")
		return
	}

	_, err = sendMessageStmt.Exec(req.UserTo, userId, MSG_TYPE_IN, req.Text, now)
	if err != nil {
		log.Println(err.Error())
		sendError(seqId, recvChan, "Could not log incoming message")
		return
	}

	reply := new(ReplyGeneric)
	reply.SeqId = seqId
	reply.Type = "REPLY_GENERIC"
	reply.Success = true

	eventsFlow <- &ControlEvent{
		evType:   EVENT_USER_REPLY,
		listener: recvChan,
		reply:    reply,
	}

	eventsFlow <- &ControlEvent{
		evType:   EVENT_NEW_MESSAGE,
		listener: recvChan,
		info: map[string]string{
			"UserFrom": fmt.Sprint(userId),
			"UserTo":   fmt.Sprint(req.UserTo),
			"Ts":       fmt.Sprint(now),
			"Text":     req.Text,
		},
	}
}

func EventsHandler(ws *websocket.Conn) {
	var info map[string]string

	if info = getAuthUserInfo(ws.Request().Cookies()); info == nil {
		ws.Write([]byte("AUTH_ERROR"))
		return
	}

	userId, err := strconv.ParseUint(info["id"], 10, 64)
	if err != nil {
		ws.Write([]byte("Cannot parse user id"))
		log.Println("Corrupt memcache user info: ", err.Error())
		return
	}

	recvChan := make(chan interface{}, 100)
	eventsFlow <- &ControlEvent{evType: EVENT_USER_CONNECTED, info: info, listener: recvChan}
	defer func() { eventsFlow <- &ControlEvent{evType: EVENT_USER_DISCONNECTED, info: info, listener: recvChan} }()

	go func() {
		for {
			req := new(BaseRequest)

			if err := websocket.JSON.Receive(ws, req); err != nil {
				ws.Close()
				recvChan <- nil
				return
			}

			if req.Type == "REQUEST_GET_MESSAGES" {
				userReq := new(RequestGetMessages)
				if err := json.Unmarshal([]byte(req.ReqData), userReq); err != nil {
					sendError(req.SeqId, recvChan, "Cannot unmarshal "+req.ReqData+": "+err.Error())
					continue
				}

				go processGetMessages(userReq, req.SeqId, recvChan, userId)
			} else if req.Type == "REQUEST_SEND_MESSAGE" {
				userReq := new(RequestSendMessage)
				if err := json.Unmarshal([]byte(req.ReqData), userReq); err != nil {
					sendError(req.SeqId, recvChan, "Cannot unmarshal "+req.ReqData+": "+err.Error())
					continue
				}

				go processSendMessage(userReq, req.SeqId, recvChan, userId)
			} else if req.Type == "REQUEST_GET_TIMELINE" {
				userReq := new(RequestGetTimeline)
				if err := json.Unmarshal([]byte(req.ReqData), userReq); err != nil {
					sendError(req.SeqId, recvChan, "Cannot unmarshal "+req.ReqData+": "+err.Error())
					continue
				}

				go processGetTimeline(userReq, req.SeqId, recvChan, userId)
			} else {
				sendError(req.SeqId, recvChan, "Invalid request type: "+req.Type)
				continue
			}
		}
	}()

	for ev := range recvChan {
		if ev == nil {
			return
		}

		if err := websocket.JSON.Send(ws, ev); err != nil {
			fmt.Println("Could not send JSON: " + err.Error())
			return
		}
	}
}

func init() {
	initSession()
}

func initStmts(db *sql.DB) {
	var err error

	loginStmt, err = db.Prepare("SELECT id, password, name FROM User WHERE email = ?")
	if err != nil {
		log.Fatal("Could not prepare email select: " + err.Error())
	}

	getMessagesStmt, err = db.Prepare(`SELECT id, message, ts, msg_type
		FROM Messages
		WHERE user_id = ? AND user_id_to = ? AND ts < ?
		ORDER BY ts DESC
		LIMIT ?`)
	if err != nil {
		log.Fatal("Could not prepare messages select: " + err.Error())
	}

	sendMessageStmt, err = db.Prepare(`INSERT INTO Messages
		(user_id, user_id_to, msg_type, message, ts)
		VALUES(?, ?, ?, ?, ?)`)
	if err != nil {
		log.Fatal("Could not prepare messages select: " + err.Error())
	}

	addToTimelineStmt, err = db.Prepare(`INSERT INTO Timeline
		(user_id, source_user_id, message, ts)
		VALUES(?, ?, ?, ?)`)
	if err != nil {
		log.Fatal("Could not prepare add to timeline: " + err.Error())
	}

	getFromTimelineStmt, err = db.Prepare(`SELECT t.id, t.source_user_id, u.name, t.message, t.ts
		FROM Timeline t
		LEFT JOIN User u ON u.id = t.source_user_id
		WHERE t.user_id = ? AND t.ts < ?
		ORDER BY t.ts DESC
		LIMIT ?`)
	if err != nil {
		log.Fatal("Could not prepare get from timeline: " + err.Error())
	}
}

func main() {
	var err error

	db, err := sql.Open("mysql", "root:root@tcp(127.0.0.1:3306)/social")
	if err != nil {
		log.Fatal("Could not connect to db: " + err.Error())
	}

	initStmts(db)

	http.Handle("/events", websocket.Handler(EventsHandler))
	go EventsDispatcher()

	http.HandleFunc("/static/", StaticServer)
	http.HandleFunc("/login", LoginHandler)
	http.HandleFunc("/logout", LogoutHandler)
	http.HandleFunc("/", IndexHandler)
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
