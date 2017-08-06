package db

import (
	"database/sql"
	"log"
	"strings"
)

type (
	City struct {
		Id   uint64
		Name string
		Lon  float64
		Lat  float64
	}
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
	GetUsersListStmt              *sql.Stmt
	GetFriendsList                *sql.Stmt
	GetFriendsCount               *sql.Stmt
	GetFriendsRequestList         *sql.Stmt
	GetRequestedAcceptedForFriend *sql.Stmt

	// Friends
	AddFriendsRequestStmt *sql.Stmt
	ConfirmFriendshipStmt *sql.Stmt

	// Profile
	GetProfileStmt    *sql.Stmt
	AddProfileStmt    *sql.Stmt
	UpdateProfileStmt *sql.Stmt

	// City
	GetCityInfoStmt       *sql.Stmt
	GetCityInfoByNameStmt *sql.Stmt
	AddCityStmt           *sql.Stmt
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
	RegisterStmt = prepareStmt(Db, "INSERT INTO socialuser(email, password, name, have_avatar) VALUES($1, $2, $3, false) RETURNING id")
	GetFriendsList = prepareStmt(Db, `SELECT friend_user_id FROM friend WHERE user_id = $1 AND request_accepted = true`)
	GetFriendsCount = prepareStmt(Db, `SELECT COUNT(*) FROM friend WHERE user_id = $1 AND request_accepted = true`)
	GetFriendsRequestList = prepareStmt(Db, `SELECT friend_user_id FROM friend WHERE user_id = $1 AND request_accepted = false`)
	GetFriendsRequestList = prepareStmt(Db, `SELECT friend_user_id FROM friend WHERE user_id = $1 AND request_accepted = false`)
	GetRequestedAcceptedForFriend = prepareStmt(Db, `SELECT request_accepted FROM friend WHERE user_id = $1 AND friend_user_id = $2`)

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
		WHERE id > $1
		ORDER BY id
		LIMIT $2`)

	GetProfileStmt = prepareStmt(Db, `SELECT
			name, birthdate, sex, description, city_id, family_position
		FROM userinfo
		WHERE user_id = $1`)

	AddProfileStmt = prepareStmt(Db, `INSERT INTO userinfo
			(user_id, name, birthdate, sex, description, city_id, family_position)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`)

	UpdateProfileStmt = prepareStmt(Db, `UPDATE userinfo SET
			name = $1, birthdate = $2, sex = $3, description = $4, city_id = $5, family_position = $6
			WHERE user_id = $7`)

	GetCityInfoStmt = prepareStmt(Db, `SELECT name, lon, lat FROM city WHERE id = $1`)
	GetCityInfoByNameStmt = prepareStmt(Db, `SELECT id, name, lon, lat FROM city WHERE name = $1`)
	AddCityStmt = prepareStmt(Db, `INSERT INTO city(name, lon, lat) VALUES($1, $2, $3) RETURNING id`)
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

func GetCityInfo(id uint64) (*City, error) {
	res := new(City)
	res.Id = id
	row := GetCityInfoStmt.QueryRow(res.Id)
	err := row.Scan(&res.Name, &res.Lon, &res.Lat)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func GetCityInfoByName(name string) (*City, error) {
	res := new(City)
	row := GetCityInfoByNameStmt.QueryRow(name)
	err := row.Scan(&res.Id, &res.Name, &res.Lon, &res.Lat)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func GetUserFriendsCount(userId uint64) (cnt uint64, err error) {
	err = GetFriendsCount.QueryRow(userId).Scan(&cnt)
	return
}

func GetUserFriends(userId uint64) (userIds []uint64, err error) {
	return getUsersByStmt(userId, GetFriendsList)
}

func GetUserFriendsRequests(userId uint64) (userIds []uint64, err error) {
	return getUsersByStmt(userId, GetFriendsRequestList)
}

func IsUserFriend(userId, friendId uint64) (isFriend, requestAccepted bool, err error) {
	row := GetRequestedAcceptedForFriend.QueryRow(friendId, userId)
	err = row.Scan(&requestAccepted)

	if err == sql.ErrNoRows {
		return false, false, nil
	} else if err != nil {
		return false, false, err
	}

	return true, requestAccepted, nil
}

func getUsersByStmt(userId uint64, stmt *sql.Stmt) (userIds []uint64, err error) {
	res, err := stmt.Query(userId)
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
