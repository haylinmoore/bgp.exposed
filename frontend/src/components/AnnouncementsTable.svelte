<script>
   export let announcements = [];
</script>

<main>
    <h3>Current Announcements ({announcements.length})</h3>
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
        {#each announcements as route, i}
            <tr>
                <td>{route.prefix}</td>
                <td>{route.path.join(", ")}</td>
                <td>{route.nexthop}</td>
                <td>
                    {#if route.communities}
                        {#each route.communities as community}
                            {community}
                            <br>
                        {/each}
                    {/if}
                </td>
                <td class="delete" on:click={() => {
                    if (confirm("Are you sure you want to remove this announcement? (" + route.prefix + ")")) {
                        // TODO
                        announcements.splice(i, 1);
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
