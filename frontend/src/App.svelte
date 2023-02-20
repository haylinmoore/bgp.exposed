<script>
    import {onMount} from "svelte";

    import Logo from "./components/Logo.svelte";
    import AnnouncementsTable from "./components/AnnouncementsTable.svelte";
    import ReceivedRoutesTable from "./components/ReceivedRoutesTable.svelte";
    import { time_ranges_to_array } from "svelte/internal";

    import './styles.scss'

    let announcements = [];
    let receivedRoutes = [];

    let socketConnected = false;
    let sessionCreated = false;
    let bgpState = "Unknown";
    let holdTimer = 0;
    let lastMessageTimer = 0;
    let keepaliveTimer = 0;
    let sentLastKeepAlive = 0;
    let lastUpdate = "Never";
    let lastKeepalive = "Never";

    let routesets = [];

    let endpoint = "http://" + window.location.hostname + ":8080/"
    if (window.location.host.includes("bgp.exposed")){
        endpoint = window.location.protocol+"//"+window.location.host+"/api/"
    }

    let socket;
    onMount(() => {
        socket = new WebSocket(endpoint.replace("http", "ws") + "ws"); // window.location.host + "/ws"

        socket.onopen = function (e) {
            console.log("ws connected");
            socketConnected = true;
        };

        socket.addEventListener("message", (e) => {
            let message = e.data;
            e = JSON.parse(e.data)
            if (e.type === "RouteData") {
                console.log("received ws message: " + message)
                if (e.data.prefixes != null){
                    for (const prefix of e.data.prefixes) {
                        receivedRoutes.push({
                            id: prefix.id,
                            prefix: prefix.prefix,
                            path: e.data.asPath,
                            nexthop: e.data.nextHop,
                            origin: e.data.origin,
                            communities: e.data.communities.map(
                                (element) => { return "[" + element.join(",") + "]" }
                            ),
                            largeCommunities: e.data.largeCommunities.map(
                                (element) => {
                                    return "[" + element.GlobalAdmin + "," + element.LocalData1 + "," + element.LocalData2 + "]"
                                }
                            ),
                            rpki: "invalid",
                            irr: false
                        });
                    }
                }
                if (e.data.withdraws != null){
                    for (const prefix of e.data.withdraws) {
                        receivedRoutes = receivedRoutes.filter(a => (a.prefix != prefix.prefix || a.id != prefix.id)); 
                    }
                }
                receivedRoutes = receivedRoutes; // Trigger svelte refresh
            } if (e.type=="FSMUpdate") {
                if (e.data.time != undefined){
                    switch (e.data.message){
                        case "recv-keepalive": 
                        lastKeepalive = new Date().toLocaleTimeString()
                        lastMessageTimer = holdTimer
                        break;
                        case "sent-keepalive": 
                        sentLastKeepAlive = keepaliveTimer;
                        break;
                        case "recv-update":
                            lastUpdate = new Date().toLocaleTimeString()
                            lastMessageTimer = holdTimer
                        default:
                            console.log(e.data)
                    }
                } else {
                    if (e.data.keepaliveTimer != 0) {
                        keepaliveTimer = e.data.keepaliveTimer
                    }
                    if (e.data.holdTimer) {
                        holdTimer = e.data.holdTimer
                        lastMessageTimer = holdTimer
                    }
                    if (e.data.state != ""){
                        bgpState = e.data.state;
                        if (bgpState == "Established"){
                            receivedRoutes = []
                            for (let route of announcements){
                                socket.send(JSON.stringify({
                                    type: "RouteData",
                                    data: {
                                        prefixes: [{prefix:route.prefix, id: route.id}],
                                        origin: route.origin,
                                        nextHop: route.nexthop,
                                        asPath: route.path,
                                    },
                                }))
                            }
                        }
                    }
                }
            } else if (e.type == "Error") {
                alert("Error: " + e.data.message)
            } else {
                console.log(e.type, e)
            }
        })

        socket.onclose = function (e) {
            console.log("ws closed");
            socketConnected = false;
        };

        socket.onerror = function (e) {
            console.log("ws error", e);
        };
        fetch(endpoint + "routesets.json").then((d)=>d.json()).then((rs)=>{
            routesets=rs
        })
    });

    let peerASN;
    let peerIP;
    let localASN;
    if (localStorage.hasOwnProperty("peerASN")) {
        peerASN = Number(localStorage.getItem("peerASN"))
    } else {
        peerASN = 65530;
    }

    if (localStorage.hasOwnProperty("peerIP")) {
        peerIP = localStorage.getItem("peerIP")
    } else {
        peerIP = "192.0.2.1";
    }

    if (localStorage.hasOwnProperty("localASN")) {
        localASN = Number(localStorage.getItem("localASN"))
    } else {
        localASN = 65510;
    }

    setInterval(()=>{
        if (sentLastKeepAlive > 0) {
            sentLastKeepAlive--
        }
        if (lastMessageTimer > 0){
            lastMessageTimer--
        }
    }, 1000) // TODO set this higher based on timers

    let md5Password;
    let addPath;
    let fullTable;

    function createOrUpdateSession() {
        if(!sessionCreated) {
            localStorage.setItem("localASN", localASN)
            localStorage.setItem("peerIP", peerIP)
            localStorage.setItem("peerASN", peerASN)

            socket.send(JSON.stringify({
                type: "CreateRequest",
                data: {
                    peerASN: peerASN,
                    peerIP: peerIP,
                    localASN: localASN
                }
            }));

            sessionCreated = true; //TODO check for success before setting
        } else {
            console.log("updating existing session");
            socket.send(JSON.stringify({
                type: "UpdateRequest",
                data: {
                    md5Password: md5Password,
                    addPath: addPath,
                    fullTable: fullTable,
                }
            }));
        }
    }

    let newAnnouncementPrefix = "192.0.2.0/24";
    let newAnnouncementNextHop = "192.168.100.100";
    let newAnnouncementPath = "65510, 65530, 65500";
    let newAnnouncementCommunities = "";
    let newAnnouncementLargeCommunities = "";

    function routesetBind(name){
        return function(event){
            if (event.target.checked){
                announceRouteset(name)
            } else {
                removeRouteset(name)
            }
        }
    }

    function removeRouteset(name){
        let toRemove = announcements.filter(a => a.routeset != undefined && a.routeset == name); 
        toRemove = toRemove.map(r=>{
            return {id:r.id, prefix:r.prefix}
        })
        socket.send(JSON.stringify({
            type: "RouteData",
            data: {
                withdraws: toRemove,
            },
        }));
        announcements = announcements.filter(a => a.routeset == undefined || a.routeset != name); 
    }

    function announceRouteset(name){

        let routes = routesets[name]
        for (let i = 0; i < routes.length; i++){
            let route = routes[i]
            let data = {
                prefixes: route.prefixes.map((p)=>{
                    return {prefix:p, id: generateRouteID()}
                }),
                origin: 0,
                nextHop: newAnnouncementNextHop,
                asPath: [],
            }
            if (route.asPath != undefined) {
                data.asPath = route.asPath
            }
            if (route.nextHop != undefined){
                data.nextHop = route.nextHop
            }
            socket.send(JSON.stringify({
                type: "RouteData",
                data: data,
            }))

            for (let j = 0; j < data.prefixes.length; j++){
                let r = data.prefixes[j]
                announcements.push({
                    id: r.id,
                    prefix: r.prefix,
                    path: data.asPath,
                    nexthop: data.nextHop,
                    origin: data.origin,
                    routeset: name
                });
            }
        }
        announcements = announcements; // Trigger svelte refresh

    }

    let id = 0
    function generateRouteID(){
        return id++;
    }

    function addAnnouncement() {
        let pathArray = newAnnouncementPath.split(",").map(x => parseInt(x.trim()));
        let routeID = generateRouteID();

        socket.send(JSON.stringify({
            type: "RouteData",
            data: {
                prefixes: [{prefix: newAnnouncementPrefix, id: routeID}],
                asPath: pathArray,
                nextHop: newAnnouncementNextHop,
                communities: newAnnouncementCommunities
                        .replace(/\s+/g, '')
                        .replace(/[\[\]]]+/g, '')
                        .split(',')
                        .map((element) => {
                            let split = element.split(':');
                            return [Number(split[0]), Number(split[1])];
                        }),
                largeCommunities: newAnnouncementLargeCommunities
                        .replace(/\s+/g, '')
                        .replace(/[\[\]]]+/g, '')
                        .split(',')
                        .map((element) => {
                            let split = element.split(':');
                            return {
                                GlobalAdmin: Number(split[0]),
                                LocalData1: Number(split[1]),
                                LocalData2: Number(split[2])
                            };
                        }),
                origin: 0, // TODO
            },
        }));

        announcements.push({
            id: routeID,
            prefix: newAnnouncementPrefix,
            path: pathArray,
            nexthop: newAnnouncementNextHop,
            communities: newAnnouncementCommunities
                        .replace(/\s+/g, '')
                        .replace(/[\[\]]]+/g, '')
                        .split(',')
                        .map((element) => { return element.split(':') })
                        .map((element) => { return "[" + element.join(":") + "]" }),
            largeCommunities: newAnnouncementLargeCommunities
                        .replace(/\s+/g, '')
                        .replace(/[\[\]]]+/g, '')
                        .split(',')
                        .map((element) => { return element.split(':') })
                        .map((element) => { return "[" + element.join(":") + "]" }),
            origin: 0, // TODO
        });
        announcements = announcements; // Trigger svelte refresh
    }

    function deleteAnnouncement(route) {
        socket.send(JSON.stringify({
            type: "RouteData",
            data: {
                withdraws: [{prefix: route.prefix, id: route.id}],
            },
        }));
    }
</script>

<main>
    <Logo/>
    <div class="container-lg">
        <div class="row mt-3">
            <div class="col-lg-4">
                <h3>Settings</h3>
                <form>
                    <div class="row mb-3">
                        <label for="peer-asn" class="col-sm-4 col-form-label">Your ASN</label>
                        <div class="col-sm-8">
                            <input type="text" class="form-control" id="peer-asn" placeholder="65530" required bind:value={peerASN}>
                        </div>
                    </div>
                    
                    <div class="row mb-3">
                        <label for="peer-ip" class="col-sm-4 col-form-label">Your IP</label>
                        <div class="col-sm-8">
                            <input type="text" class="form-control" id="peer-ip" placeholder="192.0.2.19" required bind:value={peerIP}>
                        </div>
                    </div>

                    <div class="row mb-3">
                        <label for="local-asn" class="col-sm-4 col-form-label">Our ASN</label>
                        <div class="col-sm-8">
                            <input type="text" class="form-control" id="local-asn" placeholder="65510" required bind:value={localASN}>
                        </div>
                    </div>
                    
                    <div class="row mb-3">
                        <label for="local-ip" class="col-sm-4 col-form-label">Our IP</label>
                        <div class="col-sm-8">
                            <input type="text" class="form-control" id="local-ip" value="192.0.2.1" disabled> <!-- TODO populate this -->
                        </div>
                    </div>

                    <div class="row mb-3">
                        <label for="local-router-id" class="col-sm-4 col-form-label">Our Router ID</label>
                        <div class="col-sm-8">
                            <input type="text" class="form-control" id="local-router-id" value="1.1.1.1" disabled>
                        </div>
                    </div>

                    <div class="row mb-3">
                        <label for="md5-password" class="col-sm-4 col-form-label">MD5 Password</label>
                        <div class="col-sm-8">
                            <input type="text" class="form-control" id="md5-password" placeholder="Optional">
                        </div>
                    </div>

                    <fieldset class="row mb-3">
                        <legend class="col-form-label col-sm-4 pt-0">Capabilities</legend>
                        <div class="col-sm-8">
                            <div class="form-check">
                                <input class="form-check-input" type="checkbox" id="add-path" bind:checked={addPath}>
                                <label class="form-check-label" for="add-path">ADD_PATH</label>
                            </div>
                            <div class="form-check">
                                <input class="form-check-input" type="radio" name="gridRadios" id="gridRadios2" value="option2">
                                <label class="form-check-label" for="gridRadios2">
                                    Second radio
                                </label>
                            </div>
                            <div class="form-check disabled">
                                <input class="form-check-input" type="radio" name="gridRadios" id="gridRadios3" value="option3">
                                <label class="form-check-label" for="gridRadios3">
                                    Third radio
                                </label>
                            </div>
                        </div>
                    </fieldset>
                    
                    <button type="button" class="btn btn-primary" disabled='{!socketConnected}' on:click="{createOrUpdateSession}">Save</button>
                </form>
            </div>
            <div class="col-lg-4">
                <h3>Status</h3>
                <p>
                    WebSocket is <b><span class="{socketConnected ? "text-success" : "text-danger"}">{socketConnected ? "Connected" : "Not Connected"}</span></b> <!-- TODO add a reconnect button - also, check every few seconds if we're ACTUALLY connected (e.g. after standby we might be wrong) -->
                    <br>
                    BGP Session is <b><span class="{sessionCreated ? "text-success" : "text-danger"}">{sessionCreated ? "Created" : "Not Created"}</span></b>
                    <br>
                    State: <b>{bgpState}</b>
                    <br>
                    Hold Timer: <b>{lastMessageTimer}</b>/<b>{holdTimer}</b> seconds
                    <br>
                    Keepalive Timer: <b>{sentLastKeepAlive}</b>/<b>{keepaliveTimer}</b> seconds
                    <br>
                    Last UPDATE received: <b>{lastUpdate}</b>
                    <br>
                    Last KEEPALIVE received: <b>{lastKeepalive}</b>
                </p>
            </div>
            <div class="col-lg-4">
                <h3>Log</h3>
                <div class="mb-3">
                    <textarea class="form-control log" id="log" rows="15" value="example text" disabled></textarea>
                </div>
            </div>
        </div>
        <div class="row mt-3">
            <div class="col-lg-6">
                <h3>Announcements</h3>
                <fieldset class="row mb-3">
                    <legend class="col-form-label col-sm-4 pt-0">Route Sets</legend>
                    <div class="col-sm-8">
                        {#each Object.entries(routesets) as [name, rs]}
                        <div class="form-check">
                            <input class="form-check-input" type="checkbox" id="{name}" on:change={routesetBind(name)}>
                            <label class="form-check-label" for="{name}">{name}</label>
                        </div>
                    {/each}
                    </div>
                </fieldset>
                <form>
                    <h4>Custom Prefix</h4>
                    <div class="row mb-3">
                        <label for="new-announcement-prefix" class="col-sm-4 col-form-label">Prefix</label>
                        <div class="col-sm-8">
                            <input type="text" class="form-control" id="new-announcement-prefix" placeholder="192.0.2.0/24" required bind:value={newAnnouncementPrefix}>
                        </div>
                    </div>
                    
                    <div class="row mb-3">
                        <label for="new-announcement-next-hop" class="col-sm-4 col-form-label">Next Hop</label>
                        <div class="col-sm-8">
                            <input type="text" class="form-control" id="new-announcement-next-hop" placeholder="203.0.113.48" required bind:value={newAnnouncementNextHop}>
                        </div>
                    </div>
                    
                    <div class="row mb-3">
                        <label for="new-announcement-communities" class="col-sm-4 col-form-label">Communities</label>
                        <div class="col-sm-8">
                            <input type="text" class="form-control" id="new-announcement-communities" placeholder="65510:1000, 65510:1234" bind:value={newAnnouncementCommunities}>
                        </div>
                    </div>
                    
                    <div class="row mb-3">
                        <label for="new-announcement-large-communities" class="col-sm-4 col-form-label">Large Communities</label>
                        <div class="col-sm-8">
                            <input type="text" class="form-control" id="new-announcement-large-communities" placeholder="65510:1000:1000, 65510:1000:1234" bind:value={newAnnouncementLargeCommunities}>
                        </div>
                    </div>
                    
                    <div class="row mb-3">
                        <label for="new-announcement-as-path" class="col-sm-4 col-form-label">AS Path</label>
                        <div class="col-sm-8">
                            <input type="text" class="form-control" id="new-announcement-as-path" placeholder="65530, 65510, 65500" required bind:value={newAnnouncementPath}>
                        </div>
                    </div>

                    <button type="button" class="btn btn-primary" on:click="{addAnnouncement}">Add</button>
                </form>
            </div>
        </div>
        <div class="row mt-3">
            <div class="col-12">
                <AnnouncementsTable bind:announcements deleteCallback={deleteAnnouncement}/>
            </div>
        </div>
        <div class="row mt-3">
            <div class="col-12">
                <ReceivedRoutesTable bind:receivedRoutes/>
            </div>
        </div>
    </div>
</main>


<style>

</style>