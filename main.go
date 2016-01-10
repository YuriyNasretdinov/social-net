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
)

const (
	EVENT_USER_CONNECTED = iota
	EVENT_USER_DISCONNECTED
	EVENT_ONLINE_USERS_LIST
	EVENT_USER_REPLY
	EVENT_NEW_MESSAGE
	EVENT_NEW_TIMELINE_EVENT

	REQUEST_GET_MESSAGES = iota
	REQUEST_SEND_MESSAGE
	REQUEST_GET_TIMELINE
	REQUEST_ADD_TO_TIMELINE
	REQUEST_GET_USERS_LIST
	REQUEST_ADD_FRIEND
	REQUEST_CONFIRM_FRIENDSHIP
	REQUEST_GET_MESSAGES_USERS

	REPLY_ERROR = iota
	REPLY_MESSAGES_LIST
	REPLY_GENERIC
	REPLY_GET_TIMELINE
	REPLY_GET_MESSAGES_USERS

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
		Text string
	}

	RequestGetUsersList struct {
		Limit uint64
	}

	RequestAddFriend struct {
		FriendId string
	}

	RequestConfirmFriend struct {
		FriendId string
	}

	RequestGetMessagesUsers struct {
		Limit uint64
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

	ReplyGetUsersList struct {
		BaseReply
		Users []JSUserListInfo
	}

	ReplyGetMessagesUsers struct {
		BaseReply
		Users []JSUserInfo
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
		if len(listener) >= cap(listener) {
			continue
		}
		userEv := new(EventNewTimelineStatus)
		userEv.Type = "EVENT_NEW_TIMELINE_EVENT"
		userEv.Ts = evInfo.Ts
		userEv.UserId = fmt.Sprint(evInfo.UserId)
		userEv.Text = evInfo.Text
		userEv.UserName = evInfo.UserName
		listener <- userEv
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
			sendError(seqId, recvChan, "Cannot select messages")
			log.Println(err.Error())
			return
		}
		msg.UserFrom = fmt.Sprint(req.UserTo)
		reply.Messages = append(reply.Messages, msg)
	}

	eventsFlow <- &ControlEvent{
		evType:   EVENT_USER_REPLY,
		listener: recvChan,
		reply:    reply,
	}
}

func processGetUsersList(req *RequestGetUsersList, seqId int, recvChan chan interface{}, userId uint64) {
	limit := req.Limit
	if limit > MAX_USERS_LIST_LIMIT {
		limit = MAX_USERS_LIST_LIMIT
	}

	if limit <= 0 {
		sendError(seqId, recvChan, "Limit must be greater than 0")
		return
	}

	rows, err := getUsersListStmt.Query(userId, limit)
	if err != nil {
		sendError(seqId, recvChan, "Cannot select users")
		log.Println(err.Error())
		return
	}

	reply := new(ReplyGetUsersList)
	reply.SeqId = seqId
	reply.Type = "REPLY_USERS_LIST"
	reply.Users = make([]JSUserListInfo, 0)

	defer rows.Close()
	for rows.Next() {
		var user JSUserListInfo
		var isFriendInt int
		var friendshipConfirmed sql.NullInt64

		if err = rows.Scan(&user.Name, &user.Id, &isFriendInt, &friendshipConfirmed); err != nil {
			log.Println(err.Error())
			return
		}

		user.IsFriend = (isFriendInt > 0)
		user.FriendshipConfirmed = (friendshipConfirmed.Valid && friendshipConfirmed.Int64 > 0)

		reply.Users = append(reply.Users, user)
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
		info: &InternalEventNewMessage{
			UserFrom: userId,
			UserTo:   req.UserTo,
			Ts:       fmt.Sprint(now),
			Text:     req.Text,
		},
	}
}

func getUserFriends(userId uint64) (userIds []uint64, err error) {
	res, err := getFriendsList.Query()
	if err != nil {
		return
	}

	defer res.Close()

	userIds = make([]uint64, 0)

	for res.Next() {
		var uid uint64
		if err = res.Scan(&uid); err != nil {
			log.Println(err.Error())
			return
		}

		userIds = append(userIds, uid)
	}

	return
}

func processAddToTimeline(req *RequestAddToTimeline, seqId int, recvChan chan interface{}, userId uint64, userName string) {
	var (
		err error
		now = time.Now().UnixNano()
	)

	userIds, err := getUserFriends(userId)
	if err != nil {
		log.Println(err.Error())
		sendError(seqId, recvChan, "Could not get user ids")
		return
	}

	for _, uid := range userIds {
		if _, err = addToTimelineStmt.Exec(uid, userId, req.Text, now); err != nil {
			log.Println(err.Error())
			sendError(seqId, recvChan, "Could not add to timeline")
			return
		}
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
		evType:   EVENT_NEW_TIMELINE_EVENT,
		listener: recvChan,
		info: &InternalEventNewTimelineStatus{
			UserId:   userId,
			UserName: userName,
			Ts:       fmt.Sprint(now),
			Text:     req.Text,
		},
	}
}

func processRequestAddFriend(req *RequestAddFriend, seqId int, recvChan chan interface{}, userId uint64, userName string) {
	var (
		err      error
		friendId uint64
	)

	if friendId, err = strconv.ParseUint(req.FriendId, 10, 64); err != nil {
		log.Println(err.Error())
		sendError(seqId, recvChan, "Friend id is not numeric")
		return
	}

	if friendId == userId {
		sendError(seqId, recvChan, "You cannot add yourself as a friend")
		return
	}

	if _, err = addFriendsRequestStmt.Exec(userId, friendId, 1); err != nil {
		log.Println(err.Error())
		sendError(seqId, recvChan, "Could not add user as a friend")
		return
	}

	if _, err = addFriendsRequestStmt.Exec(friendId, userId, 0); err != nil {
		log.Println(err.Error())
		sendError(seqId, recvChan, "Could not add user as a friend")
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
}

func processConfirmFriendship(req *RequestConfirmFriend, seqId int, recvChan chan interface{}, userId uint64, userName string) {
	var (
		err      error
		friendId uint64
	)

	if friendId, err = strconv.ParseUint(req.FriendId, 10, 64); err != nil {
		log.Println(err.Error())
		sendError(seqId, recvChan, "Friend id is not numeric")
		return
	}

	if _, err = confirmFriendshipStmt.Exec(userId, friendId); err != nil {
		log.Println(err.Error())
		sendError(seqId, recvChan, "Could not confirm friendship")
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
}

func processGetMessagesUsers(req *RequestGetMessagesUsers, seqId int, recvChan chan interface{}, userId uint64, userName string) {
	var (
		err error
		id uint64
		name string
		ts string
	)

	rows, err := getMessagesUsersStmt.Query(userId, req.Limit)
	if err != nil {
		log.Println(err.Error())
		sendError(seqId, recvChan, "Could not get users list for messages")
		return
	}

	defer rows.Close()

	reply := new(ReplyGetMessagesUsers)
	reply.SeqId = seqId
	reply.Type = "REPLY_GET_MESSAGES_USERS"
	reply.Users = make([]JSUserInfo, 0)

	for rows.Next() {
		if err := rows.Scan(&id, &name, &ts); err != nil {
			log.Println(err.Error())
			sendError(seqId, recvChan, "Could not get users list for messages")
			return
		}

		reply.Users = append(reply.Users, JSUserInfo{Id: fmt.Sprint(id), Name: name})
	}

	eventsFlow <- &ControlEvent{
		evType:   EVENT_USER_REPLY,
		listener: recvChan,
		reply:    reply,
	}
}

func WebsocketEventsHandler(ws *websocket.Conn) {
	var userInfo *SessionInfo

	if userInfo = getAuthUserInfo(ws.Request().Cookies()); userInfo == nil {
		ws.Write([]byte("AUTH_ERROR"))
		return
	}

	rd := bufio.NewReader(ws)
	decoder := json.NewDecoder(rd)

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

			if reqType == "REQUEST_GET_MESSAGES" {
				userReq := new(RequestGetMessages)
				if err := decoder.Decode(userReq); err != nil {
					sendError(seqId, recvChan, "Cannot decode request: "+err.Error())
					continue
				}

				go processGetMessages(userReq, seqId, recvChan, userInfo.Id)
			} else if reqType == "REQUEST_SEND_MESSAGE" {
				userReq := new(RequestSendMessage)
				if err := decoder.Decode(userReq); err != nil {
					sendError(seqId, recvChan, "Cannot decode request: "+err.Error())
					continue
				}

				go processSendMessage(userReq, seqId, recvChan, userInfo.Id)
			} else if reqType == "REQUEST_GET_TIMELINE" {
				userReq := new(RequestGetTimeline)
				if err := decoder.Decode(userReq); err != nil {
					sendError(seqId, recvChan, "Cannot decode request: "+err.Error())
					continue
				}

				go processGetTimeline(userReq, seqId, recvChan, userInfo.Id)
			} else if reqType == "REQUEST_ADD_TO_TIMELINE" {
				userReq := new(RequestAddToTimeline)
				if err := decoder.Decode(userReq); err != nil {
					sendError(seqId, recvChan, "Cannot decode request: "+err.Error())
					continue
				}

				go processAddToTimeline(userReq, seqId, recvChan, userInfo.Id, userInfo.Name)
			} else if reqType == "REQUEST_GET_USERS_LIST" {
				userReq := new(RequestGetUsersList)
				if err := decoder.Decode(userReq); err != nil {
					sendError(seqId, recvChan, "Cannot decode request: "+err.Error())
					continue
				}

				go processGetUsersList(userReq, seqId, recvChan, userInfo.Id)
			} else if reqType == "REQUEST_ADD_FRIEND" {
				userReq := new(RequestAddFriend)
				if err := decoder.Decode(userReq); err != nil {
					sendError(seqId, recvChan, "Cannot decode request: "+err.Error())
					continue
				}

				go processRequestAddFriend(userReq, seqId, recvChan, userInfo.Id, userInfo.Name)
			} else if reqType == "REQUEST_CONFIRM_FRIENDSHIP" {
				userReq := new(RequestConfirmFriend)
				if err := decoder.Decode(userReq); err != nil {
					sendError(seqId, recvChan, "Cannot decode request: "+err.Error())
					continue
				}

				go processConfirmFriendship(userReq, seqId, recvChan, userInfo.Id, userInfo.Name)
			} else if reqType == "REQUEST_GET_MESSAGES_USERS" {
				userReq := new(RequestGetMessagesUsers)
				if err := decoder.Decode(userReq); err != nil {
					sendError(seqId, recvChan, "Cannot decode request: "+err.Error())
					continue
				}

				go processGetMessagesUsers(userReq, seqId, recvChan, userInfo.Id, userInfo.Name)
			} else {
				sendError(seqId, recvChan, "Invalid request type: "+reqType)
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
