<script>
    let routes = [
        {
            prefix: "192.0.2.0/24",
            origin: 65510,
            nexthop: "203.0.113.48",
            rpki: "valid",
            irr: true,
        },
        {
            prefix: "192.0.2.0/24",
            origin: 65510,
            nexthop: "203.0.113.48",
            rpki: "valid",
            irr: false,
        }
    ]
</script>

<table>
    <thead>
    <tr>
        <th>Prefix</th>
        <th>Origin</th>
        <th>Nexthop</th>
        <th>RPKI</th>
        <th>IRR</th>
    </tr>
    </thead>
    <tbody>
    {#each routes as route}
        <tr>
            <td>{route.prefix}</td>
            <td>{route.origin}</td>
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
        </tr>
    {/each}
    </tbody>
</table>
