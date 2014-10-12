package main

import (
	"crypto/md5"
	"crypto/sha1"
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

var (
	db        *sql.DB
	loginStmt *sql.Stmt
)

func serveStatic(filename string, w http.ResponseWriter) {
	fp, err := os.Open(filename)
	if err != nil {
		w.WriteHeader(404)
		w.Write([]byte("Could not find file: " + filename))
		return
	}
	defer fp.Close()
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

	fmt.Println("Password hash: ", passwordHash(userPassword))

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

	fmt.Fprintf(w, "You have id = %d and name = %s", id, name)
}

func passwordHash(password string) string {
	sh := sha1.New()
	io.WriteString(sh, password)

	md := md5.New()
	io.WriteString(md, password)

	return fmt.Sprintf("%x:%x", sh.Sum(nil), md.Sum(nil))
}

func serveAuthPage(info map[string]string, w http.ResponseWriter) {
	fmt.Fprintf(w, "Id: %s, Name: %s", info["id"], info["name"])
}

func IndexHandler(w http.ResponseWriter, req *http.Request) {
	// validate session

	cookies := req.Cookies()
	for _, cook := range cookies {
		if cook.Name == "id" && cook.Value != "" {
			info, err := getSessionInfo(cook.Value)
			if err == nil {
				serveAuthPage(info, w)
				return
			} else {
				fmt.Println("Error: " + err.Error())
			}
		}
	}

	serveStatic("static/index.html", w)
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

	http.HandleFunc("/static/", StaticServer)
	http.HandleFunc("/login", LoginHandler)
	http.HandleFunc("/", IndexHandler)
	fmt.Println("Hello world!")
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
