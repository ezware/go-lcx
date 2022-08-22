var t = new Terminal({cursorBlink: true, disableStdin: false, logLevel: 1});
var writeData = ""

t.onData(function(data) {
    t.write(data)
    if (ws) {
        /*
        if (send == "\n") {
            send += "\r"
        }
        if (send == "\r") {
            send += "\n"
        }
        */
        ws.send(data)
    }
    /*
    if (data == "\r" || data == "\n") {
        //t.write(writeData)

        writeData = ""
    } else {
        writeData += data
    }
    */
})
//t.onKey((k,e) => console.log(k, e))
//var attachAddon = AttachAddon(ws);
//t.loadAddon(attachAddon);
oTerm = document.getElementById("term")
if (oTerm) {
    t.open(oTerm)
} else {
    console.log("Failed to get term object")
}

var wsurl = "ws://" + window.location.host + '/ws'

function getParams() {
    let searchStr = window.location.search.substring(1)
    if (searchStr) {
        let aParam = searchStr.split("&")
        let p = ""
        let pa = []
        let pn = ""
        let pv = ""
        let params = {}
        let paramStr = "params={"
        let j = 0
        for (var i = 0; i < aParam.length; i++) {
            p = aParam[i]
            pa = p.split("=")
            if (pa.length > 1) {                
                pn = pa[0]
                pv = pa[1]
                if (j > 0) {
                    paramStr +=","
                }
                paramStr += pn + ": \"" + pv + "\""
                console.log("Param" + i + ": " + pn + " = " + pv)
                j++
            } else {
                console.log("Failed to parse param" + i)
            }
        }
        paramStr += "}"
        eval(paramStr)
        console.log("parmas:", params)

        return params
    } else {
        return {}
    }
}

function getParams2() {
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
params = getParams2()
console.log("ws url: " + wsurl + ", params:" + params)
var ws = new WebSocket(wsurl + '?op=termconnect&id=' + params.id);
//var ws = new WebSocket(wsurl + '?op=wscomm&id=' + params.id);
var state = 0
ws.onopen = function(evt) {
  console.log('Connection open ...');
  //ws.send('{Id:' + params.id + '}');
};

function ctrlHandler(data) {
    console.log("Received control packet " + data)
}

function dataHandler(data) {
    console.log("Received data packet " + data)
}

ws.onmessage = function(evt) {
  console.log('Received Message: ' + evt.data);  
  msg = evt.data
  if (msg.Type == "c") {
    ctrlHandler(msg.Data)
  } else if (msg.Type == "d") {
    dataHandler(msg.Data)
  } else {
    //unknown type, write out
    t.write(evt.data)
  }
  /*
    switch (state) {
        case 0:
            if (t) {
                //t.write("Connecting to proxy " + evt.data)
                t.write(evt.data)
            }
            break
        case 1:
            if (t) {
                t.write(evt.data)
            }
            break
        default:
    }
    */
};

ws.onclose = function(evt) {
  console.log('Connection closed.');
};