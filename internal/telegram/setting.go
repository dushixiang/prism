package telegram

import (
	"net/http"
)

type Settings struct {
	Token  string
	Client *http.Client
}
