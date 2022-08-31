package main

import (
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/gobwas/ws"
	"github.com/ziutek/telnet"
	"golang.org/x/crypto/ssh"
)

func doTelnetComm(wsConn net.Conn, p *ProxyItem) {
	tcon, err := telnet.Dial(p.Type, p.getLocalAddr())
	if err != nil {
		fmt.Println("Failed to connect telnet", err)
		wsConn.Close()
		return
	}

	//var wsstderr = &wsio{wsConn, true}
	var wsstdother = &wsio{wsConn, false}

	go func() {
		defer wsConn.Close()
		defer tcon.Close()

		exitCh := make(chan int)

		//read
		go func() {
			for {
				//read from telnet, write to user
				_, err := io.Copy(wsstdother, tcon)
				if err != nil {
					fmt.Println("Failed to copy data from telnet to user")
					break
				}
			}

			exitCh <- 1
		}()
		//write
		go func() {
			for {
				//read from user, write to telnet
				_, err := io.Copy(tcon, wsstdother)
				if err != nil {
					fmt.Println("Failed to copy data from user to telnet")
					break
				}
			}

			exitCh <- 2
		}()

		<-exitCh

		fmt.Println("telnet session exited")
	}()
}

func doSshComm(conn net.Conn, p *ProxyItem) {
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

	if p.TermType == "telnet" {
		doTelnetComm(conn, p)
	} else {
		doSshComm(conn, p)
	}
}
