<script>
    export let routes = [];

    function deleteAnnouncement(route) {
        // TODO
    }
</script>

<main>
    <h3>Current Announcements</h3>
    <table>
        <thead>
        <tr>
            <td>Prefix</td>
            <td>AS Path</td>
            <td>Next Hop</td>
            <td>Communities</td>
            <td></td> <!-- Space for "-" icon -->
        </tr>
        </thead>

        <tbody>
        {#each routes as route, i}
            <tr>
                <td>{route.prefix}</td>
                <td>{route.path.join(", ")}</td>
                <td>{route.nexthop}</td>
                <td>
                    {#each route.communities as community}
                        {community}
                        <br>
                    {/each}
                </td>
                <td class="delete" on:click={() => {
                    if (confirm("Are you sure you want to remove this announcement? (" + route.prefix + ")")) {
                        deleteAnnouncement(route);
                        routes.splice(i, 1);
                        routes = routes; // Trigger svelte render
                    }
                }}>-</td>
            </tr>
        {/each}
        </tbody>
    </table>
</main>

<style>
    main, table {
        width: 100% !important;
    }

    .delete {
        color: red;
        cursor: pointer;
        font-weight: bold;
        font-size: 1.25em;
    }
</style>
