package socket

import "github.com/ygrebnov/errorc"

var (
	ErrCannotListen           = errorc.New("cannot listen")
	ErrCannotAccept           = errorc.New("cannot accept connection")
	ErrCannotConnect          = errorc.New("cannot connect")
	ErrCannotSetReadDeadline  = errorc.New("cannot set read deadline")
	ErrCannotSetWriteDeadline = errorc.New("cannot set write deadline")
	ErrCannotReceiveMessage   = errorc.New("cannot receive message")
	ErrCannotSendMessage      = errorc.New("cannot send message")
)
