<script>
    import {onMount} from "svelte";

    import Logo from "./components/Logo.svelte";
    import Input from "./components/Input.svelte";
    import Button from "./components/Button.svelte";
    import AnnouncementsTable from "./components/AnnouncementsTable.svelte";
    import ReceivedRoutesTable from "./components/ReceivedRoutesTable.svelte";
    import Checkbox from "./components/Checkbox.svelte";
    import { time_ranges_to_array } from "svelte/internal";

    let created = false;

    let announcements = [];
    let receivedRoutes = [];

    let state = "Unknown";
    let holdTimer = 0;
    let lastMessageTimer = 0;
    let keepaliveTimer = 0;
    let sentLastKeepAlive = 0;
    let lastUpdate = "Never";
    let lastKeepalive = "Never";

    let routesets = [];

    let endpoint = "http://localhost:8080/"
    if (window.location.host.includes("bgp.exposed")){
        endpoint = window.location.protocol+"//"+window.location.host+"/api/"
    }

    let socket;
    onMount(() => {
        socket = new WebSocket(endpoint.replace("http", "ws") + "ws"); // window.location.host + "/ws"

        socket.onopen = function (e) {
            console.log("ws connected");
        };

        socket.onmessage = function (e) {
            console.log(e.data);
        };

        socket.onclose = function (e) {
            console.log("ws closed");
            created = false
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
    }, 1000)

    function createSession() {
        if (socket.readyState != 1) {
            alert("Websocket connection to the backend is down. Please reload or try again later")
            return;
        }

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

        socket.addEventListener("message", (e) => {
            e = JSON.parse(e.data)
            if (e.type === "RouteData") {
                for (const prefix of e.data.prefixes) {
                    receivedRoutes.push({
                        id: prefix.id,
                        prefix: prefix.prefix,
                        path: e.data.asPath,
                        nexthop: e.data.nextHop,
                        origin: e.data.origin,
                        communities: [], // TODO
                        rpki: "invalid",
                        irr: false
                    });
                }
                receivedRoutes = receivedRoutes; // Trigger svelte refresh
            } if (e.type=="FSMUpdate") {
                if (e.data.time != undefined){
                    switch (e.data.message){
                        case "recv-keepalive": 
                        lastKeepalive = new Date().toTimeString().split(" ")[0]
                        lastMessageTimer = holdTimer
                        break;
                        case "sent-keepalive": 
                        sentLastKeepAlive = keepaliveTimer;
                        break;
                        case "recv-update":
                            lastUpdate = new Date().toTimeString().split(" ")[0]
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
                        state = e.data.state;
                        if (state == "Established"){
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
            } else {
                console.log(e.type, e)
            }
        })

        created = true;
    }

    let newAnnouncementPrefix = "192.0.2.0/24";
    let newAnnouncementNextHop = "192.168.100.100";
    let newAnnouncementPath = "65510, 65530, 65500";

    function routesetBind(name){
        return function(check){
            if (check){
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
                origin: 0, // TODO
            },
        }));

        announcements.push({
            id: routeID,
            prefix: newAnnouncementPrefix,
            path: pathArray,
            nexthop: newAnnouncementNextHop,
            origin: 0, // TODO
        });
        announcements = announcements; // Trigger svelte refresh
    }

    let md5Password;
    let addPath;
    let fullTable;

    function updateSession() {
        socket.send(JSON.stringify({
            type: "UpdateRequest",
            data: {
                md5Password: md5Password,
                addPath: addPath,
                fullTable: fullTable,
            }
        }));
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
    <p>
        <slot name="banner"/>
    </p>
    {#if !created}
        <p class="banner">BGP.exposed is a ...</p>

        <div class="row">
            <form on:submit|preventDefault={() => createSession()}>
                <h3>New BGP Session</h3>
                <Input required label="ASN" placeholder="65530" number bind:value={peerASN}/>
                <Input required label="IP" placeholder="192.0.2.19" bind:value={peerIP}/>
                <Input required bottomPadding label="Our ASN" placeholder="65510" number bind:value={localASN}/>
                <Button label="Submit"/>
            </form>
        </div>
    {:else}
        <p class="banner">
            BGP session with <b>AS{peerASN} ({peerIP})</b> State: <b>{state}</b>
            <br>
            Hold Timer: <b>{lastMessageTimer}</b>/<b>{holdTimer}</b> seconds, Keepalive Timer: <b>{sentLastKeepAlive}</b>/<b>{keepaliveTimer}</b>
            seconds
            <br>
            Last UPDATE: <b>{lastUpdate}</b>, Last KEEPALIVE: <b>{lastKeepalive}</b>
        </p>

        <div class="row">
            <div style="margin-right: 20px;">
                <form on:submit|preventDefault={() => updateSession()}>
                    <h3>Settings</h3>
                    <div class="settingsRow">
                    <span style="margin-bottom: 5px; margin-right: 12px">
                        <Input label="MD5 Password" placeholder="Optional" bind:value={md5Password}/>
                    </span>
                        <div class="col">
                            <Checkbox label="ADD_PATH?" bind:value={addPath}/>
                            <Checkbox label="Full table?" bind:value={fullTable}/>
                        </div>
                    </div>
                    <Button label="Save"/>
                </form>

                <form on:submit|preventDefault={() => addAnnouncement()}>
                    <h3>Announcements</h3>
                    <div class="col">
                        {#each Object.entries(routesets) as [name, rs]}
                            <Checkbox label={name} cb={routesetBind(name)}/>
                        {/each}
                    </div>
                    <div class="row">
                        <Input label="Prefix"
                               placeholder="192.0.2.0/24"
                               required
                               bind:value={newAnnouncementPrefix}
                               rightPadding/>
                        <Input label="Next Hop"
                               placeholder="203.0.113.48"
                               required
                               bind:value={newAnnouncementNextHop}/>
                    </div>
                    <Input label="Communities"
                           placeholder="65510:65530, 65500:65500:65510"
                           wide/>
                    <Input label="AS Path"
                           placeholder="65530, 65510, 65500"
                           bind:value={newAnnouncementPath}
                           required
                           bottomPadding wide/>
                    <Button label="Add"/>
                </form>
            </div>

            <div>
                <AnnouncementsTable bind:announcements deleteCallback={deleteAnnouncement}/>
                <ReceivedRoutesTable bind:receivedRoutes/>
            </div>
        </div>
    {/if}
</main>

<style>
    main {
        margin: 50px auto;
        padding-left: 50px;
        padding-right: 50px;
    }

    .settingsRow {
        display: flex;
        flex-direction: row;
        align-items: flex-end;
    }
</style>
