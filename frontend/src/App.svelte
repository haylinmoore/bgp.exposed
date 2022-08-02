<script>
    import {onMount} from "svelte";

    import Logo from "./components/Logo.svelte";
    import Input from "./components/Input.svelte";
    import Button from "./components/Button.svelte";
    import AnnouncementsTable from "./components/AnnouncementsTable.svelte";
    import ReceivedRoutesTable from "./components/ReceivedRoutesTable.svelte";
    import Checkbox from "./components/Checkbox.svelte";

    let created = false;

    let announcements = [];
    let receivedRoutes = [];

    let holdTimer = 0;
    let keepaliveTimer = 0
    let lastUpdate = "Never";
    let lastKeepalive = "Never";

    let socket;
    onMount(() => {
        socket = new WebSocket("ws://localhost:8080/ws"); // window.location.host + "/ws"

        socket.onopen = function (e) {
            console.log("ws connected");
        };

        socket.onmessage = function (e) {
            console.log(e.data);
        };

        socket.onclose = function (e) {
            console.log("ws closed");
        };

        socket.onerror = function (e) {
            console.log("ws error", e);
        };
    });

    let peerASN = 65530;
    let peerIP = "192.0.2.1";
    let localASN = 65510;

    function createSession() {
        socket.send(JSON.stringify({
            type: "CreateRequest",
            data: {
                peerASN: peerASN,
                peerIP: peerIP,
                localASN: localASN
            }
        }));

        socket.addEventListener("message", (e) => {
            if (e.type === "RouteData") {
                for (const prefix of e.data.prefixes) {
                    receivedRoutes.push({
                        id: prefix.id,
                        prefix: prefix.prefix,
                        path: e.data.asPath,
                        nexthop: e.data.nextHop,
                        origin: e.data.origin,
                        communities: [], // TODO
                        rpki: "valid",
                        irr: true
                    });
                    receivedRoutes = receivedRoutes; // Trigger svelte refresh
                }
            }
        })

        created = true;
    }

    let newAnnouncementPrefix = "192.0.2.0/24";
    let newAnnouncementNextHop = "192.168.100.100";
    let newAnnouncementPath = "65510, 65530, 65500";

    function addAnnouncement() {
        let pathArray = newAnnouncementPath.split(",").map(x => parseInt(x.trim()));
        let routeID = new Date().getMilliseconds();  // TODO: Better ID source

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
                prefixes: [{prefix: route.prefix, id: route.id}],
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
            BGP session with <b>AS{peerASN} ({peerIP})</b>
            <br>
            Hold Timer: <b>{holdTimer}</b>/<b>180</b> seconds, Keepalive Timer: <b>{keepaliveTimer}</b>/<b>60</b>
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
