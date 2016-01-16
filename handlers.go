package main

import (
	"fmt"
	"log"
	"strconv"
	"time"
	"database/sql"
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
)

type (
	WebsocketCtx struct {
		seqId    int
		userId   uint64
		listener chan interface{}
		userName string
	}

	ResponseError struct {
		userMsg string
		err     error
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

	RequestConfirmFriendship struct {
		FriendId string
	}

	RequestGetMessagesUsers struct {
		Limit uint64
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
)

func (ctx *WebsocketCtx) ProcessGetMessages(req *RequestGetMessages) interface{} {
	dateEnd := req.DateEnd

	if dateEnd == "" {
		dateEnd = fmt.Sprint(time.Now().UnixNano())
	}

	limit := req.Limit
	if limit > MAX_MESSAGES_LIMIT {
		limit = MAX_MESSAGES_LIMIT
	}

	if limit <= 0 {
		return &ResponseError{userMsg: "Limit must be greater than 0"}
	}

	rows, err := getMessagesStmt.Query(ctx.userId, req.UserTo, dateEnd, limit)
	if err != nil {
		return &ResponseError{userMsg: "Cannot select messages", err: err}
	}

	reply := new(ReplyGetMessages)
	reply.SeqId = ctx.seqId
	reply.Type = "REPLY_MESSAGES_LIST"
	reply.Messages = make([]Message, 0)

	defer rows.Close()
	for rows.Next() {
		var msg Message
		if err = rows.Scan(&msg.Id, &msg.Text, &msg.Ts, &msg.MsgType); err != nil {
			return &ResponseError{userMsg: "Cannot select messages", err: err}
		}
		msg.UserFrom = fmt.Sprint(req.UserTo)
		reply.Messages = append(reply.Messages, msg)
	}

	return reply
}

func (ctx *WebsocketCtx) ProcessGetUsersList(req *RequestGetUsersList) interface{} {
	limit := req.Limit
	if limit > MAX_USERS_LIST_LIMIT {
		limit = MAX_USERS_LIST_LIMIT
	}

	if limit <= 0 {
		return &ResponseError{userMsg: "Limit must be greater than 0"}
	}

	rows, err := getUsersListStmt.Query(ctx.userId, limit)
	if err != nil {
		return &ResponseError{userMsg: "Cannot select users", err: err}
	}

	reply := new(ReplyGetUsersList)
	reply.SeqId = ctx.seqId
	reply.Type = "REPLY_USERS_LIST"
	reply.Users = make([]JSUserListInfo, 0)

	defer rows.Close()
	for rows.Next() {
		var user JSUserListInfo
		var isFriendInt int
		var friendshipConfirmed sql.NullInt64

		if err = rows.Scan(&user.Name, &user.Id, &isFriendInt, &friendshipConfirmed); err != nil {
			return &ResponseError{userMsg: "Cannot select users", err: err}
		}

		user.IsFriend = (isFriendInt > 0)
		user.FriendshipConfirmed = (friendshipConfirmed.Valid && friendshipConfirmed.Int64 > 0)

		reply.Users = append(reply.Users, user)
	}

	return reply
}

func (ctx *WebsocketCtx) ProcessGetTimeline(req *RequestGetTimeline) interface{} {
	dateEnd := req.DateEnd

	if dateEnd == "" {
		dateEnd = fmt.Sprint(time.Now().UnixNano())
	}

	limit := req.Limit
	if limit > MAX_TIMELINE_LIMIT {
		limit = MAX_TIMELINE_LIMIT
	}

	if limit <= 0 {
		return &ResponseError{userMsg: "Limit must be greater than 0"}
	}

	rows, err := getFromTimelineStmt.Query(ctx.userId, dateEnd, limit)
	if err != nil {
		return &ResponseError{userMsg: "Cannot select timeline", err: err}
	}

	reply := new(ReplyGetTimeline)
	reply.SeqId = ctx.seqId
	reply.Type = "REPLY_GET_TIMELINE"
	reply.Messages = make([]TimelineMessage, 0)

	defer rows.Close()
	for rows.Next() {
		var msg TimelineMessage
		if err = rows.Scan(&msg.Id, &msg.UserId, &msg.UserName, &msg.Text, &msg.Ts); err != nil {
			return &ResponseError{userMsg: "Cannot select timeline", err: err}
		}

		reply.Messages = append(reply.Messages, msg)
	}

	return reply
}

func (ctx *WebsocketCtx) ProcessSendMessage(req *RequestSendMessage) interface{} {
	// TODO: verify that user has rights to send message to the specified person
	var (
		err error
		now = time.Now().UnixNano()
	)

	_, err = sendMessageStmt.Exec(ctx.userId, req.UserTo, MSG_TYPE_OUT, req.Text, now)
	if err != nil {
		return &ResponseError{userMsg: "Could not log outgoing message", err: err}
	}

	_, err = sendMessageStmt.Exec(req.UserTo, ctx.userId, MSG_TYPE_IN, req.Text, now)
	if err != nil {
		return &ResponseError{userMsg: "Could not log incoming message", err: err}
	}

	reply := new(ReplyGeneric)
	reply.SeqId = ctx.seqId
	reply.Type = "REPLY_GENERIC"
	reply.Success = true

	eventsFlow <- &ControlEvent{
		evType:   EVENT_NEW_MESSAGE,
		listener: ctx.listener,
		info: &InternalEventNewMessage{
			UserFrom: ctx.userId,
			UserTo:   req.UserTo,
			Ts:       fmt.Sprint(now),
			Text:     req.Text,
		},
	}

	return reply
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

func (ctx *WebsocketCtx) ProcessAddToTimeline(req *RequestAddToTimeline) interface{} {
	var (
		err error
		now = time.Now().UnixNano()
	)

	userIds, err := getUserFriends(ctx.userId)
	if err != nil {
		return &ResponseError{userMsg: "Could not get user ids", err: err}
	}

	for _, uid := range userIds {
		if _, err = addToTimelineStmt.Exec(uid, ctx.userId, req.Text, now); err != nil {
			return &ResponseError{userMsg: "Could not add to timeline", err: err}
		}
	}

	reply := new(ReplyGeneric)
	reply.SeqId = ctx.seqId
	reply.Type = "REPLY_GENERIC"
	reply.Success = true

	eventsFlow <- &ControlEvent{
		evType:   EVENT_NEW_TIMELINE_EVENT,
		listener: ctx.listener,
		info: &InternalEventNewTimelineStatus{
			UserId:   ctx.userId,
			UserName: ctx.userName,
			Ts:       fmt.Sprint(now),
			Text:     req.Text,
		},
	}

	return reply
}

func (ctx *WebsocketCtx) ProcessRequestAddFriend(req *RequestAddFriend) interface{} {
	var (
		err      error
		friendId uint64
	)

	if friendId, err = strconv.ParseUint(req.FriendId, 10, 64); err != nil {
		return &ResponseError{userMsg: "Friend id is not numeric"}
	}

	if friendId == ctx.userId {
		return &ResponseError{userMsg: "You cannot add yourself as a friend"}
	}

	if _, err = addFriendsRequestStmt.Exec(ctx.userId, friendId, 1); err != nil {
		return &ResponseError{userMsg: "Could not add user as a friend", err: err}
	}

	if _, err = addFriendsRequestStmt.Exec(friendId, ctx.userId, 0); err != nil {
		return &ResponseError{userMsg: "Could not add user as a friend", err: err}
	}

	reply := new(ReplyGeneric)
	reply.SeqId = ctx.seqId
	reply.Type = "REPLY_GENERIC"
	reply.Success = true

	return reply
}

func (ctx *WebsocketCtx) ProcessConfirmFriendship(req *RequestConfirmFriendship) interface{} {
	var (
		err      error
		friendId uint64
	)

	if friendId, err = strconv.ParseUint(req.FriendId, 10, 64); err != nil {
		return &ResponseError{userMsg: "Friend id is not numeric"}
	}

	if _, err = confirmFriendshipStmt.Exec(ctx.userId, friendId); err != nil {
		return &ResponseError{userMsg: "Could not confirm friendship", err: err}
	}

	reply := new(ReplyGeneric)
	reply.SeqId = ctx.seqId
	reply.Type = "REPLY_GENERIC"
	reply.Success = true

	return reply
}

func (ctx *WebsocketCtx) ProcessGetMessagesUsers(req *RequestGetMessagesUsers) interface{} {
	var (
		err error
		id uint64
		name string
		ts string
	)

	rows, err := getMessagesUsersStmt.Query(ctx.userId, req.Limit)
	if err != nil {
		return &ResponseError{userMsg: "Could not get users list for messages", err: err}
	}

	defer rows.Close()

	reply := new(ReplyGetMessagesUsers)
	reply.SeqId = ctx.seqId
	reply.Type = "REPLY_GET_MESSAGES_USERS"
	reply.Users = make([]JSUserInfo, 0)

	for rows.Next() {
		if err := rows.Scan(&id, &name, &ts); err != nil {
			return &ResponseError{userMsg: "Could not get users list for messages", err: err}
		}

		reply.Users = append(reply.Users, JSUserInfo{Id: fmt.Sprint(id), Name: name})
	}

	return reply
}
