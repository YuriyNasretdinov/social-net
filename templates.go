package main

import (
	"html/template"
)

var (
	authTpl = template.Must(template.ParseFiles("static/auth.html"))
)
