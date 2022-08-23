package main

import (
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"golang.org/x/crypto/ssh"
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

func doTermConnect(resp http.ResponseWriter, r *http.Request, pid string) {
	p, _ := proxies.get(pid)
	if p == nil {
		resp.Write([]byte("Proxy " + pid + " not exist"))
		return
	}

	//upgrade to ws
	conn, _, _, err := ws.UpgradeHTTP(r, resp)
	if err != nil {
		// handle error
		fmt.Println("Failed to upgrade to websocket,", err, r)
		return
	}

	//get user
	user, err := getWsEcho(conn, PROMPT_USER)
	if err != nil {
		conn.Close()
		return
	}

	//get password
	pass, err := getWsEcho(conn, PROMPT_PASSWORD)
	if err != nil {
		conn.Close()
		return
	}

	var sshcfg = &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(pass),
		},
		HostKeyCallback: ssh.HostKeyCallback(func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			if cfg.debug {
				fmt.Println("Host key received, hostname", hostname, ",addr:", remote.String(), ",key:", key)
			}
			return nil
		}),
	}

	client, err := ssh.Dial(p.Type, p.getLocalAddr(), sshcfg)
	if err != nil {
		fmt.Println("Failed to connect ssh", p, err)
		conn.Close()
		return
	}

	//client.SendRequest()
	ss, err := client.NewSession()
	if err != nil {
		fmt.Println("Failed to create session", err)
		conn.Close()
		return
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	rows := 40
	cols := 120
	err = ss.RequestPty("xterm", rows, cols, modes)
	if err != nil {
		fmt.Println("Failed to request pty", err)
	}

	writeIn, err := ss.StdinPipe()
	if err != nil {
		fmt.Println("Failed to get stdin pipe")
	}
	readOut, err := ss.StdoutPipe()
	if err != nil {
		fmt.Println("Failed to get stdout pipe")
	}
	readErr, err := ss.StderrPipe()
	if err != nil {
		fmt.Println("Failed to get stderr pipe")
	}

	var wsstderr = &wsio{conn, true}
	var wsstdother = &wsio{conn, false}

	go func() {
		var inBuf = make([]byte, 128)
		for {
			nr, err := wsstdother.Read(inBuf)
			if err != nil {
				fmt.Println("Failed to read data from ws", err, "exiting read thread")
				return
			}
			if cfg.debug {
				fmt.Println("Readed", nr, "bytes from ws", inBuf[:nr])
			}

			nw, err := writeIn.Write(inBuf[:nr])
			if err != nil {
				fmt.Println("Failed to write data to ssh stdin", err)
				return
			}

			if cfg.debug {
				fmt.Println("Writed", nw, "bytes to ssh stdin")
			}
		}
	}()
	go io.Copy(wsstdother, readOut)
	go io.Copy(wsstderr, readErr)

	go func() {
		defer ss.Close()
		defer client.Close()
		defer conn.Close()
		err := ss.Shell()
		if err != nil {
			fmt.Println("Failed to start session", err)
		}

		//nothing to do
		err = ss.Wait()
		if err != nil {
			fmt.Println("Failed to wait session", err)
		} else {
			fmt.Println("Session wait: Shell exited")
		}
	}()
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
