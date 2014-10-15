package main

import (
	"code.google.com/p/go.net/websocket"
	"crypto/md5"
	"crypto/sha1"
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	EVENT_USER_CONNECTED = iota
	EVENT_USER_DISCONNECTED
	EVENT_ONLINE_USERS_LIST
)

type (
	Event struct {
		evType uint8
		evData string
	}

	ControlEvent struct {
		evType   uint8
		info     map[string]string
		listener chan *Event
	}
)

var (
	db        *sql.DB
	loginStmt *sql.Stmt

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
	listenerMap := make(map[chan *Event]map[string]string)

	for ev := range eventsFlow {
		if ev.evType == EVENT_USER_CONNECTED {
			currentUsers := make([]string, 0, len(listenerMap))
			for _, info := range listenerMap {
				currentUsers = append(currentUsers, info["name"]+" #"+info["id"])
			}

			ev.listener <- &Event{evType: EVENT_ONLINE_USERS_LIST, evData: strings.Join(currentUsers, "|")}

			listenerMap[ev.listener] = ev.info
			for listener := range listenerMap {
				if len(listener) < cap(listener) {
					listener <- &Event{evType: EVENT_USER_CONNECTED, evData: ev.info["name"] + " #" + ev.info["id"]}
				}
			}
		} else if ev.evType == EVENT_USER_DISCONNECTED {
			delete(listenerMap, ev.listener)
			for listener := range listenerMap {
				if len(listener) < cap(listener) {
					listener <- &Event{evType: EVENT_USER_DISCONNECTED, evData: ev.info["name"] + " #" + ev.info["id"]}
				}
			}
		}
	}
}

func EventsHandler(ws *websocket.Conn) {
	var info map[string]string

	if info = getAuthUserInfo(ws.Request().Cookies()); info == nil {
		ws.Write([]byte("AUTH_ERROR"))
		return
	}

	recvChan := make(chan *Event, 100)
	eventsFlow <- &ControlEvent{evType: EVENT_USER_CONNECTED, info: info, listener: recvChan}
	defer func() { eventsFlow <- &ControlEvent{evType: EVENT_USER_DISCONNECTED, info: info, listener: recvChan} }()

	go func() {
		var msg [100]byte
		for {
			n, err := ws.Read(msg[:])
			if err == nil {
				fmt.Println("Read from " + info["name"] + ": " + string(msg[:n]))
			} else {
				ws.Close()
				recvChan <- nil
				return
			}
		}

	}()

	for ev := range recvChan {
		if ev == nil {
			return
		}

		var evTypeStr string
		if ev.evType == EVENT_USER_CONNECTED {
			evTypeStr = "EVENT_USER_CONNECTED"
		} else if ev.evType == EVENT_USER_DISCONNECTED {
			evTypeStr = "EVENT_USER_DISCONNECTED"
		} else if ev.evType == EVENT_ONLINE_USERS_LIST {
			evTypeStr = "EVENT_ONLINE_USERS_LIST"
		}

		_, err := ws.Write([]byte(fmt.Sprintf("%s:%s", evTypeStr, ev.evData)))
		if err != nil {
			fmt.Println("Write error: " + err.Error())
			return
		}
	}
}

func init() {
	initSession()
}

func main() {
	var err error

	db, err := sql.Open("mysql", "root:root@tcp(127.0.0.1:3306)/social")
	if err != nil {
		log.Fatal("Could not connect to db: " + err.Error())
	}

	loginStmt, err = db.Prepare("SELECT id, password, name FROM User WHERE email = ?")
	if err != nil {
		log.Fatal("Could not prepare email select: " + err.Error())
	}

	http.Handle("/events", websocket.Handler(EventsHandler))
	go EventsDispatcher()

	http.HandleFunc("/static/", StaticServer)
	http.HandleFunc("/login", LoginHandler)
	http.HandleFunc("/logout", LogoutHandler)
	http.HandleFunc("/", IndexHandler)
	fmt.Println("Hello world!")
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
