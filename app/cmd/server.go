package cmd

import (
)


type Server struct {
	Url	string `long:"url" env:"REMARK_URL" required:"true" description:"url to remark"`
}

type serverApp struct {
}
