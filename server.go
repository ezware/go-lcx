package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

type appcfg struct {
	port      int
	cfgFile   string
	autoStart bool
	debug     bool
	logLevel  int
}

var (
	proxies   *ProxyList
	cfg       appcfg
	defaultIp string
	iplist    []string
)

const (
	PROMPT_USER     = "User: "
	PROMPT_PASSWORD = "Password: "
)

func init() {
	flag.IntVar(&cfg.port, "p", 8210, "HTTP server port")
	flag.StringVar(&cfg.cfgFile, "c", "proxy_config.json", "Proxy config json file")
	flag.BoolVar(&cfg.autoStart, "s", true, "Auto start proxy")
	flag.BoolVar(&cfg.debug, "d", false, "Show debug info")
	flag.IntVar(&cfg.logLevel, "l", 0, "Log level")
}

func signalProc() {
	var s os.Signal

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	s = <-c
	fmt.Println("Received signal", s)
	switch s {
	case os.Interrupt:
		proxies.save(cfg.cfgFile)
	}
	os.Exit(0)
}

func main() {
	flag.Parse()

	iplist = getIPList()
	defaultIp, _ = getDefaultIp()
	fmt.Println("========== IPLIST BEGIN ==========")
	fmt.Println(iplist)
	fmt.Println("========== IPLIST END   ==========")
	fmt.Println(defaultIp)

	proxies = &ProxyList{}
	proxies.pmap = make(map[int]*ProxyItem, 10)
	proxies.maxId = 0

	//excutable path
	d := filepath.Dir(os.Args[0])
	if !filepath.IsAbs(cfg.cfgFile) {
		cfg.cfgFile = d + string(filepath.Separator) + cfg.cfgFile
	} else {
		cfgdir := filepath.Dir(cfg.cfgFile)
		os.MkdirAll(cfgdir, 0764)
	}

	proxies.loadCfg(cfg.cfgFile, cfg.autoStart)

	go signalProc()

	fmt.Println("Starting go-lcx", proxies)

	fmt.Println("Current work dir:", d)
	http.Handle("/", http.FileServer(http.Dir(d)))
	http.HandleFunc("/lcx", lcxHandler)
	http.HandleFunc("/lcx/defaultip", defaultIpHandler)
	http.HandleFunc("/lcx/iplist", iplistHandler)
	http.HandleFunc("/lcx/proxylist", lcxProxyListHandler)
	http.HandleFunc("/lcx/proxy", lcxProxyHandler) //get & del
	http.HandleFunc("/lcx/proxy/add", lcxProxyAddHandler)
	http.HandleFunc("/lcx/proxy/modify", lcxProxyModifyHandler)
	http.HandleFunc("/lcx/proxy/op", lcxProxyOpHandler) //start/stop/del
	http.HandleFunc("/ws", websocketHandler)
	err := http.ListenAndServe(":8210", http.DefaultServeMux)
	if err != nil {
		log.Println("Failed to start go-lcx server, error:", err)
	}
	fmt.Println("Exiting")
}

func lcxHandler(resp http.ResponseWriter, req *http.Request) {
	op := req.FormValue("op")
	switch op {
	case "":
		lcxProxyListHandler(resp, req)
	case "save":
		proxies.save(cfg.cfgFile)
	case "defaultip":
		defaultIpHandler(resp, req)
	case "iplist":
		iplistHandler(resp, req)
	}
}

func defaultIpHandler(resp http.ResponseWriter, req *http.Request) {
	m := req.Method
	var ip []byte

	if m == http.MethodGet {
		ip = []byte(defaultIp)
	} else {
		ip = []byte("")
	}

	resp.Write([]byte(ip))
}

func iplistHandler(resp http.ResponseWriter, req *http.Request) {
	m := req.Method
	var iplistJson []byte
	var err error

	if m == http.MethodGet {
		iplistJson, err = json.Marshal(iplist)
		if err != nil {
			fmt.Println("Failed to marshal iplist json", err)
		}
	} else {
		iplistJson = []byte("{}")
	}

	resp.Write([]byte(iplistJson))
}

func lcxProxyListHandler(resp http.ResponseWriter, req *http.Request) {
	m := req.Method
	var allProxyJson []byte

	if m == http.MethodGet {
		allProxyJson = proxies.getAllProxy()
	} else {
		allProxyJson = []byte("{}")
	}

	resp.Write([]byte(allProxyJson))
}

type errRsp struct {
	Result int
	ErrMsg string
	Id     int
	Status int
}

func (e *errRsp) ToJson() []byte {
	j, _ := json.Marshal(e)
	return j
}

func lcxProxyHandler(resp http.ResponseWriter, req *http.Request) {
	id := req.FormValue("id")
	op := req.FormValue("op")

	var pi *ProxyItem
	pi, _ = proxies.get(id)
	if pi == nil {
		resp.Write([]byte("Proxy " + id + " not found"))
		return
	}

	fmt.Println("proxy handler: id", id, ", op", op)
	m := req.Method
	switch m {
	case "GET":
		switch op {
		case "start":
			fmt.Println("Before start:", pi)
			err := pi.start()
			if err != nil {
				log.Println(err)
				var rsp = errRsp{1, err.Error(), pi.Id, pi.Status}
				resp.Write(rsp.ToJson())
			} else {
				fmt.Println("After start1:", pi)
				var rsp = errRsp{0, "Success", pi.Id, pi.Status}
				resp.Write(rsp.ToJson())
			}
		case "stop":
			pi.stop()
			var rsp = errRsp{0, "Success", pi.Id, pi.Status}
			resp.Write(rsp.ToJson())
		default:
			resp.Write(pi.ToJson())
		}

	case "DEL":
		proxies.del(id)
		resp.Write([]byte("OK"))
	default:
		fmt.Println("Unknown method:", m)
	}
}

func getFormData(req *http.Request, includeId bool) (*ProxyItem, error) {
	var idn int
	var lportn int
	var rportn int
	var paramOk = true
	var ppi *ProxyItem
	var errstr string
	var err error
	var allerr error

	if includeId {
		id := req.FormValue("id")
		idn, err = strconv.Atoi(id)
		if err != nil {
			errstr += "Invalid id:" + id
			idn = 0
			paramOk = false
		} else {
			ppi, _ = proxies.getN(idn)
			if ppi == nil {
				errstr += "Proxy " + id + " not exist"
				paramOk = false
			}
		}
	}

	lip := req.FormValue("localip")
	if lip == "" {
		errstr += "Param localip missing"
		paramOk = false
	}

	lport := req.FormValue("localport")
	if lport == "" {
		errstr += "Param localip missing"
		paramOk = false
	}
	lportn, err = strconv.Atoi(lport)
	if err != nil {
		paramOk = false
		allerr = fmt.Errorf("failed to convert lport: %v", err)
	}

	rip := req.FormValue("remoteip")
	if rip == "" {
		errstr += "Param localip missing"
		paramOk = false
	}

	rport := req.FormValue("remoteport")
	if rport == "" {
		errstr += "Param localip missing"
		paramOk = false
	}
	rportn, err = strconv.Atoi(rport)
	if err != nil {
		paramOk = false
		allerr = fmt.Errorf("failed to convert lport: %v", allerr)
	}

	desc := req.FormValue("desc")

	if paramOk {
		ppi = &ProxyItem{
			Id:         idn,
			Desc:       desc,
			LocalIp:    lip,
			LocalPort:  lportn,
			RemoteIp:   rip,
			RemotePort: rportn,
		}
	} else {
		allerr = fmt.Errorf("%s %v", errstr, allerr)
	}

	return ppi, allerr
}

func getBodyData(req *http.Request, includeId bool) (*ProxyItem, error) {
	var pi = &ProxyItem{}
	var err error
	var body []byte

	defer req.Body.Close()
	body, err = io.ReadAll(req.Body)
	if err != nil {
		fmt.Println("Failed to read post body, ", err)
	} else {
		fmt.Println("Add request body: ", body)
		err = json.Unmarshal(body, &pi)
		if err != nil {
			fmt.Println("Failed to unmarshal request json, ", err)
		} else {
			err = pi.checkParam(includeId)
		}
	}

	return pi, err
}

func lcxProxyAddHandler(resp http.ResponseWriter, req *http.Request) {
	var err error
	var pi *ProxyItem

	fmt.Println("Add request: ", req)

	switch req.Method {
	case http.MethodGet:
		pi, err = getFormData(req, false)
	case http.MethodPost:
		pi, err = getBodyData(req, false)
	default:
		resp.Write([]byte("Method not support: " + req.Method))
		return
	}

	if err != nil {
		resp.Write([]byte("Failed to get proxy data from request:" + err.Error()))
		return
	}

	pid := proxies.add(pi)
	ppi, _ := proxies.getN(pid)
	j, err := json.Marshal(ppi)
	if err != nil {
		fmt.Println("Failed to marshal proxy ", ppi)
	} else {
		resp.Write(j)
	}
}

func lcxProxyModifyHandler(resp http.ResponseWriter, req *http.Request) {
	var pi *ProxyItem
	var err error

	fmt.Println("Modify request: ", req.Form)

	switch req.Method {
	case http.MethodGet:
		pi, err = getFormData(req, true)
	case http.MethodPost:
		pi, err = getBodyData(req, true)
	default:
		resp.Write([]byte("Method not support: " + req.Method))
		return
	}

	if err != nil {
		resp.Write([]byte("Failed to get proxy data from request:" + err.Error()))
	} else {
		proxies.modify(pi)
	}
}

func lcxProxyOpHandler(resp http.ResponseWriter, req *http.Request) {
	var paramOk = true

	resp.Write([]byte("lcx modify handled"))

	id := req.FormValue("id")
	if id == "" {
		resp.Write([]byte("Param id missing"))
		paramOk = false
	}

	op := req.FormValue("op")

	pi, _ := proxies.get(id)
	if pi == nil {
		resp.Write([]byte("Proxy " + id + " not exist"))
		return
	}

	if paramOk {
		switch op {
		case "start":
			pi.start()
		case "stop":
			pi.stop()
		case "del":
			proxies.del(id)
		default:
			resp.Write([]byte("Unknown operation:" + op))
		}
	}
}

func websocketHandler(resp http.ResponseWriter, req *http.Request) {
	var op = req.FormValue("op")
	var id = req.FormValue("id")
	if cfg.debug {
		fmt.Println("Websocket connect params: ProxyId:", id, ", op:", op)
	}

	switch op {
	case "termconnect":
		doTermConnect(resp, req, id)
	case "wscomm":
		doWsComm(resp, req)
	default:
		resp.Write([]byte("Unknown operation:" + op))
	}
}

func getIPList() []string {
	// get all interfaces
	iplist := []string{}
	interfaces, err := net.Interfaces()
	if err != nil {
		fmt.Println("Error:", err)
		return iplist
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			fmt.Println("Error:", err)
			continue
		}

		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPNet:
				// IPv4 or IPv6
				//fmt.Println(v.IP)
				iplist = append(iplist, v.IP.String())
			case *net.IPAddr:
				// Normally IPv4
				//fmt.Println(v.IP)
				iplist = append(iplist, v.IP.String())
			}
		}
	}

	fmt.Println("After get ip list")
	return iplist
}

func getDefaultIp() (ip string, err error) {
	conn, err := net.Dial("udp", "8.8.8.8:53")
	if err != nil {
		fmt.Println(err)
		return "", err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	ip = localAddr.String()
	ip = strings.Split(ip, ":")[0]
	return ip, nil
}
