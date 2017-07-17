package main

import (
	"bufio"
	"crypto/md5"
	"crypto/sha1"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/YuriyNasretdinov/social-net/config"
	"github.com/YuriyNasretdinov/social-net/db"
	"github.com/YuriyNasretdinov/social-net/events"
	"github.com/YuriyNasretdinov/social-net/handlers"
	"github.com/YuriyNasretdinov/social-net/protocol"
	"github.com/YuriyNasretdinov/social-net/session"
	_ "github.com/cockroachdb/cockroach-go/crdb"
	"golang.org/x/net/websocket"
)

func serveStatic(filename string, w http.ResponseWriter) {
	fp, err := os.Open(filename)
	if err != nil {
		w.WriteHeader(404)
		log.Printf("Could not find file: %s", filename)
		return
	}
	defer fp.Close()

	if strings.HasSuffix(filename, ".css") {
		w.Header().Add("Content-type", "text/css")
	} else if strings.HasSuffix(filename, ".js") {
		w.Header().Add("Content-type", "application/javascript")
	} else if strings.HasSuffix(filename, ".jpg") {
		w.Header().Add("Content-type", "image/jpeg")
	}

	io.Copy(w, fp)
}

func StaticServer(w http.ResponseWriter, req *http.Request) {
	serveStatic(req.URL.Path[len("/"):], w)
}

func avatarPath(id int) string {
	idStr := fmt.Sprintf("%03d", id)
	return fmt.Sprintf("%c/%c/%s/%d.jpg", idStr[0], idStr[1], idStr[2:], id)
}

func AvatarServer(w http.ResponseWriter, req *http.Request) {
	userIdStr := strings.TrimSuffix(req.URL.Path[len("/avatars/"):], ".jpg")
	userId, err := strconv.Atoi(userIdStr)
	if err != nil {
		w.WriteHeader(404)
		w.Write([]byte("Could not parse user id"))
		return
	}

	serveStatic(filepath.Join(config.Conf.AvatarDir, avatarPath(userId)), w)
}

func loginUser(email, userPassword string) (sessionId string, err error) {
	var id uint64
	var password, name string

	err = db.LoginStmt.QueryRow(email).Scan(&id, &password, &name)
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

	sessionId, err = session.CreateSession(&session.SessionInfo{Id: id, Name: name})
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

func serveAuthPage(sessionInfo *session.SessionInfo, w http.ResponseWriter) {
	info := new(struct {
		session.SessionInfo
		FriendsRequestsCount int
	})

	info.Id = sessionInfo.Id
	info.Name = sessionInfo.Name
	info.FriendsRequestsCount = 0

	friendsReqs, err := db.GetUserFriendsRequests(sessionInfo.Id)
	if err != nil {
		log.Println("Could not get friends requests: ", err.Error())
	} else {
		info.FriendsRequestsCount = len(friendsReqs)
	}

	if err := authTpl.Execute(w, info); err != nil {
		fmt.Println("Could not render template: " + err.Error())
	}
}

func getAuthUserInfo(cookies []*http.Cookie) *session.SessionInfo {
	for _, cook := range cookies {
		if cook.Name == "id" && cook.Value != "" {
			info, err := session.GetSessionInfo(cook.Value)
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

func sendError(seqId int, recvChan chan interface{}, message string) {
	reply := new(protocol.ReplyError)
	reply.SeqId = seqId
	reply.Type = "REPLY_ERROR"
	reply.Message = message

	events.EventsFlow <- &events.ControlEvent{
		EvType:   events.EVENT_USER_REPLY,
		Listener: recvChan,
		Reply:    reply,
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

// ReplyGetMessages => REPLY_GET_MESSAGES
func convertCamelCaseToUnderscore(in string) string {
	out := make([]rune, 0)

	for _, c := range in {
		if unicode.IsUpper(c) && len(out) > 0 {
			out = append(out, '_')
		}
		out = append(out, unicode.ToUpper(c))
	}

	return string(out)
}

func WebsocketEventsHandler(ws *websocket.Conn) {
	var userInfo *session.SessionInfo

	if userInfo = getAuthUserInfo(ws.Request().Cookies()); userInfo == nil {
		ws.Write([]byte("AUTH_ERROR"))
		return
	}

	//	dupReader := io.TeeReader(ws, os.Stdout)
	rd := bufio.NewReader(ws)
	decoder := json.NewDecoder(rd)

	var ctx *handlers.WebsocketCtx
	ctxRefl := reflect.TypeOf(ctx)

	recvChan := make(chan interface{}, 100)
	events.EventsFlow <- &events.ControlEvent{EvType: events.EVENT_USER_CONNECTED, Info: userInfo, Listener: recvChan}
	defer func() {
		events.EventsFlow <- &events.ControlEvent{EvType: events.EVENT_USER_DISCONNECTED, Info: userInfo, Listener: recvChan}
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

			start := time.Now()
			reflMethodType := method.Type.In(1)

			userReq := reflect.New(reflMethodType.Elem()).Interface()

			if err := decoder.Decode(&userReq); err != nil {
				sendError(seqId, recvChan, "Cannot decode request: "+err.Error())
				continue
			}

			ctx = &handlers.WebsocketCtx{
				SeqId:    seqId,
				UserId:   userInfo.Id,
				Listener: recvChan,
				UserName: userInfo.Name,
			}

			resp := func() (resp interface{}) {
				defer func() {
					if r := recover(); r != nil {
						resp = &protocol.ResponseError{UserMsg: "Internal error", Err: fmt.Errorf("Panic on request: %s %v", reqCamel, r)}
					}
				}()

				respSlice := method.Func.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(userReq)})
				resp = respSlice[0].Interface()
				return
			}()

			log.Printf("Processed %s, %+v in %s", reqType, userReq, time.Since(start))

			switch v := resp.(type) {
			case *protocol.ResponseError:
				if v.Err != nil {
					log.Println(reqCamel, ":", v.Err.Error())
				}
				sendError(seqId, recvChan, v.UserMsg)
			case protocol.Reply:
				v.SetSeqId(seqId)
				v.SetReplyType(convertCamelCaseToUnderscore(strings.SplitN(fmt.Sprintf("%T", v), ".", 2)[1]))
				events.EventsFlow <- &events.ControlEvent{
					EvType:   events.EVENT_USER_REPLY,
					Listener: recvChan,
					Reply:    v,
				}
			default:
				log.Panicf("Got %T that does not satisfy protocol.Reply", v)
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
	_, err = db.RegisterStmt.Exec(email, passwordHash(userPassword), name)
	if err != nil {
		// TODO: check for duplicate key in Cockroach
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

	log.SetFlags(log.Flags() | log.Lmicroseconds)
	start := time.Now()
	log.Println("Starting")

	config.ParseConfig(configPath)

	db.Db, err = sql.Open("postgres", config.Conf.Postgresql)
	if err != nil {
		log.Fatal("Could not connect to db: " + err.Error())
	}

	log.Println("Connecting to DB")

	db.InitStmts()

	log.Println("Initializing session")

	session.InitSession()

	log.Println("Registering handlers")

	http.Handle("/events", websocket.Handler(WebsocketEventsHandler))
	go events.EventsDispatcher()

	http.HandleFunc("/avatars/", AvatarServer)
	http.HandleFunc("/static/", StaticServer)
	http.HandleFunc("/login", LoginHandler)
	http.HandleFunc("/logout", LogoutHandler)
	http.HandleFunc("/register", RegisterHandler)
	http.HandleFunc("/do-register", DoRegisterHandler)
	http.HandleFunc("/", IndexHandler)

	go listen(config.Conf.Bind)

	log.Printf("Waiting for events, init done in %s", time.Since(start))

	if testMode {
		err := runTest(config.Conf.Bind)
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
