package db

import (
	"database/sql"
	"log"
	"strings"
)

var (
	Db *sql.DB

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

//language=PostgreSQL
func InitStmts() {
	LoginStmt = prepareStmt(Db, "SELECT id, password, name FROM socialuser WHERE email = $1")
	RegisterStmt = prepareStmt(Db, "INSERT INTO socialuser(email, password, name) VALUES($1, $2, $3) RETURNING id")
	GetFriendsList = prepareStmt(Db, `SELECT id FROM socialuser`)

	GetMessagesStmt = prepareStmt(Db, `SELECT id, message, ts, is_out
		FROM messages
		WHERE user_id = $1 AND user_id_to = $2 AND ts < $3
		ORDER BY ts DESC
		LIMIT $4`)

	SendMessageStmt = prepareStmt(Db, `INSERT INTO messages
		(user_id, user_id_to, is_out, message, ts)
		VALUES($1, $2, $3, $4, $5)
		RETURNING id`)

	GetMessagesUsersStmt = prepareStmt(Db, `SELECT user_id_to, MAX(ts) AS max_ts
		FROM messages AS m
		WHERE user_id = $1
		GROUP BY user_id_to
		ORDER BY max_ts DESC
		LIMIT $2`)

	AddToTimelineStmt = prepareStmt(Db, `INSERT INTO timeline
		(user_id, source_user_id, message, ts)
		VALUES($1, $2, $3, $4)
		RETURNING id`)

	AddFriendsRequestStmt = prepareStmt(Db, `INSERT INTO friend
		(user_id, friend_user_id, request_accepted)
		VALUES($1, $2, $3)
		RETURNING id`)

	ConfirmFriendshipStmt = prepareStmt(Db, `UPDATE friend
		SET request_accepted = TRUE
		WHERE user_id = $1 AND friend_user_id = $2`)

	GetFromTimelineStmt = prepareStmt(Db, `SELECT t.id, t.source_user_id, t.message, t.ts
		FROM timeline t
		WHERE t.user_id = $1 AND t.ts < $2
		ORDER BY t.ts DESC
		LIMIT $3`)

	GetUsersListStmt = prepareStmt(Db, `SELECT
			u.name, u.id
		FROM socialuser AS u
		ORDER BY id
		LIMIT $1`)
}

// user id is string for simplicity
func GetUserNames(userIds []string) (map[string]string, error) {
	userNames := make(map[string]string)

	if len(userIds) > 0 {
		rows, err := Db.Query(`SELECT id, name FROM socialuser WHERE id IN(` + strings.Join(userIds, ",") + `)`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var id string
			var name string
			if err = rows.Scan(&id, &name); err != nil {
				return nil, err
			}

			userNames[id] = name
		}
	}

	return userNames, nil
}
