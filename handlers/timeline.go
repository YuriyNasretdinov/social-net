package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/YuriyNasretdinov/social-net/db"
	"github.com/YuriyNasretdinov/social-net/events"
	"github.com/YuriyNasretdinov/social-net/protocol"
	"github.com/cockroachdb/cockroach-go/crdb"
)

func getTimelineIDsForHash(hashID, dateEnd, limit uint64) (ids []uint64, err error) {
	rows, err := db.Db.Query(fmt.Sprintf(`SELECT timeline_id
		FROM hashtimeline
		WHERE hash_id = %d AND ts < %d
		ORDER BY ts DESC
		LIMIT %d`, hashID, dateEnd, limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id uint64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}

	return ids, nil
}

func (ctx *WebsocketCtx) ProcessGetTimelineForHash(req *protocol.RequestGetTimelineForHash) protocol.Reply {
	dateEnd := req.DateEnd

	if dateEnd == 0 {
		dateEnd = uint64(time.Now().UnixNano())
	}

	limit := req.Limit
	if limit > protocol.MAX_TIMELINE_LIMIT {
		limit = protocol.MAX_TIMELINE_LIMIT
	}

	if limit <= 0 {
		return &protocol.ResponseError{UserMsg: "Limit must be greater than 0"}
	}

	hashIDMap, err := getHashIDs(db.Db, []string{req.Hash})
	if err != nil {
		return &protocol.ResponseError{UserMsg: "Internal error while getting hashes", Err: err}
	}

	timelineIDs, err := getTimelineIDsForHash(hashIDMap[req.Hash], dateEnd, limit)
	if err != nil {
		return &protocol.ResponseError{UserMsg: "Internal error while getting timeline for hashes", Err: err}
	}

	return getTimeline(&getTimelineQuery{timelineIDs: timelineIDs})
}

type getTimelineQuery struct {
	// either of these params groups must be set
	timelineIDs []uint64

	userID  uint64
	dateEnd string
	limit   uint64
}

func getTimeline(q *getTimelineQuery) protocol.Reply {
	var rows *sql.Rows
	var err error

	reply := new(protocol.ReplyGetTimeline)
	reply.Messages = make([]protocol.TimelineMessage, 0)

	if q.userID != 0 {
		rows, err = db.GetFromTimelineStmt.Query(q.userID, q.dateEnd, q.limit)
	} else {
		if len(q.timelineIDs) == 0 {
			return reply
		}

		rows, err = db.Db.Query(`SELECT id, source_user_id, message, ts
			FROM timeline
			WHERE id IN(` + db.INuint(q.timelineIDs) + `)
			ORDER BY ts DESC`)
	}

	if err != nil {
		return &protocol.ResponseError{UserMsg: "Cannot select timeline", Err: err}
	}

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

	return getTimeline(&getTimelineQuery{
		userID:  ctx.UserId,
		dateEnd: dateEnd,
		limit:   limit,
	})
}

func insertTimeline(tx *sql.Tx, userID uint64, userIDs []uint64, text string, now int64) (ids []uint64, err error) {
	var args = make([]interface{}, 0, len(userIDs)*4)
	var values = make([]string, 0, len(userIDs))

	var cnt = 1

	for _, uid := range userIDs {
		values = append(values, fmt.Sprintf(
			`($%d, $%d, $%d, $%d)`,
			cnt, cnt+1, cnt+2, cnt+3,
		))
		cnt += 4
		args = append(args, uid, userID, text, now)
	}

	rows, err := tx.Query(
		`INSERT INTO timeline
		(user_id, source_user_id, message, ts)
		VALUES `+strings.Join(values, ", ")+`
		RETURNING id`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id uint64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}

		ids = append(ids, id)
	}

	return ids, nil
}

var hashTagRegex = regexp.MustCompile(`#((?:\pL|[0-9_])+)`)

// Parses "hello #vbambuke" and returns []string{"vbambuke"}
func extractHashTags(text string) (result []string) {
	matches := hashTagRegex.FindAllStringSubmatch(text, -1)

	for _, m := range matches {
		result = append(result, m[1])
	}

	return result
}

func getHashIDs(tx db.Querier, hashtags []string) (map[string]uint64, error) {
	if len(hashtags) == 0 {
		return nil, nil
	}

	res := make(map[string]uint64, len(hashtags))

	rows, err := tx.Query(`SELECT id, name FROM hashes WHERE name IN(` + db.INstr(hashtags) + `)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id uint64
		var name string

		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}

		res[name] = id
	}

	return res, nil
}

func getOrCreateHashIDs(tx *sql.Tx, hashtags []string) (map[string]uint64, error) {
	missing := make(map[string]struct{}, len(hashtags))
	for _, tag := range hashtags {
		missing[tag] = struct{}{}
	}

	nameToIDMap, err := getHashIDs(tx, hashtags)
	if err != nil {
		return nil, err
	}

	for name := range nameToIDMap {
		delete(missing, name)
	}

	if len(missing) == 0 {
		return nameToIDMap, nil
	}

	values := make([]string, 0, len(missing))
	for name := range missing {
		values = append(values, fmt.Sprintf(`('%s')`, db.Escape(name)))
	}

	rows, err := tx.Query(`INSERT INTO hashes(name) VALUES ` + strings.Join(values, ", ") + ` RETURNING id, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id uint64
		var name string

		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}

		nameToIDMap[name] = id
	}

	return nameToIDMap, nil
}

func insertHashTimeline(tx *sql.Tx, timelineID uint64, hashIDs []uint64, now int64) error {
	values := make([]string, 0, len(hashIDs))
	for _, id := range hashIDs {
		values = append(values, fmt.Sprintf(`(%d, %d, %d)`, id, timelineID, now))
	}

	_, err := tx.Exec(`INSERT INTO hashtimeline(hash_id, timeline_id, ts)
		VALUES ` + strings.Join(values, ", "))
	return err
}

func (ctx *WebsocketCtx) ProcessAddToTimeline(req *protocol.RequestAddToTimeline) protocol.Reply {
	var (
		err error
		now = time.Now().UnixNano()
	)

	if len(req.Text) == 0 {
		return &protocol.ResponseError{UserMsg: "Text must not be empty"}
	} else if utf8.RuneCountInString(req.Text) > maxTimelineLength {
		return &protocol.ResponseError{UserMsg: fmt.Sprintf("Text cannot exceed %d characters", maxTimelineLength)}
	}

	hashTags := extractHashTags(req.Text)

	userIds, err := db.GetUserFriends(ctx.UserId)
	if err != nil {
		return &protocol.ResponseError{UserMsg: "Could not get user ids", Err: err}
	}

	userIds = append(userIds, ctx.UserId)

	err = crdb.ExecuteTx(context.Background(), db.Db, nil, func(tx *sql.Tx) error {
		ids, err := insertTimeline(tx, ctx.UserId, userIds, req.Text, now)
		if err != nil {
			return err
		}

		nameToIDMap, err := getOrCreateHashIDs(tx, hashTags)
		if err != nil {
			return err
		}

		if len(nameToIDMap) == 0 {
			return nil
		}

		hashIDs := make([]uint64, 0, len(nameToIDMap))
		for _, id := range nameToIDMap {
			hashIDs = append(hashIDs, id)
		}

		if err := insertHashTimeline(tx, ids[0], hashIDs, now); err != nil {
			return err
		}

		return nil
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
