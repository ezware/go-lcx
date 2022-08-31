package main

import (
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

type wsio struct {
	conn     net.Conn
	isStdErr bool
}

func (ws *wsio) Read(p []byte) (n int, err error) {
	r, err := wsutil.ReadClientText(ws.conn)
	if err != nil {
		fmt.Println("Failed to read data from ws", err)
		return 0, err
	}

	var copylen int
	if cap(p) < len(r) {
		copylen = cap(p)
		if cfg.debug {
			fmt.Println("stdin buffer is small than ws readed data")
		}
	} else {
		copylen = len(r)
	}

	copy(p, r[:copylen])
	if cfg.debug {
		fmt.Println("Readed", copylen, "bytes from ws:", p)
	}
	return copylen, err
}

func (ws *wsio) Write(p []byte) (n int, err error) {
	err = wsutil.WriteServerText(ws.conn, p)
	if err != nil {
		fmt.Println("Failed to write data", p, "to ws", err, "is stderr", ws.isStdErr)
		return 0, err
	}

	if cfg.debug {
		fmt.Println("Writed", p, "to", func() string {
			if ws.isStdErr {
				return "stderr"
			} else {
				return "stdout"
			}
		}())
	}
	return len(p), err
}

func getWsEcho(conn io.ReadWriter, write string) (string, error) {
	wmsg := []byte(write)
	var op = ws.OpText
	var rcvstr string = ""

	err := wsutil.WriteServerMessage(conn, op, wmsg)
	if err != nil {
		// handle error
		fmt.Println("Failed to write ws data", write, err)
		return "", err
	}

	var echo = true
	var got_line_feed = false

	switch write {
	case PROMPT_PASSWORD:
		echo = false
	}

	for {
		rmsg, op, err := wsutil.ReadClientData(conn)
		if err != nil {
			// handle error
			fmt.Println("Failed to read ws data", err)
			return "", err
		} else {
			if cfg.debug {
				fmt.Println("Received ws data, op", op, "msg", rmsg)
			}
			if rmsg[0] == '\r' || rmsg[0] == '\n' {
				got_line_feed = true
			} else {
				rcvstr += string(rmsg)
			}
			if echo {
				err = wsutil.WriteServerMessage(conn, op, rmsg)
				if err != nil {
					fmt.Println("Failed to send echo data", rmsg)
				}
			}

			if got_line_feed {
				break
			}
		}
	}

	return rcvstr, nil
}

func doWsComm(resp http.ResponseWriter, req *http.Request) {
	conn, _, _, err := ws.UpgradeHTTP(req, resp)
	if err != nil {
		// handle error
		fmt.Println("Failed to upgrade to websocket,", err, req)
		return
	}
	go func() {
		defer conn.Close()
		for {
			msg, op, err := wsutil.ReadClientData(conn)
			if err != nil {
				// handle error
				fmt.Println("Failed to read ws data", err)
				break
			} else {
				if cfg.debug {
					fmt.Println("Received ws data, op", op, "msg", msg)
				}
			}

			err = wsutil.WriteServerMessage(conn, op, msg)
			if err != nil {
				// handle error
				fmt.Println("Failed to write ws data", err)
				break
			}
		}
	}()
}
