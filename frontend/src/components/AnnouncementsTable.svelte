<script>
    export let routes = [];
</script>

<main>
    <h4>Current Announcements</h4>
    <table>
        <thead>
        <tr>
            <td>Prefix</td>
            <td>AS Path</td>
            <td>Next Hop</td>
            <td></td> <!-- Space for "-" icon -->
        </tr>
        </thead>

        <tbody>
        {#each routes as route, i}
            <tr>
                <td>{route.prefix}</td>
                <td>{route.path}</td>
                <td>{route.nexthop}</td>
                <td class="delete" on:click={() => {
                    if (confirm("Are you sure you want to remove this announcement? (" + route.prefix + ")")) {
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

    h4 {
        margin-bottom: 10px;
    }

    .delete {
        color: red;
        cursor: pointer;
        font-weight: bold;
        font-size: 1.5em;
    }
</style>