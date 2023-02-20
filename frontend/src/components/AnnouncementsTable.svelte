<script>
    import StringList from "./StringList.svelte";
    export let announcements = [];
    export let deleteCallback = function (cb) {};
</script>

<main>
    <h3>Advertised Routes ({announcements.length})</h3>
    <table>
        <thead>
        <tr>
            <td>Prefix</td>
            <td>AS Path</td>
            <td>Next Hop</td>
            <td>Communities</td>
            <th>Large Communities</th>
            <td></td> <!-- Space for "-" icon -->
        </tr>
        </thead>

        <tbody>
        {#each announcements as route, i}
            <tr>
                <td>{route.prefix}</td>
                <td><StringList list={route.path}/></td>
                <td>{route.nexthop}</td>
                <td><StringList list={route.communities}/></td>
                <td><StringList list={route.largeCommunities}/></td>

                <td class="delete" on:click={() => {
                    if (confirm("Are you sure you want to remove this announcement? (" + route.prefix + ")")) {
                        deleteCallback(route);
                        announcements.splice(i, 1);
                        announcements = announcements; // Trigger rerender
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
