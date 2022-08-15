package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"time"
)

const (
	STATUS_STOPPED int = iota
	STATUS_STARTED
	//STATUS_CONNECTED, use instances to indicate the connect status
)

type ProxyItem struct {
	Id         int
	Status     int
	LocalIp    string
	LocalPort  int
	RemoteIp   string
	RemotePort int
	Desc       string
	Type       string //tcp, udp, unix

	//runtime attributes
	Instances int
	stopCh    chan int
	mgr       *ProxyList
}

func (pi *ProxyItem) getLocalAddr() string {
	return pi.LocalIp + ":" + strconv.Itoa(pi.LocalPort)
}

func (pi *ProxyItem) getRemoteAddr() string {
	return pi.RemoteIp + ":" + strconv.Itoa(pi.RemotePort)
}

func transData(readConn net.Conn, writeConn net.Conn, stopch chan error, tips string) {
	var buf = make([]byte, 4096)
	var debug = false

	for {
		nbytes, err := readConn.Read(buf)
		if err != nil {
			fmt.Println(tips, "read error:", err)
			stopch <- err
			return
		} else {
			if debug {
				fmt.Println(tips, "Read got", nbytes, "bytes")
			}
			nbytesWrite, err := writeConn.Write(buf[:nbytes])
			if err != nil {
				fmt.Println(tips, "write error:", err)
				stopch <- err
				return
			} else {
				if debug {
					fmt.Println(tips, "Writed", nbytesWrite, "bytes data")
				}
			}
		}
	}
}

func serverConn(pi *ProxyItem, local net.Conn, remote net.Conn) {
	defer local.Close()
	defer remote.Close()
	var err error
	l2rCh := make(chan error)
	r2lCh := make(chan error)

	go transData(local, remote, l2rCh, "local2remote")
	time.After(time.Microsecond)
	go transData(remote, local, r2lCh, "remote2local")

	select {
	case err = <-l2rCh:
		fmt.Println("local2remote thread exited:", err)
		break
	case err = <-r2lCh:
		fmt.Println("remote2local thread exited:", err)
		break
	}

	log.Printf("%s to %s proxy instance exited", local.LocalAddr().String(), remote.RemoteAddr().String())
	if pi.Instances > 0 {
		pi.Instances--
		proxies.modify(pi)
	}
}

type opResult struct {
	pi    *ProxyItem
	reqts string //request timestamp
	err   error
}

func connRcvr(pi *ProxyItem, listener net.Listener, reqTimestamp string) {
	protocol := pi.Type
	remoteAddr := pi.getRemoteAddr()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Failed to accept:", err)
			break
		}

		log.Println("Received incoming connection from", conn.RemoteAddr().String())
		remoteConn, err := net.Dial(protocol, remoteAddr)
		if err != nil {
			fmt.Println("Failed to dial", protocol, remoteAddr, err)
			continue
		}
		log.Println("Established new connection to", remoteConn.RemoteAddr().String())
		pi.Instances++
		pi.mgr.modify(pi)
		go serverConn(pi, conn, remoteConn)
	}
}

func newProxyServer(pi *ProxyItem, startResultCh chan opResult, reqTimestamp string) {
	protocol := pi.Type
	addr := pi.getLocalAddr()
	var r = opResult{pi, reqTimestamp, nil}

	//startResultCh
	//stopCh
	listener, err := net.Listen(protocol, addr)
	if err != nil {
		r.err = fmt.Errorf("failed to listen on %s %s: %v", protocol, addr, err)
		startResultCh <- r
		close(startResultCh)
		return
	}
	defer listener.Close()

	//create stop ch
	stopCh := make(chan int)
	pi.stopCh = stopCh
	fmt.Println("Created stopCh:", pi)

	go connRcvr(pi, listener, reqTimestamp)

	log.Println("Started proxy:", pi)

	startResultCh <- r
	close(startResultCh)

	//wait for stop signal
	stop := <-stopCh
	fmt.Println("Received stop proxy signal:", stop)

	close(stopCh)
	pi.stopCh = nil

	log.Println("Stopped proxy:", pi)
}

func (pi *ProxyItem) start() error {
	fmt.Println("Starting proxy")
	if pi.Status > STATUS_STOPPED {
		fmt.Println("Proxy already started:", pi)
		return nil
	}

	ch := make(chan opResult)
	ts := fmt.Sprintf("%d", time.Now().Unix())
	go newProxyServer(pi, ch, ts)

	result := <-ch
	if result.err == nil {
		pi.Status = STATUS_STARTED
	}
	return result.err
}

func (pi *ProxyItem) stop() {
	fmt.Println("Stopping proxy")
	if pi.Status == STATUS_STOPPED {
		return
	}

	pi.stopCh <- pi.Id
	time.After(time.Microsecond)
	pi.Status = STATUS_STOPPED
}

func (pi *ProxyItem) restart() error {
	fmt.Println("Restarting proxy")
	pi.stop()
	err := pi.start()
	//pi.mgr.modify(pi)
	return err
}

func (pi *ProxyItem) addDefaults() {
	if pi.Type == "" {
		pi.Type = "tcp"
	}
}

func (pi *ProxyItem) checkParam(includeId bool) error {
	var errstr string = ""
	var ok = true

	if includeId {
		if pi.Id == 0 {
			errstr += "Invalid Id\n"
			ok = false
		}
	}

	if pi.LocalIp == "" {
		errstr += "No local ip\n"
		ok = false
	}

	if pi.LocalPort == 0 {
		errstr += "Invalid local port\n"
	}

	if pi.RemoteIp == "" {
		errstr += "No remote ip\n"
		ok = false
	}

	if pi.RemotePort == 0 {
		errstr += "Invalid remote port"
	}

	if ok {
		return nil
	} else {
		return fmt.Errorf("%s", errstr)
	}
}

func (p *ProxyItem) ToJson() []byte {
	jb, err := json.Marshal(p)
	if err != nil {
		jb = []byte("failed to get proxy item")
	}

	return jb
}

type ProxyList struct {
	pmap  map[int]ProxyItem
	maxId int
}

func (pl *ProxyList) allocId() int {
	pl.maxId++
	return pl.maxId
}

func (pl *ProxyList) get(id string) (pi *ProxyItem, idx int) {
	idn, err := strconv.Atoi(id)
	if err != nil {
		fmt.Println("Invalid id: ", id)
		return nil, 0
	}

	return pl.getN(idn)
}

func (pl *ProxyList) getN(id int) (pi *ProxyItem, idx int) {
	fmt.Println("Getting proxy ", id)

	p, ok := pl.pmap[id]
	if ok {
		pi = &p
		return pi, pi.Id
	}

	return nil, 0
}

func (pl *ProxyList) add(p ProxyItem) int {
	p.Id = pl.allocId()
	p.addDefaults()
	p.mgr = pl
	pl.pmap[p.Id] = p
	fmt.Println("Added new porxy ", p)
	return p.Id
}

func (pl *ProxyList) del(id string) error {
	fmt.Println("Deleting proxy" + id)
	pi, idx := pl.get(id)
	if pi != nil {
		delete(pl.pmap, idx)
	}

	return nil
}

func needRestart(p1 *ProxyItem, p2 *ProxyItem) bool {
	if p1.LocalIp != p2.LocalIp {
		return true
	}

	if p1.LocalPort != p2.LocalPort {
		return true
	}

	if p1.RemoteIp != p2.RemoteIp {
		return true
	}

	if p1.RemotePort != p2.RemotePort {
		return true
	}

	if p1.Type != p2.Type {
		return true
	}

	return false
}

func (pl *ProxyList) modify(p *ProxyItem) {
	fmt.Println("Modifing proxy")

	pi, ok := pl.pmap[p.Id]
	if ok {
		/* test if changed */
		/*
			pi.Desc = p.Desc
			pi.LocalIp = p.LocalIp
			pi.LocalPort = p.LocalPort
			pi.RemoteIp = p.RemoteIp
			pi.RemotePort = p.RemotePort
		*/
		if pi != *p {
			p.addDefaults()
			p.mgr = pl
			restart := needRestart(&pi, p)
			fmt.Println("Before modify: ", pi)
			pl.pmap[p.Id] = *p
			fmt.Println("After modify: ", p)

			/* restart proxy use new param if changed */
			if restart {
				err := p.restart()
				if err != nil {
					fmt.Println("Failed to restart proxy:", pi)
				} else {
					pl.pmap[p.Id] = *p
					fmt.Println("After modify2: ", pl.pmap[p.Id])
				}
			}
		} else {
			fmt.Println("ProxyItem not changed", pi)
		}
	} else {
		fmt.Println("proxy", p.Id, "not exist")
	}
}

// get all proxy in json format
func (pl *ProxyList) getAllProxy() []byte {
	var buf bytes.Buffer
	var first = true

	buf.Write([]byte("["))
	for _, v := range pl.pmap {
		j, e := json.Marshal(v)
		if e != nil {
			fmt.Println("Failed to marshal proxy:", v)
			continue
		}
		if !first {
			buf.Write([]byte(","))
		} else {
			first = false
		}
		buf.Write(j)
	}
	buf.Write([]byte("]"))

	return buf.Bytes()
}

func (pl *ProxyList) save(fileName string) {
	if len(pl.pmap) < 1 {
		fmt.Println("No config to save")
		return
	}

	f, err := os.OpenFile(fileName, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Failed to open file", fileName, "for write:", err)
		return
	}

	defer f.Close()
	f.Write(pl.getAllProxy())
	fmt.Println("Config ", fileName, " saved")
}

func (pl *ProxyList) loadCfg(fileName string, autostart bool) {
	buf, err := os.ReadFile(fileName)
	if err != nil {
		if err != os.ErrNotExist {
			fmt.Println("Failed to open file", fileName, "for read:", err)
		}
		return
	}

	var items []ProxyItem

	err = json.Unmarshal(buf, &items)
	if err != nil {
		fmt.Println(err)
	}

	var maxid int
	for _, p := range items {
		if p.Id > maxid {
			maxid = p.Id
		}
		p.Status = 0
		p.Instances = 0
		p.addDefaults()
		p.mgr = pl
		pl.pmap[p.Id] = p

		if autostart {
			err = p.start()
			if err != nil {
				log.Println("Failed to start proxy", p, ", err:", err)
			} else {
				//update proxy status and stopCh
				pl.pmap[p.Id] = p
			}
		}
	}

	pl.maxId = maxid
}
