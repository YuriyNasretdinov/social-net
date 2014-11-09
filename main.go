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

	REQUEST_GET_MESSAGES = iota

	REPLY_ERROR = iota
	REPLY_MESSAGES_LIST

	MAX_MESSAGES_LIMIT = 100
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
		DateEnd uint64
		Limit   uint64
	}

	BaseReply struct {
		SeqId int
		Type  string
	}

	Message struct {
		Id   uint64
		Text string
		Ts   uint64
	}

	ReplyGetMessages struct {
		BaseReply
		Messages []Message
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

	ControlEvent struct {
		evType   uint8
		info     map[string]string
		reply    interface{}
		listener chan interface{}
	}
)

var (
	db              *sql.DB
	loginStmt       *sql.Stmt
	getMessagesStmt *sql.Stmt
	sendMessageStmt *sql.Stmt

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

func EventsDispatcher() {
	listenerMap := make(map[chan interface{}]map[string]string)

	for ev := range eventsFlow {
		if ev.evType == EVENT_USER_CONNECTED {
			currentUsers := make([]UserInfo, 0, len(listenerMap))
			for _, info := range listenerMap {
				currentUsers = append(currentUsers, UserInfo{Name: info["name"], Id: info["id"]})
			}

			ouEvent := new(EventOnlineUsersList)
			ouEvent.Type = "EVENT_ONLINE_USERS_LIST"
			ouEvent.Users = currentUsers
			ev.listener <- ouEvent

			listenerMap[ev.listener] = ev.info
			for listener := range listenerMap {
				if len(listener) >= cap(listener) {
					continue
				}

				event := new(EventUserConnected)
				event.Type = "EVENT_USER_CONNECTED"
				event.Name = ev.info["name"]
				event.Id = ev.info["id"]
				listener <- event
			}
		} else if ev.evType == EVENT_USER_DISCONNECTED {
			delete(listenerMap, ev.listener)
			for listener := range listenerMap {
				if len(listener) >= cap(listener) {
					continue
				}

				event := new(EventUserDisconnected)
				event.Type = "EVENT_USER_DISCONNECTED"
				event.Name = ev.info["name"]
				event.Id = ev.info["id"]
				listener <- event
			}
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

func processGetMessages(getMsgReq *RequestGetMessages, seqId int, recvChan chan interface{}, userId uint64) {
	dateEnd := getMsgReq.DateEnd
	if dateEnd == 0 {
		dateEnd = uint64(time.Now().Unix())
	}

	limit := getMsgReq.Limit
	if limit > MAX_MESSAGES_LIMIT {
		limit = MAX_MESSAGES_LIMIT
	}

	if limit <= 0 {
		sendError(seqId, recvChan, "Limit must be greater than 0")
		return
	}

	rows, err := getMessagesStmt.Query(userId, getMsgReq.UserTo, dateEnd, limit)
	if err != nil {
		sendError(seqId, recvChan, "Cannot select messages")
		log.Println(err.Error())
		return
	}

	reply := new(ReplyGetMessages)
	reply.SeqId = seqId
	reply.Type = "REPLY_MESSAGES_LIST"
	reply.Messages = make([]Message, 0)

	for rows.Next() {
		var msg Message
		if err = rows.Scan(&msg.Id, &msg.Text, &msg.Ts); err != nil {
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
				getMsgReq := new(RequestGetMessages)
				if err := json.Unmarshal([]byte(req.ReqData), getMsgReq); err != nil {
					sendError(req.SeqId, recvChan, "Cannot unmarshal: "+err.Error())
					continue
				}

				go processGetMessages(getMsgReq, req.SeqId, recvChan, userId)
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

	getMessagesStmt, err = db.Prepare(`SELECT id, message, UNIX_TIMESTAMP(ts)
		FROM Messages
		WHERE user_id = ? AND user_id_to = ? AND ts <= FROM_UNIXTIME(?)
		ORDER BY ts DESC
		LIMIT ?`)
	if err != nil {
		log.Fatal("Could not prepare messages select: " + err.Error())
	}

	sendMessageStmt, err = db.Prepare(`INSERT INTO Messages
		(user_id, user_id_to, msg_type, message, ts)
		VALUES(?, ?, ?, ?, NOW())`)
	if err != nil {
		log.Fatal("Could not prepare messages select: " + err.Error())
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
