package db

import (
	"database/sql"
	"log"
)

var (

	// Authorization
	LoginStmt *sql.Stmt

	// Registration
	RegisterStmt *sql.Stmt

	// Messages
	GetMessagesStmt      *sql.Stmt
	SendMessageStmt      *sql.Stmt
	GetMessagesUsersStmt *sql.Stmt

	// Timeline
	GetFromTimelineStmt *sql.Stmt
	AddToTimelineStmt   *sql.Stmt

	// Users
	GetUsersListStmt *sql.Stmt
	GetFriendsList   *sql.Stmt

	// Friends
	AddFriendsRequestStmt *sql.Stmt
	ConfirmFriendshipStmt *sql.Stmt
)

func prepareStmt(db *sql.DB, stmt string) *sql.Stmt {
	res, err := db.Prepare(stmt)
	if err != nil {
		log.Fatal("Could not prepare `" + stmt + "`: " + err.Error())
	}

	return res
}

//language=MySQL
func InitStmts(db *sql.DB) {
	LoginStmt = prepareStmt(db, "SELECT id, password, name FROM social.User WHERE email = ?")
	RegisterStmt = prepareStmt(db, "INSERT INTO social.User(email, password, name) VALUES(?, ?, ?)")
	GetFriendsList = prepareStmt(db, `SELECT id FROM social.User`)

	GetMessagesStmt = prepareStmt(db, `SELECT id, message, ts, msg_type
		FROM social.Messages
		WHERE user_id = ? AND user_id_to = ? AND ts < ?
		ORDER BY ts DESC
		LIMIT ?`)

	SendMessageStmt = prepareStmt(db, `INSERT INTO social.Messages
		(user_id, user_id_to, msg_type, message, ts)
		VALUES(?, ?, ?, ?, ?)`)

	GetMessagesUsersStmt = prepareStmt(db, `SELECT user_id_to, u.name, MAX(ts) AS max_ts
		FROM social.Messages AS m
		INNER JOIN social.User AS u ON u.id = m.user_id_to
		WHERE user_id = ?
		GROUP BY user_id_to
		ORDER BY max_ts DESC
		LIMIT ?`)

	AddToTimelineStmt = prepareStmt(db, `INSERT INTO social.Timeline
		(user_id, source_user_id, message, ts)
		VALUES(?, ?, ?, ?)`)

	AddFriendsRequestStmt = prepareStmt(db, `INSERT INTO social.Friend
		(user_id, friend_user_id, request_accepted)
		VALUES(?, ?, ?)`)

	ConfirmFriendshipStmt = prepareStmt(db, `UPDATE social.Friend
		SET request_accepted = 1
		WHERE user_id = ? AND friend_user_id = ?`)

	GetFromTimelineStmt = prepareStmt(db, `SELECT t.id, t.source_user_id, u.name, t.message, t.ts
		FROM social.Timeline t
		LEFT JOIN social.User u ON u.id = t.source_user_id
		WHERE t.user_id = ? AND t.ts < ?
		ORDER BY t.ts DESC
		LIMIT ?`)

	GetUsersListStmt = prepareStmt(db, `SELECT
			u.name, u.id, IF(f.id IS NOT NULL, 1, 0) AS is_friend, f.request_accepted
		FROM social.User AS u
		LEFT JOIN social.Friend AS f ON u.id = f.friend_user_id AND f.user_id = ?
		ORDER BY id
		LIMIT ?`)
}
