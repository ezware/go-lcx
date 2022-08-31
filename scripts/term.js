//state constants
const S_WAIT_CONNECT = 0
const S_SEND_USERNAME = 1
const S_SEND_PASSWORD = 2
const S_SHELL_IO = 3

var g_State = S_WAIT_CONNECT
var g_Term = undefined
var g_WS = undefined
var g_WSURL = "ws://" + window.location.host + '/ws'
var g_Params = undefined

function createTerm() {
    g_Term = new Terminal({cursorBlink: true, disableStdin: false, logLevel: 1, fontFamily: "consolas", fontSize: 16});
    if (!g_Term) {
        console.log("Failed to create terminal")
        return
    }

    g_Term.resize(150, 32)

    g_Term.onData(function(data) {
        let ECHO_ON = false
        switch (g_State) {
            case S_SEND_USERNAME:
                if (data == '\r' || data == '\n') {
                    g_Term.write('\n')
                    g_State = S_SEND_PASSWORD
                }
                break
            case S_SEND_PASSWORD:
                if (data == '\r' || data == '\n') {
                    g_Term.write('\r\n')
                    g_State = S_SHELL_IO
                }
                break
            case S_SHELL_IO:
                break
            case S_WAIT_CONNECT:
                if (data == '\r') {
                    console.log("Reconnecting websocket")
                    createWebSocket(g_Params)
                    return
                }
                break
            default:
                break
        }
        if (ECHO_ON) {
            g_Term.write(data)
        }
        if (g_WS) {
            g_WS.send(data)
        }
    })

    let oTerm = document.getElementById("term")
    if (oTerm) {
        g_Term.open(oTerm)
    } else {
        console.log("Failed to get term object")
    }
}

function getParams() {
    let  oGetVars = {};

    buildValue = function(sValue) {
        if (/^\s*$/.test(sValue)) { return null; }
        if (/^(true|false)$/i.test(sValue)) { return sValue.toLowerCase() === "true"; }
        if (isFinite(sValue)) { return parseFloat(sValue); }
        if (isFinite(Date.parse(sValue))) { return new Date(sValue); } // this conditional is unreliable in non-SpiderMonkey browsers
        return sValue;
    }

    if (window.location.search.length > 1) {
        for (let aItKey, nKeyId = 0, aCouples = window.location.search.substring(1).split("&"); nKeyId < aCouples.length; nKeyId++) {
            aItKey = aCouples[nKeyId].split("=");
            oGetVars[decodeURI(aItKey[0])] = aItKey.length > 1 ? buildValue(decodeURI(aItKey[1])) : null;
        }
    }

    return oGetVars
}

function ctrlHandler(data) {
    console.log("Received control packet " + data)
}

function dataHandler(data) {
    console.log("Received data packet " + data)
}

function createWebSocket(params) {
    if (g_State == S_WAIT_CONNECT) {
        g_WS = new WebSocket(g_WSURL + '?op=termconnect&id=' + params.id);
        if (!g_WS) {
            console.log("Failed to create websocket")
            return
        }

        g_WS.onopen = function(evt) {
            console.log('Connection open ...');
            g_State = S_SEND_USERNAME
        };

        g_WS.onmessage = function(evt) {
            console.log('Received Message: ' + evt.data);
            msg = evt.data
            if (msg.Type == "c") {
                ctrlHandler(msg.Data)
            } else if (msg.Type == "d") {
                dataHandler(msg.Data)
            } else {
                //unknown type, write out
                g_Term.write(evt.data)
            }
        };

        g_WS.onclose = function(evt) {
            console.log('Connection closed.');
            g_State = S_WAIT_CONNECT
            if (g_Term) {
                g_Term.write("\r\nConnection lost, press ENTER to reconnect\r\n")
            }
        };
    }
}

function setTitle(params) {
    let oTitle = document.getElementById("title")
    if (oTitle) {
        oTitle.innerText = "LocalAddr: " + params.localip + ":" + params.localport
                + ", RemoteAddr: " + params.remoteip + ":" + params.remoteport
                + ", TermType: " + params.termtype
    }
}

function resizeBtnClicked() {
    let oRows = document.getElementById("rows")
    let oCols = document.getElementById("cols")

    let rows = oRows.value
    let cols = oCols.value

    if (isNaN(rows) || rows < 5) {
        rows = 5
    }
    if (isNaN(cols) || cols < 5) {
        cols = 5
    }

    if (g_Term) {
        g_Term.resize(cols, rows)
    }
}

g_Params = getParams()
console.log("ws url: " + g_WSURL + ", params:" + g_Params)
setTitle(g_Params)
createTerm()
createWebSocket(g_Params)
