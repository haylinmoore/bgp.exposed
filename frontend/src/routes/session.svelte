<script>
    import Input from "../components/Input.svelte";
    import Button from "../components/Button.svelte";
    import ReceivedRoutesTable from "../components/ReceivedRoutesTable.svelte";
    import AnnouncementsTable from "../components/AnnouncementsTable.svelte";
    import Checkbox from "../components/Checkbox.svelte";

    let announcements = [
        {
            prefix: "192.0.2.0/24",
            path: [65510, 65530, 65500],
            nexthop: "203.0.113.48",
            communities: ["65510:65510"],
        },
        {
            prefix: "192.0.2.0/24",
            path: [65510, 65530, 65500],
            nexthop: "203.0.113.48",
            communities: ["65510:65510", "65510:65510:65510"],
        }
    ];

    let receivedRoutes = [
        {
            prefix: "192.0.2.0/24",
            path: [65510],
            nexthop: "203.0.113.48",
            rpki: "valid",
            irr: true,
            communities: ["65510:65510"],
        },
        {
            prefix: "192.0.2.0/24",
            path: [65510, 65520],
            nexthop: "203.0.113.48",
            rpki: "valid",
            irr: false,
            communities: ["65510:65510", "65510:65510:65510"],
        }
    ];

    function parseCommaDelimited(s) {
        return s.split(",").map(x => x.trim());
    }

    let newAnnouncementPrefix;
    let newAnnouncementNextHop;
    let newAnnouncementPath;

    function addAnnouncement() {
        announcements.push({
            prefix: newAnnouncementPrefix,
            path: parseCommaDelimited(newAnnouncementPath),
            nexthop: newAnnouncementNextHop,
        });
        announcements = announcements; // Trigger render
        // TODO
    }

    let md5Password;
    let addPath;
    let fullTable;


    let holdTimer = 109;
    let keepaliveTimer = 58;
    let lastKeepalive = "2020-01-01 00:00:00";
    let lastUpdate = "2020-01-01 00:00:00";
</script>

<main>
    <p class="banner">
        BGP session with <b>AS65519 (192.0.2.104)</b>
        <br>
        Hold Timer: <b>{holdTimer}</b>/<b>180</b> seconds, Keepalive Timer: <b>{keepaliveTimer}</b>/<b>60</b> seconds
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
            <AnnouncementsTable bind:routes={announcements}/>
            <ReceivedRoutesTable bind:routes={receivedRoutes}/>
        </div>
    </div>
</main>

<style>
    .settingsRow {
        display: flex;
        flex-direction: row;
        align-items: flex-end;
    }
</style>
