<script>
    import Input from "../components/Input.svelte";
    import Button from "../components/Button.svelte";
    import ReceivedRoutesTable from "../components/ReceivedRoutesTable.svelte";
    import AnnouncementsTable from "../components/AnnouncementsTable.svelte";

    let announcements = [
        {
            prefix: "192.0.2.0/24",
            path: "65510, 65530, 65500",
            nexthop: "203.0.113.48",
        },
        {
            prefix: "192.0.2.0/24",
            path: "65510, 65530, 65500",
            nexthop: "203.0.113.48",
        }
    ];

    // New announcement
    let newAnnouncementPrefix;
    let newAnnouncementNextHop;
    let newAnnouncementPath;

    function addAnnouncement() {
        announcements.push({
            prefix: newAnnouncementPrefix ? newAnnouncementPrefix : "(Default Route)",
            path: newAnnouncementPath,
            nexthop: newAnnouncementNextHop,
        });
        announcements = announcements; // Trigger render
    }
</script>

<main>
    <p class="banner">BGP session with <b>AS65519 (192.0.2.104)</b></p>

    <div class="row">
        <form class="col" on:submit|preventDefault={() => addAnnouncement()}>
            <h3>Announcements</h3>
            <div class="row">
                <Input label="Prefix"
                       placeholder="192.0.2.0/24"
                       description="Leave blank for full table"
                       bind:value={newAnnouncementPrefix}
                       rightPadding/>
                <Input label="Next Hop"
                       placeholder="203.0.113.48"
                       required
                       bind:value={newAnnouncementNextHop}/>
            </div>
            <Input label="AS Path"
                   placeholder="65530, 65510, 65500"
                   bind:value={newAnnouncementPath}
                   required
                   bottomPadding wide/>
            <Button label="Add"/>
            <AnnouncementsTable bind:routes={announcements}/>
        </form>

        <div class="col">
            <h3>Routes Received</h3>
            <ReceivedRoutesTable/>
        </div>
    </div>
</main>
