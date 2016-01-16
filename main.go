package main

import (
	"bufio"
	"crypto/md5"
	"crypto/sha1"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"errors"
	"github.com/BurntSushi/toml"
	"github.com/go-sql-driver/mysql"
	"golang.org/x/net/websocket"
	"reflect"
)

const (
	MAX_MESSAGES_LIMIT   = 100
	MAX_TIMELINE_LIMIT   = 100
	MAX_USERS_LIST_LIMIT = 100

	MSG_TYPE_OUT = "Out"
	MSG_TYPE_IN  = "In"
)

type (
	Config struct {
		Mysql    string
		Memcache string
		Bind     string
	}

	BaseEvent struct {
		Type string
	}

	JSUserInfo struct {
		Name string
		Id   string
	}

	JSUserListInfo struct {
		JSUserInfo
		IsFriend            bool
		FriendshipConfirmed bool
	}

	BaseRequest struct {
		SeqId   int
		Type    string
		ReqData string
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

	EventUserConnected struct {
		BaseEvent
		JSUserInfo
	}

	EventUserDisconnected struct {
		BaseEvent
		JSUserInfo
	}

	EventOnlineUsersList struct {
		BaseEvent
		Users []JSUserInfo
	}

	EventNewMessage struct {
		BaseEvent
		Message
	}

	InternalEventNewMessage struct {
		UserFrom uint64
		UserTo   uint64
		Ts       string
		Text     string
	}

	EventNewTimelineStatus struct {
		BaseEvent
		TimelineMessage
	}

	InternalEventNewTimelineStatus struct {
		UserId   uint64
		UserName string
		Ts       string
		Text     string
	}

	ControlEvent struct {
		evType   uint8
		info     interface{}
		reply    interface{}
		listener chan interface{}
	}
)

var (
	conf Config

	// Authorization
	loginStmt *sql.Stmt

	// Registration
	registerStmt *sql.Stmt

	// Messages
	getMessagesStmt *sql.Stmt
	sendMessageStmt *sql.Stmt
	getMessagesUsersStmt *sql.Stmt

	// Timeline
	getFromTimelineStmt *sql.Stmt
	addToTimelineStmt   *sql.Stmt

	// Users
	getUsersListStmt *sql.Stmt
	getFriendsList   *sql.Stmt

	// Friends
	addFriendsRequestStmt *sql.Stmt
	confirmFriendshipStmt *sql.Stmt

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

func loginUser(email, userPassword string) (sessionId string, err error) {
	var id uint64
	var password, name string

	err = loginStmt.QueryRow(email).Scan(&id, &password, &name)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Println("Db error: " + err.Error())
			err = errors.New("Sorry, an internal DB error occured")
		} else {
			err = errors.New("You are not registered, sorry")
		}
		return
	}

	if passwordHash(userPassword) != password {
		err = errors.New("Incorrect password")
		return
	}

	sessionId, err = createSession(&SessionInfo{Id: id, Name: name})
	if err != nil {
		log.Println("Could not create session: ", err.Error())
		err = errors.New("Internal error: could not create session")
		return
	}

	return
}

func LoginHandler(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	email := req.Form.Get("email")
	userPassword := req.Form.Get("password")

	if email == "" || userPassword == "" {
		fmt.Fprintf(w, "You must provide both email and password")
		return
	}

	sessionId, err := loginUser(email, userPassword)
	if err != nil {
		return
	}

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

func serveAuthPage(info *SessionInfo, w http.ResponseWriter) {
	if err := authTpl.Execute(w, info); err != nil {
		fmt.Println("Could not render template: " + err.Error())
	}
}

func getAuthUserInfo(cookies []*http.Cookie) *SessionInfo {
	for _, cook := range cookies {
		if cook.Name == "id" && cook.Value != "" {
			info, err := getSessionInfo(cook.Value)
			if err == nil {
				return info
			} else {
				log.Println("Get auth info error: " + err.Error())
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

func handleUserConnected(listenerMap map[chan interface{}]*SessionInfo, userListeners map[uint64]map[chan interface{}]bool, ev *ControlEvent) {
	evInfo, ok := ev.info.(*SessionInfo)
	if !ok {
		log.Println("VERY BAD: Type assertion failed: ev info is not SessionInfo")
		return
	}

	currentUsers := make([]JSUserInfo, 0, len(listenerMap))
	for _, info := range listenerMap {
		currentUsers = append(currentUsers, JSUserInfo{Name: info.Name, Id: fmt.Sprint(info.Id)})
	}

	ouEvent := new(EventOnlineUsersList)
	ouEvent.Type = "EVENT_ONLINE_USERS_LIST"
	ouEvent.Users = currentUsers
	ev.listener <- ouEvent

	listenerMap[ev.listener] = evInfo

	if userListeners[evInfo.Id] == nil {
		userListeners[evInfo.Id] = make(map[chan interface{}]bool)
	}

	userListeners[evInfo.Id][ev.listener] = true

	for listener := range listenerMap {
		if len(listener) >= cap(listener) {
			return
		}

		event := new(EventUserConnected)
		event.Type = "EVENT_USER_CONNECTED"
		event.Name = evInfo.Name
		event.Id = fmt.Sprint(evInfo.Id)
		listener <- event
	}
}

func handleUserDisconnected(listenerMap map[chan interface{}]*SessionInfo, userListeners map[uint64]map[chan interface{}]bool, ev *ControlEvent) {
	evInfo, ok := ev.info.(*SessionInfo)
	if !ok {
		log.Println("VERY BAD: Type assertion failed: ev info is not SessionInfo when user disconnects")
		return
	}

	delete(listenerMap, ev.listener)
	if userListeners[evInfo.Id] != nil {
		delete(userListeners[evInfo.Id], ev.listener)
		if len(userListeners[evInfo.Id]) == 0 {
			delete(userListeners, evInfo.Id)
		}
	}

	for listener := range listenerMap {
		if len(listener) >= cap(listener) {
			return
		}

		event := new(EventUserDisconnected)
		event.Type = "EVENT_USER_DISCONNECTED"
		event.Name = evInfo.Name
		event.Id = fmt.Sprint(evInfo.Id)
		listener <- event
	}
}

func handleNewMessage(listenerMap map[chan interface{}]*SessionInfo, userListeners map[uint64]map[chan interface{}]bool, ev *ControlEvent) {
	sourceEvent, ok := ev.info.(*InternalEventNewMessage)
	if !ok {
		log.Println("VERY BAD: Type assertion failed: source event is not InternalEventNewMessage in handleNewMessage")
		return
	}

	event := new(EventNewMessage)
	event.Type = "EVENT_NEW_MESSAGE"
	event.Ts = sourceEvent.Ts
	event.Text = sourceEvent.Text

	if userListeners[sourceEvent.UserFrom] != nil {
		for listener := range userListeners[sourceEvent.UserFrom] {
			fromEv := new(EventNewMessage)
			*fromEv = *event
			fromEv.UserFrom = fmt.Sprint(sourceEvent.UserTo)
			fromEv.MsgType = MSG_TYPE_OUT
			select {
			case listener <- fromEv:
			default:
			}

		}
	}

	if userListeners[sourceEvent.UserTo] != nil {
		for listener := range userListeners[sourceEvent.UserTo] {
			toEv := new(EventNewMessage)
			*toEv = *event
			toEv.UserFrom = fmt.Sprint(sourceEvent.UserFrom)
			toEv.MsgType = MSG_TYPE_IN
			select {
			case listener <- toEv:
			default:
			}
		}
	}
}

func handleNewTimelineEvent(listenerMap map[chan interface{}]*SessionInfo, userListeners map[uint64]map[chan interface{}]bool, ev *ControlEvent) {
	evInfo, ok := ev.info.(*InternalEventNewTimelineStatus)
	if !ok {
		log.Println("Type assertion failed: evInfo is not InternalEventNewTimelineStatus in handleNewTimelineEvent")
		return
	}

	for listener := range listenerMap {
		userEv := new(EventNewTimelineStatus)
		userEv.Type = "EVENT_NEW_TIMELINE_EVENT"
		userEv.Ts = evInfo.Ts
		userEv.UserId = fmt.Sprint(evInfo.UserId)
		userEv.Text = evInfo.Text
		userEv.UserName = evInfo.UserName

		select {
		case listener <- userEv:
		default:
		}
	}
}

func EventsDispatcher() {
	listenerMap := make(map[chan interface{}]*SessionInfo)
	userListeners := make(map[uint64]map[chan interface{}]bool)

	for ev := range eventsFlow {
		if ev.evType == EVENT_USER_CONNECTED {
			handleUserConnected(listenerMap, userListeners, ev)
		} else if ev.evType == EVENT_USER_DISCONNECTED {
			handleUserDisconnected(listenerMap, userListeners, ev)
		} else if ev.evType == EVENT_NEW_MESSAGE {
			handleNewMessage(listenerMap, userListeners, ev)
		} else if ev.evType == EVENT_NEW_TIMELINE_EVENT {
			handleNewTimelineEvent(listenerMap, userListeners, ev)
		} else if ev.evType == EVENT_USER_REPLY {
			if _, ok := listenerMap[ev.listener]; !ok {
				continue
			}

			select {
			case ev.listener <- ev.reply:
			default:
			}
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

// REQUEST_GET_MESSAGES => RequestGetMessages
func convertUnderscoreToCamelCase(in string) string {
	parts := strings.Split(in, "_")
	out := make([]string, 0, len(parts))
	for _, v := range parts {
		out = append(out, strings.ToUpper(v[0:1]), strings.ToLower(v[1:]))
	}
	return strings.Join(out, "")
}

func WebsocketEventsHandler(ws *websocket.Conn) {
	var userInfo *SessionInfo

	if userInfo = getAuthUserInfo(ws.Request().Cookies()); userInfo == nil {
		ws.Write([]byte("AUTH_ERROR"))
		return
	}

//	dupReader := io.TeeReader(ws, os.Stdout)
	rd := bufio.NewReader(ws)
	decoder := json.NewDecoder(rd)

	var ctx *WebsocketCtx
	ctxRefl := reflect.TypeOf(ctx)

	recvChan := make(chan interface{}, 100)
	eventsFlow <- &ControlEvent{evType: EVENT_USER_CONNECTED, info: userInfo, listener: recvChan}
	defer func() {
		eventsFlow <- &ControlEvent{evType: EVENT_USER_DISCONNECTED, info: userInfo, listener: recvChan}
	}()

	go func() {
		defer func() {
			log.Println("User ", userInfo.Name, " disconnected")
			ws.Close()
			recvChan <- nil
		}()

		for {
			reqType, err := rd.ReadString(' ')
			if err != nil {
				log.Println("Could not read request type from client: ", err.Error())
				return
			}

			reqType = reqType[:len(reqType)-1]

			seqIdStr, err := rd.ReadString('\n')
			if err != nil {
				log.Println("Could not read seq id string: ", err.Error())
				return
			}

			seqId, err := strconv.Atoi(seqIdStr[:len(seqIdStr)-1])
			if err != nil {
				log.Println("Sequence id is not int: ", err.Error())
				return
			}

			reqCamel := convertUnderscoreToCamelCase(strings.TrimPrefix(reqType, "REQUEST_"))
			method, ok := ctxRefl.MethodByName("Process" + reqCamel)
			if !ok {
				sendError(seqId, recvChan, "Invalid request type: "+reqType)
				var msg interface{}
				decoder.Decode(&msg)
				continue
			}

			reflMethodType := method.Type.In(1)

			userReq := reflect.New(reflMethodType.Elem()).Interface()

			if err := decoder.Decode(&userReq); err != nil {
				sendError(seqId, recvChan, "Cannot decode request: "+err.Error())
				continue
			}

			ctx = &WebsocketCtx{
				seqId: seqId,
				userId: userInfo.Id,
				listener: recvChan,
				userName: userInfo.Name,
			}

			resp := func () (resp interface{}) {
				defer func() {
					if r := recover(); r != nil {
						resp = &ResponseError{userMsg: "Internal error", err: fmt.Errorf("Panic on request: %s %v", reqCamel, r)}
					}
				}()

				respSlice := method.Func.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(userReq)})
				resp = respSlice[0].Interface()
				return
			}()

			switch v := resp.(type) {
			case *ResponseError:
				log.Println(v.err.Error())
				sendError(seqId, recvChan, v.userMsg)
			default:
				eventsFlow <- &ControlEvent{
					evType:   EVENT_USER_REPLY,
					listener: recvChan,
					reply:    v,
				}
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

func RegisterHandler(w http.ResponseWriter, req *http.Request) {
	serveStatic("static/register.html", w)
}

func registerUser(email, userPassword, name string) (err error, duplicate bool) {
	_, err = registerStmt.Exec(email, passwordHash(userPassword), name)
	if err != nil {
		if myErr, ok := err.(*mysql.MySQLError); ok && myErr.Number == 1062 { // duplicate
			err = nil
			duplicate = true
			return
		}
		log.Println("Could not register user: ", err.Error())
	}

	return
}

func DoRegisterHandler(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()

	name := req.Form.Get("name")
	email := req.Form.Get("email")
	userPassword := req.Form.Get("password")
	userPassword2 := req.Form.Get("password2")

	if name == "" || email == "" || userPassword == "" || userPassword2 == "" {
		fmt.Fprintf(w, "You must provide values for all the fields")
		return
	}

	if userPassword != userPassword2 {
		fmt.Fprintf(w, "Passwords do not match")
		return
	}

	err, dup := registerUser(email, userPassword, name)
	if err != nil {
		fmt.Fprintf(w, "Sorry, internal error occured while trying to register your user")
	} else if dup {
		fmt.Fprintf(w, "Sorry, user already exists")
	} else {
		w.Header().Add("Content-type", "text/html; charset=UTF-8")
		fmt.Fprintf(w, "Success! <a href='/'>Go to login page</a>")
	}

	return
}

func prepareStmt(db *sql.DB, stmt string) *sql.Stmt {
	res, err := db.Prepare(stmt)
	if err != nil {
		log.Fatal("Could not prepare `" + stmt + "`: " + err.Error())
	}

	return res
}

//language=MySQL
func initStmts(db *sql.DB) {
	loginStmt = prepareStmt(db, "SELECT id, password, name FROM social.User WHERE email = ?")
	registerStmt = prepareStmt(db, "INSERT INTO social.User(email, password, name) VALUES(?, ?, ?)")
	getFriendsList = prepareStmt(db, `SELECT id FROM social.User`)

	getMessagesStmt = prepareStmt(db, `SELECT id, message, ts, msg_type
		FROM social.Messages
		WHERE user_id = ? AND user_id_to = ? AND ts < ?
		ORDER BY ts DESC
		LIMIT ?`)

	sendMessageStmt = prepareStmt(db, `INSERT INTO social.Messages
		(user_id, user_id_to, msg_type, message, ts)
		VALUES(?, ?, ?, ?, ?)`)

	getMessagesUsersStmt = prepareStmt(db, `SELECT user_id_to, u.name, MAX(ts) AS max_ts
		FROM social.Messages AS m
		INNER JOIN social.User AS u ON u.id = m.user_id_to
		WHERE user_id = ?
		GROUP BY user_id_to
		ORDER BY max_ts DESC
		LIMIT ?`)

	addToTimelineStmt = prepareStmt(db, `INSERT INTO social.Timeline
		(user_id, source_user_id, message, ts)
		VALUES(?, ?, ?, ?)`)

	addFriendsRequestStmt = prepareStmt(db, `INSERT INTO social.Friend
		(user_id, friend_user_id, request_accepted)
		VALUES(?, ?, ?)`)

	confirmFriendshipStmt = prepareStmt(db, `UPDATE social.Friend
		SET request_accepted = 1
		WHERE user_id = ? AND friend_user_id = ?`)

	getFromTimelineStmt = prepareStmt(db, `SELECT t.id, t.source_user_id, u.name, t.message, t.ts
		FROM social.Timeline t
		LEFT JOIN social.User u ON u.id = t.source_user_id
		WHERE t.user_id = ? AND t.ts < ?
		ORDER BY t.ts DESC
		LIMIT ?`)

	getUsersListStmt = prepareStmt(db, `SELECT
			u.name, u.id, IF(f.id IS NOT NULL, 1, 0) AS is_friend, f.request_accepted
		FROM social.User AS u
		LEFT JOIN social.Friend AS f ON u.id = f.friend_user_id AND f.user_id = ?
		ORDER BY id
		LIMIT ?`)
}

func parseConfig(path string) {
	fp, err := os.Open(path)
	if err != nil {
		log.Fatal("Could not open config " + err.Error())
	}

	defer fp.Close()

	contents, err := ioutil.ReadAll(fp)
	if err != nil {
		log.Fatal("Could not read config: " + err.Error())
	}

	if _, err = toml.Decode(string(contents), &conf); err != nil {
		log.Fatal("Could not parse config: " + err.Error())
	}
}

func listen(addr string) {
	log.Fatal("ListenAndServe: ", http.ListenAndServe(addr, nil))
}

func main() {
	var (
		err        error
		configPath string
		testMode   bool
	)

	flag.StringVar(&configPath, "c", "config.toml", "Path to application config")
	flag.BoolVar(&testMode, "test-mode", false, "Do self-testing")
	flag.Parse()

	parseConfig(configPath)

	db, err := sql.Open("mysql", conf.Mysql)
	if err != nil {
		log.Fatal("Could not connect to db: " + err.Error())
	}

	initStmts(db)
	initSession()

	http.Handle("/events", websocket.Handler(WebsocketEventsHandler))
	go EventsDispatcher()

	http.HandleFunc("/static/", StaticServer)
	http.HandleFunc("/login", LoginHandler)
	http.HandleFunc("/logout", LogoutHandler)
	http.HandleFunc("/register", RegisterHandler)
	http.HandleFunc("/do-register", DoRegisterHandler)
	http.HandleFunc("/", IndexHandler)

	go listen(conf.Bind)

	if testMode {
		err := runTest(conf.Bind)
		if err == nil {
			log.Print("SUCCESS!")
		} else {
			log.Fatalf("FAILURE: %s", err.Error())
		}
	} else {
		var nilCh chan bool
		<-nilCh
	}
}
