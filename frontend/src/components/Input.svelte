<script>
    export let value, label, placeholder, description;
    export let rightPadding, bottomPadding = false;
    export let wide, number, required = false;
</script>

<main class:rightPadding class:bottomPadding>
    <p>{label}</p>
    <!-- Can't use a ternary here since type bindings don't work right -->
    {#if number}
        <input type="number" {placeholder} {required} class:wide bind:value>
    {:else}
        <input type="text" {placeholder} {required} class:wide bind:value>
    {/if}
    {#if description}
        <p class="description">{description}</p>
    {/if}
</main>

<style>
    input {
        padding: 10px;
        margin: 5px 0;
        color: white;
        background-color: black;
        border: 2px solid white;
        width: 180px;
    }

    p {
        font-size: 12pt;
        margin: 10px 0;
    }

    .description {
        color: #e4e4e4;
        font-size: 10pt;
        margin: 0;
    }

    .rightPadding {
        padding-right: 10px;
    }

    .bottomPadding {
        padding-bottom: 10px;
    }

    .wide {
        width: 400px;
    }
</style>
