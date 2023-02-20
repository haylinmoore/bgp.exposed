<script>
    import StringList from "./StringList.svelte";
    export let receivedRoutes = [];
</script>

<main>
    <h3>Received Routes ({receivedRoutes.length})</h3>
    <table class="table">
        <thead>
        <tr>
            <th>Prefix</th>
            <th>AS Path</th>
            <th>Nexthop</th>
            <th>RPKI</th>
            <th>IRR</th>
            <th>Communities</th>
            <th>Large Communities</th>
        </tr>
        </thead>
        <tbody>
        {#each receivedRoutes as route}
            <tr>
                <td>{route.prefix}</td>
                <td><StringList list={route.path}/></td>
                <td>{route.nexthop}</td>
                {#if route.rpki === "valid"}
                    <td style="color: lightgreen">Valid</td>
                {:else if route.rpki === "notFound"}
                    <td style="color: yellow">Not Found</td>
                {:else if route.rpki === "invalid"}
                    <td style="color: red">Invalid</td>
                {/if}
                {#if route.irr}
                    <td style="color: lightgreen">Found</td>
                {:else}
                    <td style="color: red">Not Found</td>
                {/if}
                <td><StringList list={route.communities}/></td>
                <td><StringList list={route.largeCommunities}/></td>
            </tr>
        {/each}
        </tbody>
    </table>
</main>
