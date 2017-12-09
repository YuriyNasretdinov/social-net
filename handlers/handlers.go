package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/YuriyNasretdinov/social-net/db"
	"github.com/YuriyNasretdinov/social-net/events"
	"github.com/YuriyNasretdinov/social-net/protocol"
	"github.com/cockroachdb/cockroach-go/crdb"
)

const (
	DATE_FORMAT = "2006-01-02"
)

type (
	WebsocketCtx struct {
		SeqId    int
		UserId   uint64
		Listener chan interface{}
		UserName string
	}
)

func (ctx *WebsocketCtx) ProcessGetMessages(req *protocol.RequestGetMessages) protocol.Reply {
	dateEnd := req.DateEnd

	if dateEnd == "" {
		dateEnd = fmt.Sprint(time.Now().UnixNano())
	}

	limit := req.Limit
	if limit > protocol.MAX_MESSAGES_LIMIT {
		limit = protocol.MAX_MESSAGES_LIMIT
	}

	if limit <= 0 {
		return &protocol.ResponseError{UserMsg: "Limit must be greater than 0"}
	}

	rows, err := db.GetMessagesStmt.Query(ctx.UserId, req.UserTo, dateEnd, limit)
	if err != nil {
		return &protocol.ResponseError{UserMsg: "Cannot select messages", Err: err}
	}

	reply := new(protocol.ReplyMessagesList)
	reply.Messages = make([]protocol.Message, 0)

	defer rows.Close()
	for rows.Next() {
		var msg protocol.Message
		if err = rows.Scan(&msg.Id, &msg.Text, &msg.Ts, &msg.IsOut); err != nil {
			return &protocol.ResponseError{UserMsg: "Cannot select messages", Err: err}
		}
		msg.UserFrom = fmt.Sprint(req.UserTo)
		reply.Messages = append(reply.Messages, msg)
	}

	return reply
}

func (ctx *WebsocketCtx) ProcessGetUsersList(req *protocol.RequestGetUsersList) protocol.Reply {
	limit := req.Limit
	if limit > protocol.MAX_USERS_LIST_LIMIT {
		limit = protocol.MAX_USERS_LIST_LIMIT
	}

	if limit <= 0 {
		return &protocol.ResponseError{UserMsg: "Limit must be greater than 0"}
	}

	var rows *sql.Rows
	var err error

	if req.Search == "" {
		rows, err = db.GetUsersListStmt.Query(req.MinId, limit)
		if err != nil {
			return &protocol.ResponseError{UserMsg: "Cannot select users", Err: err}
		}
	} else {
		rows, err = db.GetUsersListWithSearchStmt.Query(req.MinId, "%"+req.Search+"%", limit)
		if err != nil {
			return &protocol.ResponseError{UserMsg: "Cannot select users", Err: err}
		}
	}

	reply := new(protocol.ReplyUsersList)
	reply.Users = make([]protocol.JSUserListInfo, 0)

	potentialFriends := make([]string, 0)

	defer rows.Close()
	for rows.Next() {
		var user protocol.JSUserListInfo
		var potentialFriendId int64

		if err = rows.Scan(&user.Name, &potentialFriendId); err != nil {
			return &protocol.ResponseError{UserMsg: "Cannot select users", Err: err}
		}

		user.Id = fmt.Sprint(potentialFriendId)
		reply.Users = append(reply.Users, user)
		potentialFriends = append(potentialFriends, user.Id)
	}

	friendsMap := make(map[string]bool)

	if len(potentialFriends) > 0 {
		friendRows, err := db.Db.Query(`SELECT friend_user_id, request_accepted FROM friend
		WHERE user_id = ` + fmt.Sprint(ctx.UserId) + ` AND friend_user_id IN(` + strings.Join(potentialFriends, ",") + `)`)
		if err != nil {
			return &protocol.ResponseError{UserMsg: "Cannot select users", Err: err}
		}
		defer friendRows.Close()

		for friendRows.Next() {
			var friendId string
			var requestAccepted bool
			if err = friendRows.Scan(&friendId, &requestAccepted); err != nil {
				return &protocol.ResponseError{UserMsg: "Cannot select users", Err: err}
			}

			friendsMap[friendId] = requestAccepted
		}
	}

	for i, user := range reply.Users {
		reply.Users[i].FriendshipConfirmed, reply.Users[i].IsFriend = friendsMap[user.Id]
	}

	return reply
}

func (ctx *WebsocketCtx) ProcessGetFriends(req *protocol.RequestGetFriends) protocol.Reply {
	limit := req.Limit
	if limit > protocol.MAX_FRIENDS_LIMIT {
		limit = protocol.MAX_FRIENDS_LIMIT
	}

	if limit <= 0 {
		return &protocol.ResponseError{UserMsg: "Limit must be greater than 0"}
	}

	friendUserIds, err := db.GetUserFriends(ctx.UserId)
	if err != nil {
		return &protocol.ResponseError{UserMsg: "Could not get friends", Err: err}
	}

	friendRequestUserIds, err := db.GetUserFriendsRequests(ctx.UserId)
	if err != nil {
		return &protocol.ResponseError{UserMsg: "Could not get friends", Err: err}
	}

	reply := new(protocol.ReplyGetFriends)
	reply.Users = make([]protocol.JSUserInfo, 0)
	reply.FriendRequests = make([]protocol.JSUserInfo, 0)

	friendUserIdsStr := make([]string, 0)

	for _, userId := range friendUserIds {
		userIdStr := fmt.Sprint(userId)
		reply.Users = append(reply.Users, protocol.JSUserInfo{Id: userIdStr})
		friendUserIdsStr = append(friendUserIdsStr, userIdStr)
	}

	for _, userId := range friendRequestUserIds {
		userIdStr := fmt.Sprint(userId)
		reply.FriendRequests = append(reply.FriendRequests, protocol.JSUserInfo{Id: userIdStr})
		friendUserIdsStr = append(friendUserIdsStr, userIdStr)
	}

	userNames, err := db.GetUserNames(friendUserIdsStr)
	if err != nil {
		return &protocol.ResponseError{UserMsg: "Could not get friends", Err: err}
	}

	for i, user := range reply.Users {
		reply.Users[i].Name = userNames[user.Id]
	}

	for i, user := range reply.FriendRequests {
		reply.FriendRequests[i].Name = userNames[user.Id]
	}

	return reply
}

func (ctx *WebsocketCtx) ProcessGetTimeline(req *protocol.RequestGetTimeline) protocol.Reply {
	dateEnd := req.DateEnd

	if dateEnd == "" {
		dateEnd = fmt.Sprint(time.Now().UnixNano())
	}

	limit := req.Limit
	if limit > protocol.MAX_TIMELINE_LIMIT {
		limit = protocol.MAX_TIMELINE_LIMIT
	}

	if limit <= 0 {
		return &protocol.ResponseError{UserMsg: "Limit must be greater than 0"}
	}

	rows, err := db.GetFromTimelineStmt.Query(ctx.UserId, dateEnd, limit)
	if err != nil {
		return &protocol.ResponseError{UserMsg: "Cannot select timeline", Err: err}
	}

	reply := new(protocol.ReplyGetTimeline)
	reply.Messages = make([]protocol.TimelineMessage, 0)

	userIds := make([]string, 0)

	defer rows.Close()
	for rows.Next() {
		var msg protocol.TimelineMessage
		if err = rows.Scan(&msg.Id, &msg.UserId, &msg.Text, &msg.Ts); err != nil {
			return &protocol.ResponseError{UserMsg: "Cannot select timeline", Err: err}
		}

		reply.Messages = append(reply.Messages, msg)
		userIds = append(userIds, msg.UserId)
	}

	userNames, err := db.GetUserNames(userIds)
	if err != nil {
		return &protocol.ResponseError{UserMsg: "Cannot select timeline", Err: err}
	}

	for i, row := range reply.Messages {
		reply.Messages[i].UserName = userNames[row.UserId]
	}

	return reply
}

func (ctx *WebsocketCtx) ProcessSendMessage(req *protocol.RequestSendMessage) protocol.Reply {
	// TODO: verify that user has rights to send message to the specified person
	var (
		err error
		now = time.Now().UnixNano()
	)

	if len(req.Text) == 0 {
		return &protocol.ResponseError{UserMsg: "Message text must not be empty"}
	}

	_, err = db.SendMessageStmt.Exec(ctx.UserId, req.UserTo, protocol.MSG_TYPE_OUT, req.Text, now)
	if err != nil {
		return &protocol.ResponseError{UserMsg: "Could not log outgoing message", Err: err}
	}

	_, err = db.SendMessageStmt.Exec(req.UserTo, ctx.UserId, protocol.MSG_TYPE_IN, req.Text, now)
	if err != nil {
		return &protocol.ResponseError{UserMsg: "Could not log incoming message", Err: err}
	}

	reply := new(protocol.ReplyGeneric)
	reply.Success = true

	events.EventsFlow <- &events.ControlEvent{
		EvType:   events.EVENT_NEW_MESSAGE,
		Listener: ctx.Listener,
		Info: &events.InternalEventNewMessage{
			UserFrom:     ctx.UserId,
			UserFromName: ctx.UserName,
			UserTo:       req.UserTo,
			Ts:           fmt.Sprint(now),
			Text:         req.Text,
		},
	}

	return reply
}

func (ctx *WebsocketCtx) ProcessAddToTimeline(req *protocol.RequestAddToTimeline) protocol.Reply {
	var (
		err error
		now = time.Now().UnixNano()
	)

	if len(req.Text) == 0 {
		return &protocol.ResponseError{UserMsg: "Text must not be empty"}
	}

	userIds, err := db.GetUserFriends(ctx.UserId)
	if err != nil {
		return &protocol.ResponseError{UserMsg: "Could not get user ids", Err: err}
	}

	userIds = append(userIds, ctx.UserId)

	err = crdb.ExecuteTx(db.Db, func(tx *sql.Tx) error {
		var args = make([]interface{}, 0, len(userIds)*4)
		var values = make([]string, 0, len(userIds))

		var cnt = 1

		for _, uid := range userIds {
			values = append(values, fmt.Sprintf(
				`($%d, $%d, $%d, $%d)`,
				cnt, cnt+1, cnt+2, cnt+3,
			))
			cnt += 4
			args = append(args, uid, ctx.UserId, req.Text, now)
		}

		_, err := tx.Exec(
			`INSERT INTO timeline
			(user_id, source_user_id, message, ts)
			VALUES `+strings.Join(values, ", "),
			args...,
		)

		return err
	})

	if err != nil {
		return &protocol.ResponseError{UserMsg: "Could not add to timeline", Err: err}
	}

	reply := new(protocol.ReplyGeneric)
	reply.Success = true

	events.EventsFlow <- &events.ControlEvent{
		EvType:   events.EVENT_NEW_TIMELINE_EVENT,
		Listener: ctx.Listener,
		Info: &events.InternalEventNewTimelineStatus{
			UserId:        ctx.UserId,
			FriendUserIds: userIds,
			UserName:      ctx.UserName,
			Ts:            fmt.Sprint(now),
			Text:          req.Text,
		},
	}

	return reply
}

func (ctx *WebsocketCtx) ProcessAddFriend(req *protocol.RequestAddFriend) protocol.Reply {
	var (
		err      error
		friendId uint64
	)

	if friendId, err = strconv.ParseUint(req.FriendId, 10, 64); err != nil {
		return &protocol.ResponseError{UserMsg: "Friend id is not numeric"}
	}

	if friendId == ctx.UserId {
		return &protocol.ResponseError{UserMsg: "You cannot add yourself as a friend"}
	}

	if _, err = db.AddFriendsRequestStmt.Exec(ctx.UserId, friendId, 1); err != nil {
		return &protocol.ResponseError{UserMsg: "Could not add user as a friend", Err: err}
	}

	if _, err = db.AddFriendsRequestStmt.Exec(friendId, ctx.UserId, 0); err != nil {
		return &protocol.ResponseError{UserMsg: "Could not add user as a friend", Err: err}
	}

	ev := &events.EventFriendRequest{}
	ev.UserId = friendId
	ev.Type = "EVENT_FRIEND_REQUEST"

	events.EventsFlow <- &events.ControlEvent{
		EvType:   events.EVENT_FRIEND_REQUEST,
		Listener: ctx.Listener,
		Reply:    ev,
	}

	reply := new(protocol.ReplyGeneric)
	reply.Success = true

	return reply
}

func (ctx *WebsocketCtx) ProcessConfirmFriendship(req *protocol.RequestConfirmFriendship) protocol.Reply {
	var (
		err      error
		friendId uint64
	)

	if friendId, err = strconv.ParseUint(req.FriendId, 10, 64); err != nil {
		return &protocol.ResponseError{UserMsg: "Friend id is not numeric"}
	}

	if _, err = db.ConfirmFriendshipStmt.Exec(ctx.UserId, friendId); err != nil {
		return &protocol.ResponseError{UserMsg: "Could not confirm friendship", Err: err}
	}

	reply := new(protocol.ReplyGeneric)
	reply.Success = true

	return reply
}

func (ctx *WebsocketCtx) ProcessGetMessagesUsers(req *protocol.RequestGetMessagesUsers) protocol.Reply {
	var (
		err error
		id  uint64
		ts  string
	)

	rows, err := db.GetMessagesUsersStmt.Query(ctx.UserId, req.Limit)
	if err != nil {
		return &protocol.ResponseError{UserMsg: "Could not get users list for messages", Err: err}
	}

	defer rows.Close()

	reply := new(protocol.ReplyGetMessagesUsers)
	reply.Users = make([]protocol.JSUserInfo, 0)

	userIds := make([]string, 0)
	usersMap := make(map[uint64]bool)

	for rows.Next() {
		if err := rows.Scan(&id, &ts); err != nil {
			return &protocol.ResponseError{UserMsg: "Could not get users list for messages", Err: err}
		}

		usersMap[id] = true

		userId := fmt.Sprint(id)
		reply.Users = append(reply.Users, protocol.JSUserInfo{Id: userId})
		userIds = append(userIds, userId)
	}

	friendIds, err := db.GetUserFriends(ctx.UserId)
	if err != nil {
		return &protocol.ResponseError{UserMsg: "Could not get users list for messages", Err: err}
	}

	for _, friendId := range friendIds {
		if usersMap[friendId] {
			continue
		}

		userId := fmt.Sprint(friendId)
		reply.Users = append(reply.Users, protocol.JSUserInfo{Id: userId})
		userIds = append(userIds, userId)
	}

	userNames, err := db.GetUserNames(userIds)
	if err != nil {
		return &protocol.ResponseError{UserMsg: "Could not get users list for messages", Err: err}
	}

	for i, user := range reply.Users {
		reply.Users[i].Name = userNames[user.Id]
	}

	return reply
}

func (ctx *WebsocketCtx) ProcessGetProfile(req *protocol.RequestGetProfile) protocol.Reply {
	reply := new(protocol.ReplyGetProfile)

	userIdStr := fmt.Sprint(req.UserId)
	userNames, err := db.GetUserNames([]string{userIdStr})
	if err != nil {
		return &protocol.ResponseError{UserMsg: "Could not get user profile", Err: err}
	}

	if len(userNames) == 0 {
		return &protocol.ResponseError{UserMsg: "No such user", Err: err}
	}

	reply.Name = userNames[userIdStr]

	row, err := db.GetProfileStmt.Query(req.UserId)
	if err != nil {
		return &protocol.ResponseError{UserMsg: "Could not get user profile", Err: err}
	}
	defer row.Close()

	var birthdate time.Time
	if !row.Next() {
		return reply
	}

	err = row.Scan(&reply.Name, &birthdate, &reply.Sex, &reply.Description, &reply.CityId, &reply.FamilyPosition)
	if err != nil {
		return &protocol.ResponseError{UserMsg: "Could not get user profile", Err: err}
	}

	reply.Birthdate = birthdate.Format(DATE_FORMAT)

	city, err := db.GetCityInfo(reply.CityId)
	if err != nil {
		log.Printf("Could not get city by id=%d for user id=%d", reply.CityId, req.UserId)
		city = &db.City{}
	}

	reply.FriendsCount, err = db.GetUserFriendsCount(req.UserId)
	if err != nil {
		log.Printf("Could not get friends count for user %d: %s", req.UserId, err.Error())
	}

	reply.IsFriend, reply.RequestAccepted, err = db.IsUserFriend(ctx.UserId, req.UserId)
	if err != nil {
		log.Printf("Could not get information about friendship for user %d: %s", req.UserId, err.Error())
	}

	reply.CityName = city.Name
	return reply
}

func (ctx *WebsocketCtx) ProcessUpdateProfile(req *protocol.RequestUpdateProfile) protocol.Reply {
	reply := new(protocol.ReplyGeneric)
	reply.Success = true

	if req.CityName == "" || req.Birthdate == "" || req.Name == "" {
		return &protocol.ResponseError{UserMsg: "All fields must be filled in"}
	}

	var cityId uint64
	city, err := db.GetCityInfoByName(req.CityName)
	if err != nil {
		res := db.AddCityStmt.QueryRow(req.CityName, 0, 0)
		if err = res.Scan(&cityId); err != nil {
			return &protocol.ResponseError{UserMsg: "Could not update user profile", Err: err}
		}
	} else {
		cityId = city.Id
	}

	row, err := db.GetProfileStmt.Query(ctx.UserId)
	if err != nil {
		return &protocol.ResponseError{UserMsg: "Could not update user profile", Err: err}
	}
	defer row.Close()

	if !row.Next() {
		_, err := db.AddProfileStmt.Exec(&ctx.UserId, &req.Name, &req.Birthdate, &req.Sex, "", &cityId, &req.FamilyPosition)
		if err != nil {
			return &protocol.ResponseError{UserMsg: "Could not update user profile", Err: err}
		}
	} else {
		_, err := db.UpdateProfileStmt.Exec(&req.Name, &req.Birthdate, &req.Sex, "", &cityId, &req.FamilyPosition, &ctx.UserId)
		if err != nil {
			return &protocol.ResponseError{UserMsg: "Could not update user profile", Err: err}
		}
	}

	return reply
}
