
    /*
    localip
    localport
    remoteip
    remoteport
    status:  0 disabled, 1 stopped, 2 started, 3 connected
    desc
    */
    function convertProxyItemFromServer(serverObj) {
        var localObj = {
            id: serverObj.Id,
            desc: serverObj.Desc,
            localip: serverObj.LocalIp,
            localport: serverObj.LocalPort,
            remoteip: serverObj.RemoteIp,
            remoteport: serverObj.RemotePort,
            status: serverObj.Status,
            instances: serverObj.Instances
        }
        return localObj
    }

    var lastSelectedTr = undefined
    Vue.prototype.$http = axios
    var vapp = new Vue({
        el: "#vApp",
        data: {
            statusName: [ "stopped", "started", "connected" ],
            startBtnVals: [ "el-icon-video-play", "el-icon-video-pause" ],
            proxylist: [
                /* example
                {
                    id: 2,
                    desc: "B4020",
                    localip: "10.213.216.44",
                    localport: 32202,
                    remoteip: "192.168.10.5",
                    remoteport: 22,
                    status: 3
                }
                */
            ],
            proxyListColumns: [
                {
                    field: "id",
                    label: "编号",
                    sortable: true,
		    width: 80
                },
                {
                    field: 'desc',
                    label: '设备描述'
                },
                {
                    field: 'localip',
                    label: '本地IP地址',
			width: 180
                },
                {
                    field: 'localport',
                    label: '本地端口',
                },
                {
                    field: 'remoteip',
                    label: '远端IP地址',
			width: 180,
                    centered: false
                },
                {
                    field: 'remoteport',
                    label: '远端端口',
                },
                {
                    field: 'showStatus',
                    label: '状态',
                },
                {
                    field: 'instances',
                    label: '实例数'
                },
                {
                    field: 'op',
                    label: '操作'
                }
            ],
            isEditModalActive: false,
            editFieldConfig:[
                {
                    label: "测试标签",
                    model: 0
                },
                {
                    label: "测试标签2",
                    model: 1
                }
            ],
            editDesc: "",
            editLocalIp: "",
            editLocalPort: 0,
            editRemoteIp: "",
            editRemotePort: 0,
            editTitle: "",
            editMode: 0,
            editId: 0,  //proxy id
            editProxyArrayIndex: undefined  //proxy list index
        },
        computed: {
            proxyList: function() {
                var showlist=[]
                for (i=0; i < this.proxylist.length; i++) {
                    var pitem = this.proxylist[i]
                    pitem.showStatus = this.statusName[pitem.status]
                    showlist.push(pitem)
                }
                return showlist
            }
        },
        created: function() {
            this.$http.get("/lcx/proxylist").then(
                function(res){
                    var newobj = {}
                    console.log("get proxylist:" + res.status + ", resdata:" + res.data)
                    for (var i = 0; i < res.data.length; i++) {
                        vapp.proxylist.push(convertProxyItemFromServer(res.data[i]))
                    }
                    vapp.proxylist.sort(function(a, b){return a.id - b.id})
                },function(res){
                    console.log(res.status);
                });
        },
        methods: {
            testClick: function(e) {
                console.log("Test clicked")
            },
            trClicked: function(e) {
                console.log("tr clicked")
                me = e.currentTarget
                me.style.backgroundColor = "cadetblue"
                //e.currentTarget.class = "trselected"
                if (lastSelectedTr) {
                    //lastSelectedTr.class = "trnormal"
                    if (lastSelectedTr != me) {
                        lastSelectedTr.style.backgroundColor = "white"
                    }
                }

                lastSelectedTr = e.currentTarget

            },
            startBtnClicked: function(e, p) {
                console.log(p.id + "start clicked" + e)
                me = e.currentTarget.firstElementChild
                if (me.className == this.startBtnVals[0]) {
                    //do start
                    this.startProxy(p.id)

                    //don't modify there, it's async
                    //me.className = this.startBtnVals[1]
                } else {
                    //do stop
                    this.stopProxy(p.id)
                    //me.className = this.startBtnVals[0]
                }
            },
            delBtnClicked: function(e, p) {
                console.log("Delete proxy " + p.id)
            },
            termBtnClicked: function(e, p) {
                console.log("Openning term " + p.id)
                window.open("term.html?localip=" + p.localip + "&localport=" + p.localport)
            },
            cellClicked: function(row, col, rowIndex, colIndex) {
                console.log("Cell clicked, row " + rowIndex + ", col " + colIndex + ", field: " + col.field)
            },
            getArrayIndexByProxyId: function(pid) {
                //var haveIndex = 0
                for (i = 0; i < this.proxylist.length; i++) {
                    if (this.proxylist[i].id == pid) {
                        //haveIndex = 1
                        console.log("found proxy index" + i)
                        return i
                    }
                }
                return undefined
            },
            tblDblClicked: function(row, col, evt) {
                console.log("Double clicked, row:", row.desc)
                this.editId = row.id
                this.editDesc = row.desc
                this.editLocalIp = row.localip
                this.editLocalPort = row.localport
                this.editRemoteIp = row.remoteip
                this.editRemotePort = row.remoteport
                this.editTitle = '修改代理信息'
                this.editMode = 1
                var haveIndex = undefined
                haveIndex = this.getArrayIndexByProxyId(row.id)
                this.editProxyArrayIndex = haveIndex

                this.isEditModalActive = true
            },
            addProxyClicked: function() {
                console.log("Add proxy," + this)
                this.editId = 0
                this.editDesc = ""
                this.editLocalIp = ""
                this.editLocalPort = 30000
                this.editRemoteIp = ""
                this.editRemotePort = 22
                this.editTitle = '新增代理信息'
                this.editMode = 0

                this.isEditModalActive = true;
            },
            addLocalProxyItem: function() {
                console.log("Add local proxy item")
            },
            updateLocalProxyItem: function(proxyItem) {
                proxyItem.desc = this.editDesc
                proxyItem.localip = this.editLocalIp
                proxyItem.localport = this.editLocalPort
                proxyItem.remoteip = this.editRemoteIp
                proxyItem.remoteport = this.editRemotePort
                //newProxy.status = 0
            },
            getProxyVar: function() {
                var newProxy = {}
                newProxy.Id = this.editId
                newProxy.Desc = this.editDesc
                newProxy.LocalIp = this.editLocalIp
                newProxy.LocalPort = parseInt(this.editLocalPort)
                newProxy.RemoteIp = this.editRemoteIp
                newProxy.RemotePort = parseInt(this.editRemotePort)
                newProxy.Status = 0
                return newProxy
            },
            addProxy: function(p) {
                this.$http.post("/lcx/proxy/add", p).then(
                function(res){
                    console.log("post res:" + res.status + ", resbody:" + res.data)
                    vapp.proxylist.push(convertProxyItemFromServer(res.data))
                },function(res){
                    console.log(res.status);
                });
            },
            updateProxy: function(p) {
                this.$http.post("/lcx/proxy/modify", p).then(
                function(res){
                    console.log("post res:" + res.status + ", resbody:" + res.data)
                },function(res){
                    console.log(res.status);
                });
            },
            saveProxy: function() {
                switch (this.editMode) {
                    case 0:
                        console.log("Add proxy")
                        this.addProxy(this.getProxyVar())
                        break;
                    case 1:
                        console.log("Update proxy")
                        this.updateProxy(this.getProxyVar())
                        if (this.editProxyArrayIndex != undefined) {
                            this.updateLocalProxyItem(this.proxylist[this.editProxyArrayIndex])
                        } else {
                            console.log("Failed to get proxy array index")
                        }
                        break;
                    default:
                        console.log("Edit mode " + this.editMode + " is unknown")
                }

                this.isEditModalActive = false
            },
            startProxy: function(pid) {
                this.$http.get("/lcx/proxy?id=" + pid + "&op=start").then(function(res){
                    console.log("start proxy:" + res.status + ", resdata:" + res.data)
                    if (res.data.Result != 0) {
                        vapp.$message.error(res.data.Id + '启动失败：' + res.data.ErrMsg);
                    } else {
                        //modify status here
                        //setStatus
                        id = vapp.getArrayIndexByProxyId(res.data.Id)
                        if (id < vapp.proxylist.length) {
                            vapp.proxylist[id].status = res.data.Status
                        }
                    }
                },function(res){
                    console.log(res.status);
                })
            },
            stopProxy: function(pid) {
                this.$http.get("/lcx/proxy?id=" + pid + "&op=stop").then(function(res){
                    if (res.data.Result != 0) {
                        vapp.$message.error(res.data.Id + '停止失败：' + res.data.ErrMsg);
                    } else {
                        //modify status here
                        //setStatus
                        id = vapp.getArrayIndexByProxyId(res.data.Id)
                        if (id < vapp.proxylist.length) {
                            vapp.proxylist[id].status = res.data.Status
                        }
                    }
                },function(res){
                    console.log(res.status);
                })
            },
            cancelEdit: function() {
                console.log("Cancel edit")
                this.isEditModalActive = false
            }
        }
    });

    if (1) {
    /* hide loading */
    oLoading = document.getElementById("loading");
    oLoading.style.display = "none";

    /* show the content */
    oApp = document.getElementById("vApp");
    oApp.style.display = "block";
    }
